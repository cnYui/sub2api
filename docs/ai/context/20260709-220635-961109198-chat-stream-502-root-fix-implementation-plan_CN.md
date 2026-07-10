# Chat Completions 流式 502 根治 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 OpenAI APIKey 账号下的标准 `/v1/chat/completions` 请求默认走上游 raw Chat Completions，避免 Chat 到 Responses 转换路径因 terminal usage 缺失返回 502。

**Architecture:** 在 `ForwardAsChatCompletions()` 入口增加协议感知分流：APIKey + Chat shape 默认 raw chat；Responses shape 和 OAuth 继续保留现有 Responses 路径；手动覆盖模式优先级最高。复用现有 raw chat 计费保护和 `stream_options.include_usage=true` 注入，不改账务保护。

**Tech Stack:** Go、Gin、gjson、现有 `openai_compat` helper、现有 OpenAI gateway service unit tests。

---

## 文件结构

- Modify: `backend/internal/pkg/openai_compat/upstream_capability.go`
  - 职责：新增 `ResolveResponsesSupportMode()`，供 service 层读取手动覆盖模式。
- Modify: `backend/internal/pkg/openai_compat/upstream_capability_test.go`
  - 职责：锁定手动模式解析行为。
- Modify: `backend/internal/service/openai_gateway_chat_completions.go`
  - 职责：替换 APIKey `/v1/chat/completions` 入口分流策略。
- Modify: `backend/internal/service/openai_gateway_chat_completions_raw_test.go`
  - 职责：覆盖 APIKey Chat shape 默认 raw chat，以及未探测账号不再先试 Responses。
- Modify: `backend/internal/service/openai_gateway_chat_completions_test.go`
  - 职责：保留 `force_responses` 行为，覆盖 Responses shape 兼容分支。
- Create: `docs/ai/context/20260709-220635-961109198-chat-stream-502-root-fix-result_CN.md`
  - 职责：实施完成后记录 RED/GREEN、验证命令、影响范围。
- Modify: `AGENTS.md`
  - 职责：实施完成后追加长期记忆。

## Task 1: 新增手动模式解析 helper

**Files:**
- Modify: `backend/internal/pkg/openai_compat/upstream_capability_test.go`
- Modify: `backend/internal/pkg/openai_compat/upstream_capability.go`

- [ ] **Step 1: 写失败测试**

在 `TestNormalizeResponsesSupportMode` 前新增：

```go
func TestResolveResponsesSupportMode(t *testing.T) {
	tests := []struct {
		name  string
		extra map[string]any
		want  ResponsesSupportMode
	}{
		{"nil extra", nil, ResponsesSupportModeAuto},
		{"empty extra", map[string]any{}, ResponsesSupportModeAuto},
		{"auto", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeAuto)}, ResponsesSupportModeAuto},
		{"force responses", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeForceResponses)}, ResponsesSupportModeForceResponses},
		{"force chat completions", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeForceChatCompletions)}, ResponsesSupportModeForceChatCompletions},
		{"invalid mode", map[string]any{ExtraKeyResponsesMode: "enabled"}, ResponsesSupportModeAuto},
		{"wrong type", map[string]any{ExtraKeyResponsesMode: true}, ResponsesSupportModeAuto},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveResponsesSupportMode(tc.extra)
			if got != tc.want {
				t.Errorf("ResolveResponsesSupportMode(%v) = %q, want %q", tc.extra, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: 运行 RED**

Run:

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 ./internal/pkg/openai_compat -run TestResolveResponsesSupportMode
```

Expected: FAIL，错误包含 `undefined: ResolveResponsesSupportMode`。

- [ ] **Step 3: 实现 helper**

在 `NormalizeResponsesSupportMode()` 后增加：

```go
// ResolveResponsesSupportMode 从账号 extra 中读取手动覆盖模式。
// 缺失、空值或非法类型按 auto 处理，让调用方继续跟随探测结果。
func ResolveResponsesSupportMode(extra map[string]any) ResponsesSupportMode {
	if extra == nil {
		return ResponsesSupportModeAuto
	}
	mode, ok := extra[ExtraKeyResponsesMode].(string)
	if !ok {
		return ResponsesSupportModeAuto
	}
	return NormalizeResponsesSupportMode(mode)
}
```

