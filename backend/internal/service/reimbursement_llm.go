package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
)

// 硬编码默认值：settings 与 env 都没给时兜底。
const (
	defaultReimbursementLLMBaseURL   = "https://api.deepseek.com"
	defaultReimbursementLLMModel     = "deepseek-flash"
	defaultReimbursementLLMTimeoutMS = 60000
	minReimbursementLLMTimeoutMS     = 5000
	maxReimbursementLLMTimeoutMS     = 120000
	maxReimbursementLLMBaseURLLen    = 200
	maxReimbursementLLMModelLen      = 100
	reimbursementLLMMaxTokens        = 1024
	// 上游错误体只截前 512 字节进日志，避免把大响应写进日志。
	reimbursementLLMErrorBodyLimit = 512
	// 上游响应体上限：JSON 抽取结果远小于此，超过即视为异常。
	reimbursementLLMResponseLimit = 1 << 20
)

// 配置来源标识，随配置视图返回给管理端。
const (
	ReimbursementLLMConfigSourceSettings = "settings"
	ReimbursementLLMConfigSourceEnv      = "env"
)

// reimbursementSystemPrompt 已用真实 DeepSeek API 跑过契约里的三个案例，逐字使用，
// 改动措辞会影响抽取稳定性，改前必须重新跑 live 测试。
const reimbursementSystemPrompt = `你是一个「企业开票/报销信息」结构化抽取器。用户会粘贴一段自由格式的中文文本（可能来自聊天记录、开票资料截图转文字、邮件等），你要从中抽取以下 6 个字段，并且只输出一个 JSON 对象，不要输出任何解释、前后缀或 Markdown 代码块。

输出 JSON 的固定结构（键名必须完全一致，全部保留，缺失的字段填 null）：
{
  "company_name": string|null,   // 公司名称 / 名称 / 抬头 / 账户名称（开票单位全称）
  "tax_id": string|null,         // 公司税号 / 纳税人识别号 / 统一社会信用代码
  "bank_account": string|null,   // 银行账户 / 账号 / 账户号码 / 基本户账号（纯数字）
  "bank_name": string|null,      // 开户行 / 开户银行（银行全称+支行）
  "address": string|null,        // 地址 / 注册地址 / 公司地址
  "amount": number|null,         // 报销/开票金额，单位人民币元，纯数字
  "notes": string|null           // 你对本次抽取的简短备注（如某字段疑似不完整），没有则 null
}

抽取规则：
1. 只填文本里明确出现的信息；找不到就填 null，绝不编造、绝不用示例或常识补全。
2. tax_id：去掉所有空格、连字符等分隔符，统一为大写字母+数字（例："9131 0115 MAKF GD5N XY" → "91310115MAKFGD5NXY"）。
3. bank_account：只保留数字，去掉空格和连字符。不要把税号、电话当成账号。
4. bank_name：保留银行全称与支行名（例："中国工商银行股份有限公司北京东城支行"），不要截断。
5. company_name：取开票单位全称；若同时出现「名称」和「账户名称」且一致，取其一即可。
6. amount：识别「金额」「报销金额」「开票金额」「合计」「¥」「元」等表述的数值，去掉货币符号和千分位，输出 number（例："金额414.1" → 414.1，"¥1,234.50" → 1234.5）。若出现多个金额且无法判断哪个是报销金额，填 null 并在 notes 说明。
7. 电话、邮箱、联系人等与 6 个字段无关的信息一律忽略。
8. 如果同时提供了「已解析字段」（previous_fields），表示这是用户对上一轮结果的补充：以本次新文本为准更新明确提到的字段，其余字段沿用 previous_fields 的值；不要因为新文本没提到就把已有字段清空。
9. 若文本与开票/报销信息完全无关，所有字段填 null，并在 notes 写「未识别到开票信息」。`

// ReimbursementLLMConfig 是 settings 表 reimbursement_llm_config 的 JSON 形状。
// api_key 明文存储，与仓库现有 content_moderation_config / SMTP 密码同口径。
type ReimbursementLLMConfig struct {
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKey    string `json:"api_key"`
	TimeoutMS int    `json:"timeout_ms"`
}

