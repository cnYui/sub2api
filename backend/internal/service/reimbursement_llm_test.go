//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 契约第 7 节的真实 DeepSeek 返回，直接作为 canned content。
const (
	reimbursementCase1Text = "开票信息：\n名称\t福州斯摩尔贸易有限公司\n纳税人识别号\t91350102068793190Q\n开户行\t中国银行福州东区支行\n账号\t406565458594\n地址\t福州市鼓楼区鼓东街道井大路205号七星井新村B17座中侧3层354\n电话\t13605948312\n邮箱：419016664@qq.com   金额45"
	reimbursementCase2Text = "公司名称：上海熠视智能科技有限公司\n公司税号：9131 0115 MAKF GD5N XY\n基本户信息：\n账户名称:上海熠视智能科技有限公司\n账户号码:121995771710001\n开户银行:招商银行股份有限公司上海张杨支行  金额414.1"
	reimbursementCase3Text = "名称: 北京和讯在线信息咨询服务有限公司\n纳税人识别号: 91110105723558454P\n开户行: 中国工商银行股份有限公司北京东城支行\n银行账号: 0200080709024530517\n地址: 北京市朝阳区朝外大街22号10层A1-3\n电话: 010-85650972   金额  49"

	reimbursementCase1Content = `{"company_name":"福州斯摩尔贸易有限公司","tax_id":"91350102068793190Q","bank_account":"406565458594","bank_name":"中国银行福州东区支行","address":"福州市鼓楼区鼓东街道井大路205号七星井新村B17座中侧3层354","amount":45,"notes":null}`
	reimbursementCase2Content = `{"company_name":"上海熠视智能科技有限公司","tax_id":"91310115MAKFGD5NXY","bank_account":"121995771710001","bank_name":"招商银行股份有限公司上海张杨支行","address":null,"amount":414.1,"notes":null}`
	reimbursementCase3Content = `{"company_name":"北京和讯在线信息咨询服务有限公司","tax_id":"91110105723558454P","bank_account":"0200080709024530517","bank_name":"中国工商银行股份有限公司北京东城支行","address":"北京市朝阳区朝外大街22号10层A1-3","amount":49,"notes":null}`
)

type reimbursementCapturedRequest struct {
	Authorization string
	Body          map[string]any
}

// newReimbursementLLMServer 伪造 DeepSeek chat/completions：按调用次序返回 responder 的内容。
func newReimbursementLLMServer(t *testing.T, responder func(call int, w http.ResponseWriter, r *http.Request)) (*httptest.Server, *atomic.Int32, *[]reimbursementCapturedRequest) {
	t.Helper()
	var calls atomic.Int32
	captured := &[]reimbursementCapturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/chat/completions", r.URL.Path)
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var body map[string]any
		require.NoError(t, json.Unmarshal(raw, &body))
		*captured = append(*captured, reimbursementCapturedRequest{Authorization: r.Header.Get("Authorization"), Body: body})
		call := int(calls.Add(1))
		responder(call, w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls, captured
}

func writeReimbursementChatContent(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": "chatcmpl-test",
		"choices": []map[string]any{{
			"index":         0,
			"finish_reason": "stop",
			"message":       map[string]any{"role": "assistant", "content": content},
		}},
		"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 20},
	})
}

func newReimbursementTestParser(t *testing.T, srv *httptest.Server, apiKey string) (*ReimbursementParser, *stubSettingRepo) {
	t.Helper()
	repo := newStubSettingRepo()
	stored := ReimbursementLLMConfig{BaseURL: srv.URL, Model: "deepseek-flash", APIKey: apiKey, TimeoutMS: 5000}
	raw, err := json.Marshal(stored)
	require.NoError(t, err)
	repo.values[SettingKeyReimbursementLLMConfig] = string(raw)
	parser := NewReimbursementParser(repo, &config.Config{})
	parser.httpClient = srv.Client()
	return parser, repo
}