- [ ] **Step 4: 运行 GREEN**

Run:

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 ./internal/pkg/openai_compat -run TestResolveResponsesSupportMode
```

Expected: PASS。

## Task 2: 写 Chat shape 分流失败测试

**Files:**
- Modify: `backend/internal/service/openai_gateway_chat_completions_raw_test.go`
- Modify: `backend/internal/service/openai_gateway_chat_completions_test.go`

- [ ] **Step 1: 新增 APIKey supported=true 仍走 raw chat 的测试**

在 `TestForwardAsRawChatCompletions_ForcesStreamUsageUpstreamAndPassesUsageDownstream` 后新增：

```go
func TestForwardAsChatCompletions_APIKeyAutoChatShapeUsesRawChatWhenResponsesSupported(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"content":"ok"}}]}`,
		"",
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-5.5","choices":[],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_shape_raw"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}
	account := rawChatCompletionsTestAccount()
	account.Extra = map[string]any{
		"openai_responses_mode":      "auto",
		"openai_responses_supported": true,
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 9, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
	require.Equal(t, "http://upstream.example/v1/chat/completions", upstream.lastReq.URL.String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Bool())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	require.Contains(t, rec.Body.String(), `"choices"`)
	require.Contains(t, rec.Body.String(), "data: [DONE]")
}
```

- [ ] **Step 2: 更新未探测账号测试预期**

把 `TestForwardAsChatCompletions_UnknownResponsesSupportFallbackUsesVersionedChatURL` 改名为：

```go
func TestForwardAsChatCompletions_APIKeyAutoUnknownChatShapeUsesRawChatDirectly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"glm-4.5-air","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_raw_unknown_direct"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl_1","object":"chat.completion","model":"glm-4.5-air","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`,
		)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}
	account := rawChatCompletionsTestAccount()
	account.Credentials["base_url"] = "https://open.bigmodel.cn/api/paas/v4"

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://open.bigmodel.cn/api/paas/v4/chat/completions", upstream.requests[0].URL.String())
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"content":"ok"`)
}
```

- [ ] **Step 3: 保留 force_responses 下 prompt cache 行为**

在 `backend/internal/service/openai_gateway_chat_completions_test.go` 的 `TestForwardAsChatCompletions_APIKeyPropagatesPromptCacheKeyInResponsesBody` 中，把 `Extra` 改成：

```go
Extra: map[string]any{
	"openai_responses_mode":      "force_responses",
	"openai_responses_supported": true,
},
```

- [ ] **Step 4: 增加 Responses shape 保持 Responses 路径的测试**

在同一文件中追加：

```go
func TestForwardAsChatCompletions_APIKeyResponsesShapeKeepsResponsesPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.5","input":[{"role":"user","content":[{"type":"input_text","text":"hello"}]}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_responses_shape"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"stop before response parsing"}}`)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          4,
		Name:        "openai-compatible",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-compatible",
		},
		Extra: map[string]any{
			"openai_responses_mode":      "auto",
			"openai_responses_supported": true,
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.Error(t, err)
	require.Nil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://api.openai.com/v1/responses", upstream.lastReq.URL.String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "messages").Exists())
}
```

- [ ] **Step 5: 运行 RED**

Run:

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestForwardAsChatCompletions_APIKeyAutoChatShapeUsesRawChatWhenResponsesSupported|TestForwardAsChatCompletions_APIKeyAutoUnknownChatShapeUsesRawChatDirectly|TestForwardAsChatCompletions_APIKeyResponsesShapeKeepsResponsesPath|TestForwardAsChatCompletions_APIKeyPropagatesPromptCacheKeyInResponsesBody'
```

Expected: 新增的 Chat shape raw 测试 FAIL。当前代码在 `openai_responses_supported=true` 或未知状态下会先走 `/v1/responses`，不是期望的 `/v1/chat/completions`。

## Task 3: 实现协议感知分流

