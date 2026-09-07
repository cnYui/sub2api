package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/tidwall/gjson"
)

// monitorHTTPClient 共享一个 http.Client，避免每次检测重建 transport。
// 自定义 Transport 在 dial 时强制再次校验 IP，防止 DNS rebinding 绕过 validateEndpoint。
var monitorHTTPClient = newSSRFSafeHTTPClient(monitorRequestTimeout)

// newSSRFSafeHTTPClient 返回一个使用 safeDialContext 的 http.Client。
// 仅供监控模块对外发起请求使用——所有目标都应是公网 endpoint。
func newSSRFSafeHTTPClient(timeout time.Duration) *http.Client {
	tr := &http.Transport{
		DialContext:           safeDialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          16,
		IdleConnTimeout:       monitorIdleConnTimeout,
		TLSHandshakeTimeout:   monitorTLSHandshakeTimeout,
		ResponseHeaderTimeout: monitorResponseHeaderTimeout,
	}
	return &http.Client{Timeout: timeout, Transport: servertiming.WrapRoundTripper(tr)}
}

// CheckOptions 承载一次检测的自定义入参。
// 所有字段都是可选（零值即等价于"用默认行为"）。
type CheckOptions struct {
	// APIMode 当前固定为 models；保留字段用于 API/模板快照兼容。
	APIMode string
	// ExtraHeaders 用户自定义 HTTP 头（merge 到 adapter 默认 headers，用户优先）。
	ExtraHeaders map[string]string
	// BodyOverrideMode: off | merge | replace
	BodyOverrideMode string
	// BodyOverride 在 merge 模式下做浅合并（key 命中黑名单时静默丢弃），
	// 在 replace 模式下直接当作完整 body。
	BodyOverride map[string]any
}

// runChecksForModels 对一个监控的所有模型共享一次 GET /v1/models 请求。
// 目录探测只验证 HTTP 2xx、响应可解析以及目标模型是否存在，不产生推理 token。
func runChecksForModels(ctx context.Context, provider, endpoint, apiKey string, models []string, opts *CheckOptions) []*CheckResult {
	checkedAt := time.Now()
	start := time.Now()
	catalog, rawBody, statusCode, err := callModels(ctx, provider, endpoint, apiKey, opts)
	latencyMs := int(time.Since(start) / time.Millisecond)
	results := make([]*CheckResult, len(models))
	for i, model := range models {
		result := &CheckResult{
			Model:     model,
			Status:    MonitorStatusError,
			LatencyMs: &latencyMs,
			CheckedAt: checkedAt,
		}
		if err != nil {
			result.Message = truncateMessage(sanitizeErrorMessage(err.Error()))
			results[i] = result
			continue
		}
		if statusCode < 200 || statusCode >= 300 {
			result.Message = truncateMessage(sanitizeErrorMessage(fmt.Sprintf("upstream HTTP %d: %s", statusCode, truncateForErrorBody(rawBody))))
			results[i] = result
			continue
		}
		if _, ok := catalog[strings.TrimSpace(model)]; !ok {
			result.Status = MonitorStatusFailed
			result.Message = truncateMessage(fmt.Sprintf("model %q not found in /v1/models", strings.TrimSpace(model)))
			results[i] = result
			continue
		}
		// 目录探测的成功语义就是“目录请求返回 2xx 且模型存在”。
		result.Status = MonitorStatusOperational
		results[i] = result
	}
	return results
}

// callModels 发起一次带鉴权的 GET /v1/models，并解析 OpenAI 兼容目录。
func callModels(ctx context.Context, provider, endpoint, apiKey string, opts *CheckOptions) (map[string]struct{}, string, int, error) {
	if err := validateAPIMode(provider, checkAPIMode(opts)); err != nil {
		return nil, "", 0, err
	}
	headers := modelsProbeHeaders(provider, apiKey)
	headers = mergeHeaders(headers, opts)
	respBytes, status, err := getRawJSON(ctx, joinURL(endpoint, providerModelsPath), headers)
	if err != nil {
		return nil, "", status, err
	}
	if status < 200 || status >= 300 {
		return nil, string(respBytes), status, nil
	}
	catalog, err := parseModelsCatalog(respBytes)
	if err != nil {
		return nil, string(respBytes), status, fmt.Errorf("parse /v1/models response: %w", err)
	}
	return catalog, string(respBytes), status, nil
}