func reimbStr(s string) *string { return &s }

func TestReimbursementParser_ParseContractCases(t *testing.T) {
	cases := []struct {
		name         string
		text         string
		content      string
		wantFields   ReimbursementFields
		wantMissing  []string
		wantComplete bool
	}{
		{
			name:    "case1",
			text:    reimbursementCase1Text,
			content: reimbursementCase1Content,
			wantFields: ReimbursementFields{
				CompanyName: reimbStr("福州斯摩尔贸易有限公司"),
				TaxID:       reimbStr("91350102068793190Q"),
				BankAccount: reimbStr("406565458594"),
				BankName:    reimbStr("中国银行福州东区支行"),
				Address:     reimbStr("福州市鼓楼区鼓东街道井大路205号七星井新村B17座中侧3层354"),
				Amount:      reimbFloat(45),
			},
			wantMissing:  []string{},
			wantComplete: true,
		},
		{
			name:    "case2 missing address",
			text:    reimbursementCase2Text,
			content: reimbursementCase2Content,
			wantFields: ReimbursementFields{
				CompanyName: reimbStr("上海熠视智能科技有限公司"),
				TaxID:       reimbStr("91310115MAKFGD5NXY"),
				BankAccount: reimbStr("121995771710001"),
				BankName:    reimbStr("招商银行股份有限公司上海张杨支行"),
				Address:     nil,
				Amount:      reimbFloat(414.1),
			},
			wantMissing:  []string{"address"},
			wantComplete: false,
		},
		{
			name:    "case3",
			text:    reimbursementCase3Text,
			content: reimbursementCase3Content,
			wantFields: ReimbursementFields{
				CompanyName: reimbStr("北京和讯在线信息咨询服务有限公司"),
				TaxID:       reimbStr("91110105723558454P"),
				BankAccount: reimbStr("0200080709024530517"),
				BankName:    reimbStr("中国工商银行股份有限公司北京东城支行"),
				Address:     reimbStr("北京市朝阳区朝外大街22号10层A1-3"),
				Amount:      reimbFloat(49),
			},
			wantMissing:  []string{},
			wantComplete: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, calls, captured := newReimbursementLLMServer(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
				writeReimbursementChatContent(w, tc.content)
			})
			parser, _ := newReimbursementTestParser(t, srv, "sk-test-key-1234")

			result, err := parser.Parse(context.Background(), tc.text, nil)
			require.NoError(t, err)
			require.Equal(t, tc.wantFields, result.Fields)
			require.Equal(t, tc.wantMissing, result.Missing)
			require.Equal(t, tc.wantComplete, result.Complete)
			require.Nil(t, result.Notes)
			require.Equal(t, int32(1), calls.Load())

			// 请求体形状：模型、json_object、system prompt 逐字、用户消息带前缀。
			req := (*captured)[0]
			require.Equal(t, "Bearer sk-test-key-1234", req.Authorization)
			require.Equal(t, "deepseek-flash", req.Body["model"])
			require.Equal(t, false, req.Body["stream"])
			require.Equal(t, float64(0), req.Body["temperature"])
			rf, ok := req.Body["response_format"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "json_object", rf["type"])
			msgs, ok := req.Body["messages"].([]any)
			require.True(t, ok)
			require.Len(t, msgs, 2)
			sys, ok := msgs[0].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "system", sys["role"])
			require.Equal(t, reimbursementSystemPrompt, sys["content"])
			user, ok := msgs[1].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "用户文本：\n"+tc.text, user["content"])
		})
	}
}

