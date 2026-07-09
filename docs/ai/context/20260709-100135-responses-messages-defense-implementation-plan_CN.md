# Responses Messages Defense Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让正式 OpenAI `/v1/responses` 在收到 Chat Completions `messages` body 时本地返回清晰 400，而不是转发上游后表现成 502。

**Architecture:** 在 `OpenAIGatewayHandler.Responses()` 的基础 JSON、model、stream 校验之后增加最小 shape guard：有 `messages` 且没有 `input` 时返回 `invalid_request_error`。不复用 Chat Completions 转换链路，不改变 `/v1/chat/completions`，不改 Claude/Anthropic 兼容 `GatewayHandler.Responses()`。

**Tech Stack:** Go、Gin、gjson、现有 handler 单测、`go test`。

---

## 文件结构

- Modify: `backend/internal/handler/openai_gateway_handler_test.go`
  - 职责：新增 handler 层回归测试，证明 `/v1/responses` + `messages` 在进入依赖检查/上游转发前返回 400。
- Modify: `backend/internal/handler/openai_gateway_handler.go`
  - 职责：新增 Responses body shape guard 与错误消息常量/函数。
- Create: `docs/ai/context/20260709-100135-responses-messages-defense-result_CN.md`
  - 职责：记录 TDD、验证命令、影响范围和未触碰项。
- Modify: `AGENTS.md`
  - 职责：追加本次修复结果长期记忆。

## Task 1: 写失败测试

**Files:**
- Modify: `backend/internal/handler/openai_gateway_handler_test.go`

- [ ] **Step 1: 新增测试函数**

在 `TestReadRequestBodyWithPrealloc_MaxBytesError` 后追加：

```go
func TestOpenAIResponsesRejectsMessagesWithoutInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/v1/responses",
		strings.NewReader(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`),
	)
	groupID := int64(5)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
		ID:      32,
		UserID:  13,
		GroupID: &groupID,
	})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 13, Concurrency: 1})

	h := &OpenAIGatewayHandler{
		gatewayService:      &service.OpenAIGatewayService{},
		billingCacheService: &service.BillingCacheService{},
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(nil), SSEPingFormatComment, 0),
	}
	h.Responses(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid_request_error", gjson.GetBytes(rec.Body.Bytes(), "error.type").String())
	require.Equal(t, "/v1/responses expects input; use /v1/chat/completions for messages", gjson.GetBytes(rec.Body.Bytes(), "error.message").String())
}
```

- [ ] **Step 2: 运行 RED**

Run:

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run TestOpenAIResponsesRejectsMessagesWithoutInput
```

Expected: FAIL。当前代码没有本地 shape guard，会继续执行后续网关流程并返回非 400，或在缺少真实仓储/上游依赖时进入服务不可用错误；这证明测试覆盖的是新行为。

## Task 2: 实现最小 guard

**Files:**
- Modify: `backend/internal/handler/openai_gateway_handler.go`

- [ ] **Step 1: 新增错误消息常量**

在 `type openAIModelBodyReplaceFunc` 附近增加：

```go
const openAIResponsesMessagesWithoutInputMessage = "/v1/responses expects input; use /v1/chat/completions for messages"
```

- [ ] **Step 2: 新增 shape guard helper**

在 `openAIModelMappedBody()` 后增加：

```go
func openAIResponsesHasMessagesWithoutInput(body []byte) bool {
	return gjson.GetBytes(body, "messages").Exists() && !gjson.GetBytes(body, "input").Exists()
}
```

- [ ] **Step 3: 在 Responses handler 中调用 guard**

在 `reqLog = reqLog.With(...)` 后、`previousResponseID := ...` 前插入：

```go
	if openAIResponsesHasMessagesWithoutInput(body) {
		reqLog.Warn("openai.request_validation_failed",
			zap.String("reason", "responses_messages_without_input"),
		)
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", openAIResponsesMessagesWithoutInputMessage)
		return
	}
```

- [ ] **Step 4: 运行 GREEN**

Run:

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run TestOpenAIResponsesRejectsMessagesWithoutInput
```

Expected: PASS。

## Task 3: 回归验证

**Files:**
- Test only

- [ ] **Step 1: 跑 handler 目标包**

Run:

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler
```

Expected: PASS。

- [ ] **Step 2: 跑格式检查**

Run:

```bash
git diff --check
```

Expected: 无输出，exit 0。

## Task 4: 结果归档

**Files:**
- Create: `docs/ai/context/20260709-100135-responses-messages-defense-result_CN.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: 新建结果文档**

写明：

- RED 命令与失败原因。
- GREEN 命令与结果。
- 回归验证命令与结果。
- 只改本地代码和文档，未改 DB、nginx、Redis、容器。
- 行为变化：`/v1/responses` + `messages` 本地 400；`/v1/chat/completions` 转换链路不变。

- [ ] **Step 2: 更新 AGENTS.md**

在“最高优先级定论”顶部追加本次修复结果，引用 result 文档。

- [ ] **Step 3: 最终检查**

Run:

```bash
git diff --check
```

Expected: 无输出，exit 0。

## 自检

- 设计文档中的所有要求都有对应任务：本地 400、非自动转换、不改 chat 路径、不改运行态、TDD 验证。
- 无 `TBD`、`TODO`、空泛“补测试”占位。
- 函数名、常量名和测试命令在各任务中一致。