func modelsProbeHeaders(provider, apiKey string) map[string]string {
	switch provider {
	case MonitorProviderAnthropic:
		return map[string]string{"x-api-key": apiKey, "anthropic-version": monitorAnthropicAPIVersion}
	case MonitorProviderGemini:
		return map[string]string{"x-goog-api-key": apiKey}
	default:
		return map[string]string{"Authorization": "Bearer " + apiKey}
	}
}

type modelsCatalogItem struct {
	ID string `json:"id"`
}

type modelsCatalogResponse struct {
	Data   []modelsCatalogItem `json:"data"`
	Models []modelsCatalogItem `json:"models"`
}

func parseModelsCatalog(body []byte) (map[string]struct{}, error) {
	var envelope modelsCatalogResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	items := envelope.Data
	if len(items) == 0 {
		items = envelope.Models
	}
	if items == nil {
		return nil, errors.New("response has no data array")
	}
	catalog := make(map[string]struct{}, len(items))
	for _, item := range items {
		if id := strings.TrimSpace(item.ID); id != "" {
			catalog[id] = struct{}{}
		}
	}
	return catalog, nil
}

// providerAdapter 描述某个 provider 在 challenge 检测中需要的几件事：
//   - 拼出请求路径（含 model 占位）
//   - 序列化请求体
//   - 构造鉴权头
//   - 从响应 JSON 中提取文本（默认按 gjson path；需要时可自定义）
//
// 加新 provider 只需要在 providerAdapters 里增加一个条目，无需触碰 callProvider / validateProvider。
type providerAdapter struct {
	buildPath    func(model string) string
	buildBody    func(model, prompt string) ([]byte, error)
	buildHeaders func(apiKey string) map[string]string
	textPath     string // gjson 提取响应文本的 path
	extractText  func([]byte) string
}

// providerAdapters 全部已支持的 provider。键值即 MonitorProvider* 字符串。
//
//nolint:gochecknoglobals // 适配器表是只读静态数据，初始化后不变更。
var providerAdapters = map[string]providerAdapter{
	MonitorProviderOpenAI: providerOpenAIChatAdapter,
	MonitorProviderGrok:   providerGrokChatAdapter,
	MonitorProviderAnthropic: {
		buildPath: func(string) string { return providerAnthropicPath },
		buildBody: func(model, prompt string) ([]byte, error) {
			return json.Marshal(map[string]any{
				"model":      model,
				"messages":   []map[string]string{{"role": "user", "content": prompt}},
				"max_tokens": monitorChallengeMaxTokens,
			})
		},
		buildHeaders: func(apiKey string) map[string]string {
			return map[string]string{
				"x-api-key":         apiKey,
				"anthropic-version": monitorAnthropicAPIVersion,
			}
		},
		extractText: extractAnthropicMonitorText,
	},
	MonitorProviderGemini: {
		// Gemini 把 model 名写在 URL path 上：/v1beta/models/{model}:generateContent
		buildPath: func(model string) string { return fmt.Sprintf(providerGeminiPathTemplate, model) },
		buildBody: func(_, prompt string) ([]byte, error) {
			return json.Marshal(map[string]any{
				"contents": []map[string]any{
					{"parts": []map[string]any{{"text": prompt}}},
				},
				"generationConfig": map[string]any{"maxOutputTokens": monitorChallengeMaxTokens},
			})
		},
		// 使用 x-goog-api-key header 而不是 ?key= query，避免 *url.Error 把 key 回填到错误日志。
		buildHeaders: func(apiKey string) map[string]string {
			return map[string]string{"x-goog-api-key": apiKey}
		},
		textPath: "candidates.0.content.parts.0.text",
	},
}