// ReimbursementLLMConfigView 是返回给管理端的配置视图，永不含明文 key。
type ReimbursementLLMConfigView struct {
	BaseURL          string `json:"base_url"`
	Model            string `json:"model"`
	TimeoutMS        int    `json:"timeout_ms"`
	APIKeyConfigured bool   `json:"api_key_configured"`
	APIKeyMasked     string `json:"api_key_masked"`
	APIKeySource     string `json:"api_key_source"`
}

// UpdateReimbursementLLMConfigInput 是管理端 PUT 的输入；nil 表示不修改。
type UpdateReimbursementLLMConfigInput struct {
	BaseURL     *string
	Model       *string
	TimeoutMS   *int
	APIKey      *string
	ClearAPIKey bool
}

// ReimbursementLLMTestResult 是管理端「测试解析」的返回。
type ReimbursementLLMTestResult struct {
	OK        bool                      `json:"ok"`
	LatencyMS int64                     `json:"latency_ms"`
	Model     string                    `json:"model"`
	Result    *ReimbursementParseResult `json:"result"`
}

// reimbursementResolvedConfig 是三级来源合并后的生效配置。
type reimbursementResolvedConfig struct {
	BaseURL      string
	Model        string
	APIKey       string
	TimeoutMS    int
	APIKeySource string
}

// ReimbursementParser 负责 DeepSeek 调用与配置读写。每次调用即时解析配置，不缓存，
// 管理员后台改完立即生效。
type ReimbursementParser struct {
	settingRepo SettingRepository
	cfg         *config.Config
	// httpClient 非 nil 时优先使用（单测注入 httptest client）；生产由 clientFor 按超时构建。
	httpClient *http.Client
	now        func() time.Time
}

func NewReimbursementParser(settingRepo SettingRepository, cfg *config.Config) *ReimbursementParser {
	return &ReimbursementParser{
		settingRepo: settingRepo,
		cfg:         cfg,
		now:         time.Now,
	}
}

// Parse 调 DeepSeek 抽取六字段并做归一化 / 防御性合并。text 已由上层校验长度。
func (p *ReimbursementParser) Parse(ctx context.Context, text string, previous *ReimbursementFields) (*ReimbursementParseResult, error) {
	resolved, err := p.resolveConfig(ctx)
	if err != nil {
		return nil, err
	}
	if resolved.APIKey == "" {
		return nil, ErrReimbursementLLMNotConfigured
	}
	var normalizedPrevious *ReimbursementFields
	if previous != nil {
		np := NormalizeReimbursementFields(*previous)
		normalizedPrevious = &np
	}
	userMessage := buildReimbursementUserMessage(text, normalizedPrevious)
	payload, err := p.callWithRetry(ctx, resolved, userMessage)
	if err != nil {
		return nil, err
	}
	fields := NormalizeReimbursementFields(payload.fields())
	fields = MergeReimbursementFields(fields, normalizedPrevious)
	return BuildReimbursementParseResult(fields, payload.Notes), nil
}

// buildReimbursementUserMessage 按契约第 6 节拼用户消息。
func buildReimbursementUserMessage(text string, previous *ReimbursementFields) string {
	if previous == nil {
		return "用户文本：\n" + text
	}
	raw, err := json.Marshal(previous)
	if err != nil {
		raw = []byte("{}")
	}
	return "previous_fields（上一轮已解析的字段，作为基底）：\n" + string(raw) + "\n\n本次用户补充/新文本：\n" + text
}

type reimbursementChatRequest struct {
	Model          string                   `json:"model"`
	Messages       []reimbursementChatMsg   `json:"messages"`
	Temperature    float64                  `json:"temperature"`
	MaxTokens      int                      `json:"max_tokens"`
	Stream         bool                     `json:"stream"`
	ResponseFormat reimbursementChatRespFmt `json:"response_format"`
}

type reimbursementChatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type reimbursementChatRespFmt struct {
	Type string `json:"type"`
}

type reimbursementChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// reimbursementLLMPayload 是模型输出的 JSON；amount 用 RawMessage 兼容 number 与字符串。
type reimbursementLLMPayload struct {
	CompanyName *string         `json:"company_name"`
	TaxID       *string         `json:"tax_id"`
	BankAccount *string         `json:"bank_account"`
	BankName    *string         `json:"bank_name"`
	Address     *string         `json:"address"`
	Amount      json.RawMessage `json:"amount"`
	Notes       *string         `json:"notes"`
}

func (p *reimbursementLLMPayload) fields() ReimbursementFields {
	return ReimbursementFields{
		CompanyName: p.CompanyName,
		TaxID:       p.TaxID,
		BankAccount: p.BankAccount,
		BankName:    p.BankName,
		Address:     p.Address,
		Amount:      parseReimbursementAmountRaw(p.Amount),
	}
}

// parseReimbursementAmountRaw 兼容 414.1、"414.1"、"¥1,234.50"、"45元" 等写法；解析不了当缺失。
func parseReimbursementAmountRaw(raw json.RawMessage) *float64 {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		return &num
	}
	var str string
	if err := json.Unmarshal(raw, &str); err != nil {
		return nil
	}
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9', r == '.', r == '-':
			return r
		default:
			return -1
		}
	}, str)
	if cleaned == "" {
		return nil
	}
	num, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return nil
	}
	return &num
}

// reimbursementLLMHTTPError 携带上游状态码，供重试策略判断 4xx/5xx。
type reimbursementLLMHTTPError struct {
	status int
	body   string
}

func (e *reimbursementLLMHTTPError) Error() string {
	return fmt.Sprintf("reimbursement llm status %d: %s", e.status, e.body)
}

// callWithRetry 最多重试 1 次，只对网络错误 / 5xx 重试；4xx 与解析失败不重试。
func (p *ReimbursementParser) callWithRetry(ctx context.Context, cfg *reimbursementResolvedConfig, userMessage string) (*reimbursementLLMPayload, error) {
	payload, err := p.callOnce(ctx, cfg, userMessage)
	if err == nil {
		return payload, nil
	}
	if reimbursementLLMRetryable(err) {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-time.After(200 * time.Millisecond):
			payload, err = p.callOnce(ctx, cfg, userMessage)
			if err == nil {
				return payload, nil
			}
		}
	}
	// 只记录错误摘要，不记录用户文本或抽取结果。
	slog.Warn("reimbursement.llm_call_failed", "model", cfg.Model, "error", err.Error())
	return nil, ErrReimbursementLLMFailed.WithCause(err)
}

func reimbursementLLMRetryable(err error) bool {
	var httpErr *reimbursementLLMHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.status >= 500
	}
	var parseErr *reimbursementLLMParseError
	if errors.As(err, &parseErr) {
		return false
	}
	// 其余（连接失败、超时）视为网络错误。
	return true
}

// reimbursementLLMParseError 表示上游 2xx 但内容不是可用 JSON。
type reimbursementLLMParseError struct{ msg string }

func (e *reimbursementLLMParseError) Error() string { return e.msg }

func (p *ReimbursementParser) callOnce(ctx context.Context, cfg *reimbursementResolvedConfig, userMessage string) (*reimbursementLLMPayload, error) {
	endpoint, err := url.JoinPath(strings.TrimRight(cfg.BaseURL, "/"), "/chat/completions")
	if err != nil {
		return nil, &reimbursementLLMParseError{msg: "invalid base url: " + err.Error()}
	}
	body, err := json.Marshal(reimbursementChatRequest{
		Model: cfg.Model,
		Messages: []reimbursementChatMsg{
			{Role: "system", Content: reimbursementSystemPrompt},
			{Role: "user", Content: userMessage},
		},
		Temperature:    0,
		MaxTokens:      reimbursementLLMMaxTokens,
		Stream:         false,
		ResponseFormat: reimbursementChatRespFmt{Type: "json_object"},
	})
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client, err := p.clientFor(timeout)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, reimbursementLLMErrorBodyLimit))
		return nil, &reimbursementLLMHTTPError{status: resp.StatusCode, body: strings.TrimSpace(string(snippet))}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, reimbursementLLMResponseLimit))
	if err != nil {
		return nil, err
	}
	var out reimbursementChatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, &reimbursementLLMParseError{msg: "decode chat response: " + err.Error()}
	}
	if len(out.Choices) == 0 {
		return nil, &reimbursementLLMParseError{msg: "chat response has no choices"}
	}
	payload, err := parseReimbursementLLMContent(out.Choices[0].Message.Content)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// parseReimbursementLLMContent 解析模型正文：trim、剥 ```json 围栏、再 Unmarshal。