**Files:**
- Modify: `backend/internal/service/openai_gateway_chat_completions.go`

- [ ] **Step 1: 替换入口分流**

把当前入口处：

```go
	if account.Type == AccountTypeAPIKey && !openai_compat.ShouldUseResponsesAPI(account.Extra) {
		return s.forwardAsRawChatCompletions(ctx, c, account, body, defaultMappedModel)
	}
```

替换为：

```go
	if shouldForwardAPIKeyChatCompletionsAsRawChat(account, body) {
		return s.forwardAsRawChatCompletions(ctx, c, account, body, defaultMappedModel)
	}
```

- [ ] **Step 2: 新增分流 helper**

在 `ForwardAsChatCompletions()` 前新增：

```go
func shouldForwardAPIKeyChatCompletionsAsRawChat(account *Account, body []byte) bool {
	if account == nil || account.Type != AccountTypeAPIKey {
		return false
	}
	switch openai_compat.ResolveResponsesSupportMode(account.Extra) {
	case openai_compat.ResponsesSupportModeForceChatCompletions:
		return true
	case openai_compat.ResponsesSupportModeForceResponses:
		return false
	}
	if openAIChatCompletionsBodyHasMessages(body) {
		return true
	}
	return !openai_compat.ShouldUseResponsesAPI(account.Extra)
}

func openAIChatCompletionsBodyHasMessages(body []byte) bool {
	return gjson.GetBytes(body, "messages").Exists()
}
```

- [ ] **Step 3: 更新函数注释**

把 `ForwardAsChatCompletions` 上方“当前路由策略”段落改成：

```go
// 当前路由策略：
//   - OAuth 账号继续走 CC→Responses 转换，保留 ChatGPT/Codex 上游行为
//   - APIKey 账号 + force_chat_completions → 走 raw /v1/chat/completions
//   - APIKey 账号 + force_responses → 走 CC→Responses 转换
//   - APIKey 账号 + auto + Chat Completions shape（有 messages）→ 走 raw /v1/chat/completions
//   - APIKey 账号 + auto + Responses shape（无 messages、有 input）→ 保留 Responses shape 兼容路径
```

- [ ] **Step 4: 运行 GREEN**

Run:

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestForwardAsChatCompletions_APIKeyAutoChatShapeUsesRawChatWhenResponsesSupported|TestForwardAsChatCompletions_APIKeyAutoUnknownChatShapeUsesRawChatDirectly|TestForwardAsChatCompletions_APIKeyResponsesShapeKeepsResponsesPath|TestForwardAsChatCompletions_APIKeyPropagatesPromptCacheKeyInResponsesBody'
```

Expected: PASS。

## Task 4: 回归验证

**Files:**
- Test only

- [ ] **Step 1: 跑相关包单测**

Run:

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/pkg/openai_compat ./internal/service
```

Expected: PASS。

- [ ] **Step 2: 跑格式检查**

Run:

```bash
git diff --check
```

Expected: 无输出，exit 0。

## Task 5: 结果归档

**Files:**
- Create: `docs/ai/context/20260709-220635-961109198-chat-stream-502-root-fix-result_CN.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: 新建结果文档**

记录以下内容：

- RED 命令、失败测试名、失败原因。
- GREEN 命令和结果。
- 完整回归命令和结果。
- 行为变化：APIKey 自动模式下 Chat shape 入站走 raw `/v1/chat/completions`；Responses shape、OAuth、`force_responses` 保持旧路径。
- 未改 DB、未重启容器、未改 nginx、未发布公网。

- [ ] **Step 2: 更新 AGENTS.md**

在“最高优先级定论”顶部追加一条长期记忆，引用 result 文档。

- [ ] **Step 3: 最终检查**

Run:

```bash
git diff --check
```

Expected: 无输出，exit 0。

## 自检

- 设计要求均有任务覆盖：Chat shape 默认 raw、Responses shape 保留、OAuth 保留、force 模式保留、TDD 和回归验证。
- 没有占位步骤；每个代码改动步骤都给出可直接执行的片段。
- 函数名、测试名、路径和命令一致。