//nolint:gochecknoglobals // 适配器表是只读静态数据，初始化后不变更。
var providerOpenAIChatAdapter = newOpenAICompatibleChatAdapter(providerOpenAIPath)

//nolint:gochecknoglobals // 适配器表是只读静态数据，初始化后不变更。
var providerGrokChatAdapter = newOpenAICompatibleChatAdapter(providerGrokPath)

func newOpenAICompatibleChatAdapter(path string) providerAdapter {
	return providerAdapter{
		buildPath: func(string) string { return path },
		buildBody: func(model, prompt string) ([]byte, error) {
			return json.Marshal(map[string]any{
				"model":      model,
				"messages":   []map[string]string{{"role": "user", "content": prompt}},
				"max_tokens": monitorChallengeMaxTokens,
				"stream":     false,
			})
		},
		buildHeaders: func(apiKey string) map[string]string {
			return map[string]string{"Authorization": "Bearer " + apiKey}
		},
		textPath: "choices.0.message.content",
	}
}

// isSupportedProvider 校验 provider 字符串是否在 adapter 表中。
// 供 validate.go 的 validateProvider 复用，避免两份 switch 漂移。
func isSupportedProvider(p string) bool {
	_, ok := providerAdapters[p]
	return ok
}

func extractAnthropicMonitorText(respBytes []byte) string {
	content := gjson.GetBytes(respBytes, "content")
	if !content.IsArray() {
		return ""
	}

	parts := make([]string, 0, 1)
	content.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() != "text" {
			return true
		}
		text := strings.TrimSpace(item.Get("text").String())
		if text != "" {
			parts = append(parts, text)
		}
		return true
	})
	return strings.Join(parts, "\n")
}

// mergeHeaders 把用户自定义 headers 合并到 adapter 默认 headers 上。
// 用户值覆盖默认；命中黑名单（hop-by-hop / 由 http.Client 自管的）的 key 静默丢弃。
func mergeHeaders(base map[string]string, opts *CheckOptions) map[string]string {
	if opts == nil || len(opts.ExtraHeaders) == 0 {
		return base
	}
	out := make(map[string]string, len(base)+len(opts.ExtraHeaders))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range opts.ExtraHeaders {
		if IsForbiddenHeaderName(k) {
			continue
		}
		out[k] = v
	}
	return out
}

func checkAPIMode(opts *CheckOptions) string {
	if opts == nil {
		return MonitorAPIModeModels
	}
	return defaultAPIMode(opts.APIMode)
}

func validateReplaceRequestBody(provider, apiMode string, body map[string]any) error {
	if provider != MonitorProviderOpenAI && provider != MonitorProviderGrok {
		return nil
	}
	switch defaultAPIMode(apiMode) {
	case MonitorAPIModeResponses:
		if strings.TrimSpace(stringFromAny(body["instructions"])) == "" || !hasNonEmptyBodyValue(body["input"]) {
			return fmt.Errorf("replace mode responses body: instructions and input are required")
		}
	case MonitorAPIModeChatCompletions:
		if !hasNonEmptyBodyValue(body["messages"]) {
			return fmt.Errorf("replace mode chat_completions body: messages are required")
		}
	}
	return nil
}

func stringFromAny(v any) string {
	s, _ := v.(string)
	return s
}

func hasNonEmptyBodyValue(v any) bool {
	switch val := v.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(val) != ""
	case []any:
		return len(val) > 0
	case []map[string]any:
		return len(val) > 0
	case []map[string]string:
		return len(val) > 0
	default:
		return true
	}
}

// getRawJSON 发送 GET 请求并限制响应体大小，供无 token 的目录探测使用。
func getRawJSON(ctx context.Context, fullURL string, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := monitorHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, monitorResponseMaxBytes))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read body: %w", err)
	}
	return respBody, resp.StatusCode, nil
}