func parseReimbursementLLMContent(content string) (*reimbursementLLMPayload, error) {
	text := strings.TrimSpace(content)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		if idx := strings.Index(text, "\n"); idx >= 0 {
			// 第一行是 ```json 之类的语言标记，整行丢掉。
			text = text[idx+1:]
		}
		text = strings.TrimSuffix(strings.TrimSpace(text), "```")
		text = strings.TrimSpace(text)
	}
	if text == "" {
		return nil, &reimbursementLLMParseError{msg: "empty llm content"}
	}
	// 模型偶尔会在 JSON 前后夹解释文字，只截取最外层花括号。
	if start := strings.Index(text, "{"); start > 0 {
		text = text[start:]
	}
	if end := strings.LastIndex(text, "}"); end >= 0 && end < len(text)-1 {
		text = text[:end+1]
	}
	var payload reimbursementLLMPayload
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, &reimbursementLLMParseError{msg: "llm content is not valid json: " + err.Error()}
	}
	return &payload, nil
}

func (p *ReimbursementParser) clientFor(timeout time.Duration) (*http.Client, error) {
	if p.httpClient != nil {
		return p.httpClient, nil
	}
	client, err := httpclient.GetClient(httpclient.Options{Timeout: timeout, ValidateResolvedIP: true})
	if err != nil {
		return nil, fmt.Errorf("build reimbursement llm client: %w", err)
	}
	return client, nil
}

// loadSettingsConfig 读 settings 表；未配置 / 解析失败都返回空配置而不是报错，
// 让 env 兜底继续生效。
func (p *ReimbursementParser) loadSettingsConfig(ctx context.Context) ReimbursementLLMConfig {
	var cfg ReimbursementLLMConfig
	if p.settingRepo == nil {
		return cfg
	}
	raw, err := p.settingRepo.GetValue(ctx, SettingKeyReimbursementLLMConfig)
	if err != nil {
		if !errors.Is(err, ErrSettingNotFound) {
			slog.Warn("reimbursement.load_settings_failed", "error", err.Error())
		}
		return cfg
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		slog.Warn("reimbursement.settings_json_invalid", "error", err.Error())
		return ReimbursementLLMConfig{}
	}
	return cfg
}

// resolveConfig 逐字段按 settings > env > 默认 合并。
func (p *ReimbursementParser) resolveConfig(ctx context.Context) (*reimbursementResolvedConfig, error) {
	stored := p.loadSettingsConfig(ctx)
	var env config.ReimbursementConfig
	if p.cfg != nil {
		env = p.cfg.Reimbursement
	}
	out := &reimbursementResolvedConfig{
		BaseURL:   firstNonEmptyTrimmed(stored.BaseURL, env.LLMBaseURL, defaultReimbursementLLMBaseURL),
		Model:     firstNonEmptyTrimmed(stored.Model, env.LLMModel, defaultReimbursementLLMModel),
		TimeoutMS: defaultReimbursementLLMTimeoutMS,
	}
	switch {
	case stored.TimeoutMS > 0:
		out.TimeoutMS = stored.TimeoutMS
	case env.LLMTimeoutMS > 0:
		out.TimeoutMS = env.LLMTimeoutMS
	}
	// env 里的超时不经过 PUT 校验，这里再夹一次范围，避免 0.1 秒或十分钟的超时。
	if out.TimeoutMS < minReimbursementLLMTimeoutMS {
		out.TimeoutMS = minReimbursementLLMTimeoutMS
	}
	if out.TimeoutMS > maxReimbursementLLMTimeoutMS {
		out.TimeoutMS = maxReimbursementLLMTimeoutMS
	}
	switch {
	case strings.TrimSpace(stored.APIKey) != "":
		out.APIKey = strings.TrimSpace(stored.APIKey)
		out.APIKeySource = ReimbursementLLMConfigSourceSettings
	case strings.TrimSpace(env.LLMAPIKey) != "":
		out.APIKey = strings.TrimSpace(env.LLMAPIKey)
		out.APIKeySource = ReimbursementLLMConfigSourceEnv
	}
	return out, nil
}