func TestReimbursementParser_SupplementRoundMergesPrevious(t *testing.T) {
	// 模型只回了地址、其余 null：防御性合并必须沿用 previous。
	srv, _, captured := newReimbursementLLMServer(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeReimbursementChatContent(w, `{"company_name":null,"tax_id":null,"bank_account":null,"bank_name":null,"address":"上海市浦东新区张杨路500号华润时代广场12楼","amount":null,"notes":null}`)
	})
	parser, _ := newReimbursementTestParser(t, srv, "sk-test-key-1234")

	previous := &ReimbursementFields{
		CompanyName: reimbStr("上海熠视智能科技有限公司"),
		TaxID:       reimbStr("9131 0115 makf gd5n xy"), // 未归一化输入也要被归一化
		BankAccount: reimbStr("121995771710001"),
		BankName:    reimbStr("招商银行股份有限公司上海张杨支行"),
		Amount:      reimbFloat(414.1),
	}
	result, err := parser.Parse(context.Background(), "地址是上海市浦东新区张杨路500号华润时代广场12楼", previous)
	require.NoError(t, err)
	require.True(t, result.Complete)
	require.Empty(t, result.Missing)
	require.Equal(t, "上海熠视智能科技有限公司", *result.Fields.CompanyName)
	require.Equal(t, "91310115MAKFGD5NXY", *result.Fields.TaxID)
	require.Equal(t, "121995771710001", *result.Fields.BankAccount)
	require.Equal(t, "招商银行股份有限公司上海张杨支行", *result.Fields.BankName)
	require.Equal(t, "上海市浦东新区张杨路500号华润时代广场12楼", *result.Fields.Address)
	require.Equal(t, 414.1, *result.Fields.Amount)

	msgs, ok := (*captured)[0].Body["messages"].([]any)
	require.True(t, ok)
	user, ok := msgs[1].(map[string]any)
	require.True(t, ok)
	content, ok := user["content"].(string)
	require.True(t, ok)
	require.True(t, strings.HasPrefix(content, "previous_fields（上一轮已解析的字段，作为基底）：\n{"))
	require.Contains(t, content, `"tax_id":"91310115MAKFGD5NXY"`)
	require.Contains(t, content, `"address":null`)
	require.Contains(t, content, "\n\n本次用户补充/新文本：\n地址是上海市浦东新区张杨路500号华润时代广场12楼")
}

func TestReimbursementParser_AcceptsFencedContentAndStringAmount(t *testing.T) {
	srv, _, _ := newReimbursementLLMServer(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeReimbursementChatContent(w, "```json\n{\"company_name\":\"示例公司\",\"tax_id\":\"91-3101 15abc\",\"bank_account\":\"6222 0000-1234\",\"bank_name\":\"示例银行\",\"address\":\"  某地  \",\"amount\":\"¥1,234.50\",\"notes\":\"  \"}\n```")
	})
	parser, _ := newReimbursementTestParser(t, srv, "sk-test-key-1234")

	result, err := parser.Parse(context.Background(), "任意文本", nil)
	require.NoError(t, err)
	require.True(t, result.Complete)
	require.Equal(t, "示例公司", *result.Fields.CompanyName)
	require.Equal(t, "91310115ABC", *result.Fields.TaxID)
	require.Equal(t, "622200001234", *result.Fields.BankAccount)
	require.Equal(t, "某地", *result.Fields.Address)
	require.Equal(t, 1234.5, *result.Fields.Amount)
	require.Nil(t, result.Notes, "空白 notes 归一为 null")
}

func TestReimbursementParser_IrrelevantTextAllMissing(t *testing.T) {
	srv, _, _ := newReimbursementLLMServer(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeReimbursementChatContent(w, `{"company_name":null,"tax_id":null,"bank_account":null,"bank_name":null,"address":null,"amount":null,"notes":"未识别到开票信息"}`)
	})
	parser, _ := newReimbursementTestParser(t, srv, "sk-test-key-1234")

	result, err := parser.Parse(context.Background(), "你好，请问今天天气怎么样？", nil)
	require.NoError(t, err)
	require.False(t, result.Complete)
	require.Equal(t, ReimbursementFieldKeys, result.Missing)
	require.Equal(t, ReimbursementFields{}, result.Fields)
	require.NotNil(t, result.Notes)
	require.Equal(t, "未识别到开票信息", *result.Notes)

	// 六个键序列化后总是全部存在。
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	for _, key := range ReimbursementFieldKeys {
		require.Contains(t, string(raw), `"`+key+`":null`)
	}
}