// joinURL 把 base origin 与 path 拼成完整 URL。
// 容忍 base 末尾有/无斜杠，path 必带前导斜杠。
func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

// monitorSensitiveQueryParamRegex 匹配 URL query 中可能泄露凭证的参数：
// key / api_key / api-key / access_token / token / authorization / x-api-key。
// 大小写不敏感，匹配 `?name=value` 或 `&name=value` 形式（value 截到 & 或字符串末尾）。
var monitorSensitiveQueryParamRegex = regexp.MustCompile(`(?i)([?&](?:key|api[_-]?key|access[_-]?token|token|authorization|x-api-key)=)[^&\s"']+`)

// monitorAPIKeyPatterns 匹配常见 provider 的 API key 字面量。
// 顺序敏感：sk-ant- 必须放在 sk- 之前，否则会被通用 sk- 模式先消费。
var monitorAPIKeyPatterns = []struct {
	pattern *regexp.Regexp
	replace string
}{
	// Anthropic（带前缀，必须先匹配）：sk-ant-xxxxxxx
	{regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`), "sk-ant-***REDACTED***"},
	// OpenAI / Anthropic 通用 sk-: sk-xxxxxxx
	{regexp.MustCompile(`sk-[A-Za-z0-9-]{20,}`), "sk-***REDACTED***"},
	// xAI API Key：xai-xxxxxxx
	{regexp.MustCompile(`xai-[A-Za-z0-9_-]{6,}`), "xai-***REDACTED***"},
	// Gemini / Google API Key：固定前缀 + 35 位
	{regexp.MustCompile(`AIza[A-Za-z0-9_-]{35}`), "AIza***REDACTED***"},
	// JWT 三段式（Bearer 后常出现）：eyJxxx.eyJxxx.signature
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`), "eyJ***REDACTED.JWT***"},
}

// sanitizeErrorMessage 擦除错误/响应文本中可能泄露的 API key。
// 处理两类来源：
//  1. URL query 中的 ?key= / ?api_key= 等（Go *url.Error 会回填完整 URL）
//  2. 上游 HTTP body 文本里直接出现的 sk-* / xai-* / AIza* / JWT 等密钥碎片
//
// 注意：与 gemini_messages_compat_service.go 的 sanitizeUpstreamErrorMessage 关注点类似但参数集更广，
// 监控模块独立维护，避免互相耦合。
func sanitizeErrorMessage(msg string) string {
	if msg == "" {
		return msg
	}
	msg = monitorSensitiveQueryParamRegex.ReplaceAllString(msg, `${1}REDACTED`)
	for _, p := range monitorAPIKeyPatterns {
		msg = p.pattern.ReplaceAllString(msg, p.replace)
	}
	return msg
}

// truncateMessage 把消息按 monitorMessageMaxBytes 截断，避免 DB 列溢出与日志过长。
func truncateMessage(msg string) string {
	if len(msg) <= monitorMessageMaxBytes {
		return msg
	}
	const ellipsis = "...(truncated)"
	cutoff := monitorMessageMaxBytes - len(ellipsis)
	if cutoff < 0 {
		cutoff = 0
	}
	return msg[:cutoff] + ellipsis
}

// truncateForErrorBody 把上游错误响应 body 压到 monitorErrorBodySnippetMaxBytes 以内，
// 并顺手把连续空白折成一个空格：上游 HTML 错误页常含大量缩进/换行，保留会浪费预算。
// 被 truncateMessage 做最终总截断兜底，所以这里只负责 body 自身的精简。
func truncateForErrorBody(body string) string {
	body = strings.Join(strings.Fields(body), " ")
	if len(body) <= monitorErrorBodySnippetMaxBytes {
		return body
	}
	const ellipsis = "...(body truncated)"
	cutoff := monitorErrorBodySnippetMaxBytes - len(ellipsis)
	if cutoff < 0 {
		cutoff = 0
	}
	return body[:cutoff] + ellipsis
}