// reimbursementConfigError 生成同 reason、不同 message 的 400，便于前端按 reason 归类。
func reimbursementConfigError(message string) error {
	return infraerrors.BadRequest(ErrReimbursementConfigInvalid.Reason, message)
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

func (p *ReimbursementParser) configView(resolved *reimbursementResolvedConfig) *ReimbursementLLMConfigView {
	return &ReimbursementLLMConfigView{
		BaseURL:          resolved.BaseURL,
		Model:            resolved.Model,
		TimeoutMS:        resolved.TimeoutMS,
		APIKeyConfigured: resolved.APIKey != "",
		APIKeyMasked:     maskSecretTail(resolved.APIKey),
		APIKeySource:     resolved.APIKeySource,
	}
}

// GetConfig 返回当前生效配置（掩码视图）。
func (p *ReimbursementParser) GetConfig(ctx context.Context) (*ReimbursementLLMConfigView, error) {
	resolved, err := p.resolveConfig(ctx)
	if err != nil {
		return nil, err
	}
	return p.configView(resolved), nil
}

// UpdateConfig 写 settings：api_key 空串/缺省保留旧值，clear_api_key 清空 settings 里的 key。
func (p *ReimbursementParser) UpdateConfig(ctx context.Context, input UpdateReimbursementLLMConfigInput) (*ReimbursementLLMConfigView, error) {
	if p.settingRepo == nil {
		return nil, reimbursementConfigError("setting repository unavailable")
	}
	stored := p.loadSettingsConfig(ctx)
	if input.BaseURL != nil {
		baseURL := strings.TrimSpace(*input.BaseURL)
		if baseURL != "" {
			if len(baseURL) > maxReimbursementLLMBaseURLLen {
				return nil, reimbursementConfigError("base_url is too long")
			}
			parsed, err := url.Parse(baseURL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				return nil, reimbursementConfigError("base_url must start with http:// or https://")
			}
		}
		stored.BaseURL = baseURL
	}
	if input.Model != nil {
		model := strings.TrimSpace(*input.Model)
		if len(model) > maxReimbursementLLMModelLen {
			return nil, reimbursementConfigError("model is too long")
		}
		stored.Model = model
	}
	if input.TimeoutMS != nil {
		timeout := *input.TimeoutMS
		if timeout != 0 && (timeout < minReimbursementLLMTimeoutMS || timeout > maxReimbursementLLMTimeoutMS) {
			return nil, reimbursementConfigError("timeout_ms must be between 5000 and 120000")
		}
		stored.TimeoutMS = timeout
	}
	if input.ClearAPIKey {
		stored.APIKey = ""
	} else if input.APIKey != nil && strings.TrimSpace(*input.APIKey) != "" {
		stored.APIKey = strings.TrimSpace(*input.APIKey)
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return nil, fmt.Errorf("marshal reimbursement llm config: %w", err)
	}
	if err := p.settingRepo.Set(ctx, SettingKeyReimbursementLLMConfig, string(raw)); err != nil {
		return nil, fmt.Errorf("save reimbursement llm config: %w", err)
	}
	return p.GetConfig(ctx)
}

// Test 用当前生效配置真实调用一次，供管理端验证 key 与连通性。
func (p *ReimbursementParser) Test(ctx context.Context, text string) (*ReimbursementLLMTestResult, error) {
	resolved, err := p.resolveConfig(ctx)
	if err != nil {
		return nil, err
	}
	start := p.now()
	result, err := p.Parse(ctx, text, nil)
	if err != nil {
		return nil, err
	}
	return &ReimbursementLLMTestResult{
		OK:        true,
		LatencyMS: p.now().Sub(start).Milliseconds(),
		Model:     resolved.Model,
		Result:    result,
	}, nil
}