func TestReimbursementParser_Upstream5xxRetriesOnceThenFails(t *testing.T) {
	srv, calls, _ := newReimbursementLLMServer(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
	})
	parser, _ := newReimbursementTestParser(t, srv, "sk-test-key-1234")

	_, err := parser.Parse(context.Background(), "x", nil)
	require.ErrorIs(t, err, ErrReimbursementLLMFailed)
	require.Equal(t, int32(2), calls.Load(), "5xx 重试一次")
	// 对外 message 固定中文，不透上游原文。
	var appErr interface{ Error() string }
	require.True(t, errors.As(err, &appErr))
	require.Contains(t, err.Error(), "解析服务暂时不可用")
}

func TestReimbursementParser_Upstream4xxDoesNotRetry(t *testing.T) {
	srv, calls, _ := newReimbursementLLMServer(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"invalid key"}`, http.StatusUnauthorized)
	})
	parser, _ := newReimbursementTestParser(t, srv, "sk-bad")

	_, err := parser.Parse(context.Background(), "x", nil)
	require.ErrorIs(t, err, ErrReimbursementLLMFailed)
	require.Equal(t, int32(1), calls.Load())
}

func TestReimbursementParser_InvalidJSONContentFails(t *testing.T) {
	srv, calls, _ := newReimbursementLLMServer(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeReimbursementChatContent(w, "抱歉，我无法处理。")
	})
	parser, _ := newReimbursementTestParser(t, srv, "sk-test-key-1234")

	_, err := parser.Parse(context.Background(), "x", nil)
	require.ErrorIs(t, err, ErrReimbursementLLMFailed)
	require.Equal(t, int32(1), calls.Load(), "内容解析失败不重试")
}

func TestReimbursementParser_TimeoutFails(t *testing.T) {
	release := make(chan struct{})
	srv, _, _ := newReimbursementLLMServer(t, func(_ int, w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
		writeReimbursementChatContent(w, reimbursementCase1Content)
	})
	t.Cleanup(func() { close(release) })
	parser, _ := newReimbursementTestParser(t, srv, "sk-test-key-1234")

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := parser.Parse(ctx, "x", nil)
	require.ErrorIs(t, err, ErrReimbursementLLMFailed)
	require.Less(t, time.Since(start), 3*time.Second)
}

func TestReimbursementParser_NotConfigured(t *testing.T) {
	repo := newStubSettingRepo()
	parser := NewReimbursementParser(repo, &config.Config{})
	_, err := parser.Parse(context.Background(), "x", nil)
	require.ErrorIs(t, err, ErrReimbursementLLMNotConfigured)

	view, err := parser.GetConfig(context.Background())
	require.NoError(t, err)
	require.False(t, view.APIKeyConfigured)
	require.Equal(t, "", view.APIKeyMasked)
	require.Equal(t, "", view.APIKeySource)
	require.Equal(t, defaultReimbursementLLMBaseURL, view.BaseURL)
	require.Equal(t, defaultReimbursementLLMModel, view.Model)
	require.Equal(t, defaultReimbursementLLMTimeoutMS, view.TimeoutMS)
}

func TestReimbursementParser_SettingsTakePrecedenceOverEnv(t *testing.T) {
	settingsSrv, settingsCalls, captured := newReimbursementLLMServer(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeReimbursementChatContent(w, reimbursementCase1Content)
	})
	envSrv, envCalls, _ := newReimbursementLLMServer(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeReimbursementChatContent(w, reimbursementCase1Content)
	})
	cfg := &config.Config{}
	cfg.Reimbursement = config.ReimbursementConfig{
		LLMAPIKey:    "env-key-9999",
		LLMBaseURL:   envSrv.URL,
		LLMModel:     "env-model",
		LLMTimeoutMS: 7000,
	}

	// settings 只覆盖 key 与 base_url，model / timeout 留空 → 落到 env。
	repo := newStubSettingRepo()
	repo.values[SettingKeyReimbursementLLMConfig] = `{"base_url":"` + settingsSrv.URL + `","api_key":"settings-key-abcd"}`
	parser := NewReimbursementParser(repo, cfg)
	parser.httpClient = settingsSrv.Client()

	view, err := parser.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, ReimbursementLLMConfigSourceSettings, view.APIKeySource)
	require.Equal(t, settingsSrv.URL, view.BaseURL)
	require.Equal(t, "env-model", view.Model)
	require.Equal(t, 7000, view.TimeoutMS)

	_, err = parser.Parse(context.Background(), "x", nil)
	require.NoError(t, err)
	require.Equal(t, int32(1), settingsCalls.Load())
	require.Equal(t, int32(0), envCalls.Load())
	require.Equal(t, "Bearer settings-key-abcd", (*captured)[0].Authorization)
	require.Equal(t, "env-model", (*captured)[0].Body["model"])

	// settings 清空后回落到 env。
	repo.values[SettingKeyReimbursementLLMConfig] = ""
	parser.httpClient = envSrv.Client()
	view, err = parser.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, ReimbursementLLMConfigSourceEnv, view.APIKeySource)
	require.Equal(t, envSrv.URL, view.BaseURL)
	_, err = parser.Parse(context.Background(), "x", nil)
	require.NoError(t, err)
	require.Equal(t, int32(1), envCalls.Load())
}

func TestReimbursementParser_ConfigViewNeverLeaksKey(t *testing.T) {
	repo := newStubSettingRepo()
	repo.values[SettingKeyReimbursementLLMConfig] = `{"api_key":"sk-super-secret-value-dde9"}`
	parser := NewReimbursementParser(repo, &config.Config{})

	view, err := parser.GetConfig(context.Background())
	require.NoError(t, err)
	require.True(t, view.APIKeyConfigured)
	require.Equal(t, "********dde9", view.APIKeyMasked)
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "sk-super-secret-value")
	require.NotContains(t, string(raw), "super-secret")
	require.Contains(t, string(raw), `"api_key_source":"settings"`)
}

func TestReimbursementParser_UpdateConfig(t *testing.T) {
	repo := newStubSettingRepo()
	parser := NewReimbursementParser(repo, &config.Config{})
	ctx := context.Background()

	view, err := parser.UpdateConfig(ctx, UpdateReimbursementLLMConfigInput{
		BaseURL:   reimbStr("https://api.deepseek.com/"),
		Model:     reimbStr("deepseek-flash"),
		TimeoutMS: reimbInt(30000),
		APIKey:    reimbStr("sk-first-key-1111"),
	})
	require.NoError(t, err)
	require.True(t, view.APIKeyConfigured)
	require.Equal(t, "********1111", view.APIKeyMasked)
	require.Equal(t, 30000, view.TimeoutMS)

	// 空串 api_key 保留旧值。
	view, err = parser.UpdateConfig(ctx, UpdateReimbursementLLMConfigInput{APIKey: reimbStr("   "), Model: reimbStr("deepseek-v4-pro")})
	require.NoError(t, err)
	require.Equal(t, "********1111", view.APIKeyMasked)
	require.Equal(t, "deepseek-v4-pro", view.Model)
	var stored ReimbursementLLMConfig
	require.NoError(t, json.Unmarshal([]byte(repo.values[SettingKeyReimbursementLLMConfig]), &stored))
	require.Equal(t, "sk-first-key-1111", stored.APIKey)

	// clear_api_key 清空 settings 里的 key。
	view, err = parser.UpdateConfig(ctx, UpdateReimbursementLLMConfigInput{ClearAPIKey: true, APIKey: reimbStr("ignored-when-clearing")})
	require.NoError(t, err)
	require.False(t, view.APIKeyConfigured)
	require.Equal(t, "", view.APIKeySource)

	// 校验失败。
	_, err = parser.UpdateConfig(ctx, UpdateReimbursementLLMConfigInput{BaseURL: reimbStr("ftp://x")})
	require.ErrorIs(t, err, ErrReimbursementConfigInvalid)
	_, err = parser.UpdateConfig(ctx, UpdateReimbursementLLMConfigInput{TimeoutMS: reimbInt(1000)})
	require.ErrorIs(t, err, ErrReimbursementConfigInvalid)
	_, err = parser.UpdateConfig(ctx, UpdateReimbursementLLMConfigInput{TimeoutMS: reimbInt(500000)})
	require.ErrorIs(t, err, ErrReimbursementConfigInvalid)
	_, err = parser.UpdateConfig(ctx, UpdateReimbursementLLMConfigInput{Model: reimbStr(strings.Repeat("m", 101))})
	require.ErrorIs(t, err, ErrReimbursementConfigInvalid)
}

func TestReimbursementParser_TestReturnsLatencyAndModel(t *testing.T) {
	srv, _, _ := newReimbursementLLMServer(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeReimbursementChatContent(w, reimbursementCase3Content)
	})
	parser, _ := newReimbursementTestParser(t, srv, "sk-test-key-1234")

	result, err := parser.Test(context.Background(), reimbursementCase3Text)
	require.NoError(t, err)
	require.True(t, result.OK)
	require.Equal(t, "deepseek-flash", result.Model)
	require.GreaterOrEqual(t, result.LatencyMS, int64(0))
	require.True(t, result.Result.Complete)
}

func TestParseReimbursementAmountRaw(t *testing.T) {
	cases := map[string]*float64{
		`414.1`:       reimbFloat(414.1),
		`"414.1"`:     reimbFloat(414.1),
		`"¥1,234.50"`: reimbFloat(1234.5),
		`"45元"`:       reimbFloat(45),
		`null`:        nil,
		``:            nil,
		`"abc"`:       nil,
		`true`:        nil,
	}
	for raw, want := range cases {
		got := parseReimbursementAmountRaw(json.RawMessage(raw))
		if want == nil {
			require.Nil(t, got, raw)
			continue
		}
		require.NotNil(t, got, raw)
		require.InDelta(t, *want, *got, 1e-9, raw)
	}
}

func TestNormalizeReimbursementFields(t *testing.T) {
	in := ReimbursementFields{
		CompanyName: reimbStr("  示例公司  "),
		TaxID:       reimbStr(" 9131-0115 makf_gd5n xy "),
		BankAccount: reimbStr("6222 0000-1234 abc"),
		BankName:    reimbStr(""),
		Address:     reimbStr("   "),
		Amount:      reimbFloat(12.345),
	}
	out := NormalizeReimbursementFields(in)
	require.Equal(t, "示例公司", *out.CompanyName)
	require.Equal(t, "91310115MAKFGD5NXY", *out.TaxID)
	require.Equal(t, "622200001234", *out.BankAccount)
	require.Nil(t, out.BankName)
	require.Nil(t, out.Address)
	require.Equal(t, 12.35, *out.Amount)
	require.Equal(t, []string{"bank_name", "address"}, MissingReimbursementFields(out))

	require.Nil(t, NormalizeReimbursementFields(ReimbursementFields{Amount: reimbFloat(0)}).Amount)
	require.Nil(t, NormalizeReimbursementFields(ReimbursementFields{Amount: reimbFloat(-1)}).Amount)
	require.Nil(t, NormalizeReimbursementFields(ReimbursementFields{Amount: reimbFloat(2e9)}).Amount)
	long := strings.Repeat("地", 600)
	require.Equal(t, 500, len([]rune(*NormalizeReimbursementFields(ReimbursementFields{Address: reimbStr(long)}).Address)))
}

func reimbInt(v int) *int { return &v }

func reimbFloat(v float64) *float64 { return &v }
