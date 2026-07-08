# Formal V1 Only API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 禁用所有裸 OpenAI/Codex 兼容入口，让正式模型 API 只通过 `/v1/*` 访问，裸路径统一返回 `400 INVALID_BASE_URL`。

**Architecture:** 后端和 Nginx 双层拦截同一批裸路径，避免公网和直连 `127.0.0.1:18084` 行为不一致。正式 `/v1/*`、`/api/*`、`/v1beta/*`、`/antigravity/*` 不变；不处理静态资源缓存和 `sub_filter` 历史兼容问题。

**Tech Stack:** Go、Gin、Nginx、`go test`、`curl`。

---

## 约束

- 本项目约定计划文档放在 `docs/ai/context/`，不使用 `docs/superpowers/plans/`。
- 默认不提交 git，除非用户明确要求。
- 不重构无关 Nginx 配置，例如 `sub_filter`、`Clear-Site-Data`、timeout、buffering。
- 不暴露完整 API Key、token 或敏感配置。

## 文件结构

- 修改：`backend/internal/server/routes/gateway_test.go`
  - 负责验证裸 OpenAI/Codex 路径返回 `400 INVALID_BASE_URL`，正式 `/v1/*` 路由仍存在。
- 修改：`backend/internal/server/routes/gateway.go`
  - 负责移除裸路径兼容转发，新增统一 `invalidBaseURLHandler` 和裸路径注册。
- 修改：`backend/internal/handler/stream_error_event.go`
  - 负责把 Responses 流式错误判定收敛到正式 `/v1/responses` 和内部 handler 测试路径，不再把裸 `/responses` 当成有效 Responses API。
- 修改：`backend/internal/handler/stream_error_event_test.go`
  - 负责更新历史裸 `/responses` 回归测试，避免继续要求裸路径发 `response.failed`。
- 修改：`/opt/homebrew/etc/nginx/servers/aaccx-root.conf`
  - 负责让 `aaccx.pw` 下的裸 OpenAI/Codex 路径直接返回 `400`。
- 修改：`/opt/homebrew/etc/nginx/servers/cliproxy.conf`
  - 负责让 `api.aaccx.pw` 下的裸 OpenAI/Codex 路径直接返回 `400`。
- 新建：`docs/ai/context/YYYYMMDD-HHMMSS-formal-v1-only-api-result_CN.md`
  - 负责记录实施结果、验证命令和残留风险。
- 修改：`AGENTS.md`
  - 追加最终实施结果记忆。

## Task 1: 后端路由测试先失败

**Files:**
- Modify: `backend/internal/server/routes/gateway_test.go`

- [ ] **Step 1: 替换旧的裸 Responses 兼容测试**

把 `TestGatewayRoutesOpenAIResponsesCompactPathIsRegistered` 和 `TestGatewayRoutesOpenAIImagesPathsAreRegistered` 替换为下面两组测试：

```go
func TestGatewayRoutesRejectBareOpenAICompatiblePaths(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/models?client_version=0.142.5"},
		{http.MethodPost, "/responses"},
		{http.MethodPost, "/responses/compact"},
		{http.MethodGet, "/responses"},
		{http.MethodPost, "/chat/completions"},
		{http.MethodPost, "/embeddings"},
		{http.MethodPost, "/images/generations"},
		{http.MethodPost, "/images/edits"},
		{http.MethodPost, "/backend-api/codex/responses"},
		{http.MethodPost, "/backend-api/codex/responses/compact"},
		{http.MethodGet, "/backend-api/codex/responses"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"model":"gpt-5"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), `"code":"INVALID_BASE_URL"`)
			require.Contains(t, w.Body.String(), `https://api.aaccx.pw/v1`)
		})
	}
}

func TestGatewayRoutesKeepFormalV1OpenAIPathsRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/models"},
		{http.MethodPost, "/v1/responses"},
		{http.MethodPost, "/v1/responses/compact"},
		{http.MethodPost, "/v1/chat/completions"},
		{http.MethodPost, "/v1/embeddings"},
		{http.MethodPost, "/v1/images/generations"},
		{http.MethodPost, "/v1/images/edits"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"model":"gpt-5"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			require.NotEqual(t, http.StatusBadRequest, w.Code)
			require.NotContains(t, w.Body.String(), `"code":"INVALID_BASE_URL"`)
		})
	}
}
```

- [ ] **Step 2: 运行失败测试**

Run:

```bash
go test -count=1 -tags=unit ./internal/server/routes
```

Expected: 失败。至少裸 `/responses`、`/responses/compact`、`/images/generations` 仍命中旧 handler 或返回非 `400 INVALID_BASE_URL`。

## Task 2: 后端路由实现裸路径 400

**Files:**
- Modify: `backend/internal/server/routes/gateway.go`

- [ ] **Step 1: 新增统一错误响应 helper**

在 `getGroupPlatform` 之前增加：

```go
func invalidBaseURLHandler(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error": gin.H{
			"type":    "invalid_request_error",
			"code":    "INVALID_BASE_URL",
			"message": "Invalid API path. Use base_url=https://api.aaccx.pw/v1 and the corresponding /v1 endpoint.",
		},
	})
}

func registerInvalidBaseURLRoutes(r *gin.Engine) {
	r.Any("/models", invalidBaseURLHandler)
	r.Any("/responses", invalidBaseURLHandler)
	r.Any("/responses/*subpath", invalidBaseURLHandler)
	r.Any("/chat/completions", invalidBaseURLHandler)
	r.Any("/embeddings", invalidBaseURLHandler)
	r.Any("/images/generations", invalidBaseURLHandler)
	r.Any("/images/edits", invalidBaseURLHandler)
	r.Any("/backend-api/codex/responses", invalidBaseURLHandler)
	r.Any("/backend-api/codex/responses/*subpath", invalidBaseURLHandler)
}
```

- [ ] **Step 2: 删除裸路径兼容转发注册**

删除 `RegisterGatewayRoutes` 中这段旧代码：

```go
// OpenAI Responses API（不带v1前缀的别名）— auto-route based on group platform
responsesHandler := func(c *gin.Context) {
	if getGroupPlatform(c) == service.PlatformOpenAI {
		h.OpenAIGateway.Responses(c)
		return
	}
	h.Gateway.Responses(c)
}
r.POST("/responses", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), resolveGroupAnthropic, requireGroupAnthropic, responsesHandler)
r.POST("/responses/*subpath", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), resolveGroupAnthropic, requireGroupAnthropic, responsesHandler)
r.GET("/responses", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), resolveGroupAnthropic, requireGroupAnthropic, h.OpenAIGateway.ResponsesWebSocket)
codexDirect := r.Group("/backend-api/codex")
codexDirect.Use(bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), resolveGroupAnthropic, requireGroupAnthropic)
{
	codexDirect.POST("/responses", responsesHandler)
	codexDirect.POST("/responses/*subpath", responsesHandler)
	codexDirect.GET("/responses", h.OpenAIGateway.ResponsesWebSocket)
}
// OpenAI Chat Completions API（不带v1前缀的别名）— auto-route based on group platform
r.POST("/chat/completions", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), resolveGroupAnthropic, requireGroupAnthropic, func(c *gin.Context) {
	if getGroupPlatform(c) == service.PlatformOpenAI {
		h.OpenAIGateway.ChatCompletions(c)
		return
	}
	h.Gateway.ChatCompletions(c)
})
r.POST("/embeddings", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), resolveGroupAnthropic, requireGroupAnthropic, func(c *gin.Context) {
	openAIOnly("Embeddings", h.OpenAIGateway.Embeddings)(c)
})
r.POST("/images/generations", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), resolveGroupAnthropic, requireGroupAnthropic, func(c *gin.Context) {
	openAIOnly("Images", h.OpenAIGateway.Images)(c)
})
r.POST("/images/edits", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), resolveGroupAnthropic, requireGroupAnthropic, func(c *gin.Context) {
	openAIOnly("Images", h.OpenAIGateway.Images)(c)
})
```

在删除位置补入：

```go
registerInvalidBaseURLRoutes(r)
```

- [ ] **Step 3: 运行后端路由测试**

Run:

```bash
go test -count=1 -tags=unit ./internal/server/routes
```

Expected: PASS。

## Task 3: 收敛 Responses 流式错误判定测试

**Files:**
- Modify: `backend/internal/handler/stream_error_event_test.go`
- Modify: `backend/internal/handler/stream_error_event.go`

- [ ] **Step 1: 修改测试为只承认正式 `/v1/responses` 和内部 handler 路径**

把 `TestInboundIsResponses_CoversAllRoutes` 替换为：

```go
func TestInboundIsResponses_CoversFormalResponsesRoutes(t *testing.T) {
	cases := []struct {
		route string
		want  bool
	}{
		{"/v1/responses", true},
		{"/v1/responses/compact", true},
		{"/openai/v1/responses", true},
		{"/openai/v1/responses/compact", true},
		{"/responses", false},
		{"/responses/compact", false},
		{"/backend-api/codex/responses", false},
		{"/backend-api/codex/responses/compact", false},
		{"/v1/chat/completions", false},
		{"/v1/messages", false},
		{"/", false},
		{"/responses-fake", false},
	}
	for _, tc := range cases {
		t.Run(tc.route, func(t *testing.T) {
			c, _ := newGinContextForEndpoint(t, tc.route)
			assert.Equal(t, tc.want, inboundIsResponses(c), "route=%q", tc.route)
		})
	}
}
```

把 `TestInboundIsResponses_FallsBackToURLPath` 中的请求路径改成正式路径：

```go
c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
assert.True(t, inboundIsResponses(c), "URL.Path fallback must work when FullPath is empty")
```

删除 `TestOpenAIHandleStreamingAwareError_BareResponsesRouteEmitsResponseFailed`，因为裸 `/responses` 不再是有效 Responses API。

- [ ] **Step 2: 运行失败测试**

Run:

```bash
go test -count=1 -tags=unit ./internal/handler -run 'TestInboundIsResponses|TestOpenAIHandleStreamingAwareError'
```

Expected: 失败。`inboundIsResponses` 仍会把裸 `/responses` 或 `/backend-api/codex/responses` 判为 true。

- [ ] **Step 3: 修改 `inboundIsResponses`**

把 `inboundIsResponses` 里的返回逻辑改成：

```go
func inboundIsResponses(c *gin.Context) bool {
	if c == nil {
		return false
	}
	p := strings.TrimRight(c.FullPath(), "/")
	if p == "" && c.Request != nil && c.Request.URL != nil {
		p = strings.TrimRight(c.Request.URL.Path, "/")
	}
	if p == "" {
		return false
	}

	return p == "/v1/responses" ||
		strings.HasPrefix(p, "/v1/responses/") ||
		p == "/openai/v1/responses" ||
		strings.HasPrefix(p, "/openai/v1/responses/")
}
```

同步更新函数注释，删除裸 `/responses` 和 `/backend-api/codex/responses` 作为有效路由的描述，改为说明这两个路径已由 `INVALID_BASE_URL` 处理。

- [ ] **Step 4: 运行 handler 目标测试**

Run:

```bash
go test -count=1 -tags=unit ./internal/handler -run 'TestInboundIsResponses|TestOpenAIHandleStreamingAwareError'
```

Expected: PASS。

## Task 4: Nginx 配置裸路径 400

**Files:**
- Modify: `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`
- Modify: `/opt/homebrew/etc/nginx/servers/cliproxy.conf`

- [ ] **Step 1: 修改 `aaccx-root.conf` 裸路径 location**

把原来的代理 location：

```nginx
# Codex/OpenAI 兼容客户端可能把 Base URL 配成 https://aaccx.pw，
# 这些裸路径必须进入 Sub2API，不能落到 yui.web。
location ~ ^/(responses(/.*)?|backend-api/codex/responses(/.*)?|chat/completions|embeddings|images/(generations|edits))$ {
    proxy_pass http://127.0.0.1:18084;
    proxy_http_version 1.1;
    proxy_request_buffering off;

    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 86400;
}
```

替换为：

```nginx
# 裸 OpenAI/Codex 路径不是正式 API；客户端必须使用 https://api.aaccx.pw/v1。
location ~ ^/(models|responses(/.*)?|backend-api/codex/responses(/.*)?|chat/completions|embeddings|images/(generations|edits))$ {
    default_type application/json;
    return 400 '{"error":{"type":"invalid_request_error","code":"INVALID_BASE_URL","message":"Invalid API path. Use base_url=https://api.aaccx.pw/v1 and the corresponding /v1 endpoint."}}';
}
```

- [ ] **Step 2: 修改 `cliproxy.conf`，在 `location /` 前增加裸路径 400**

在 `location = /index.html { ... }` 和 `location / { ... }` 之间增加：

```nginx
# 裸 OpenAI/Codex 路径不是正式 API；客户端必须使用 https://api.aaccx.pw/v1。
location ~ ^/(models|responses(/.*)?|backend-api/codex/responses(/.*)?|chat/completions|embeddings|images/(generations|edits))$ {
    default_type application/json;
    return 400 '{"error":{"type":"invalid_request_error","code":"INVALID_BASE_URL","message":"Invalid API path. Use base_url=https://api.aaccx.pw/v1 and the corresponding /v1 endpoint."}}';
}
```

- [ ] **Step 3: 检查 Nginx 配置语法**

Run:

```bash
nginx -t
```

Expected:

```text
nginx: the configuration file /opt/homebrew/etc/nginx/nginx.conf syntax is ok
nginx: configuration file /opt/homebrew/etc/nginx/nginx.conf test is successful
```

- [ ] **Step 4: reload Nginx**

Run:

```bash
nginx -s reload
```

Expected: 命令退出码为 0；`/opt/homebrew/var/log/nginx/error.log` 不出现新的配置加载错误。

## Task 5: 回归验证

**Files:**
- No source changes

- [ ] **Step 1: 运行后端目标测试**

Run:

```bash
go test -count=1 -tags=unit ./internal/server/routes
go test -count=1 -tags=unit ./internal/handler -run 'TestInboundIsResponses|TestOpenAIHandleStreamingAwareError'
```

Expected: 两条命令都 PASS。

- [ ] **Step 2: 运行后端编译门**

Run:

```bash
go test -count=1 ./cmd/server
```

Expected: PASS。

- [ ] **Step 3: 验证后端直连 18084**

Run:

```bash
curl -sS -o /tmp/sub2api-bare-responses.json -w 'bare_responses=%{http_code}\n' \
  -X POST http://127.0.0.1:18084/responses \
  -H 'Content-Type: application/json' \
  --data '{"model":"gpt-5"}'

cat /tmp/sub2api-bare-responses.json
```

Expected:

```text
bare_responses=400
```

Body contains:

```json
"code":"INVALID_BASE_URL"
```

- [ ] **Step 4: 验证 Nginx `api.aaccx.pw` Host**

Run:

```bash
for path in /models /responses /responses/compact /chat/completions /embeddings /images/generations /images/edits /backend-api/codex/responses /backend-api/codex/responses/compact; do
  curl -sS -o /tmp/sub2api-nginx-api.json -w "api${path}=%{http_code}\n" \
    -X POST "http://127.0.0.1:8080${path}" \
    -H 'Host: api.aaccx.pw' \
    -H 'Content-Type: application/json' \
    --data '{"model":"gpt-5"}'
  grep -q '"code":"INVALID_BASE_URL"' /tmp/sub2api-nginx-api.json
done
```

Expected: 每行状态码都是 `400`，`grep` 不失败。

- [ ] **Step 5: 验证 Nginx `aaccx.pw` Host**

Run:

```bash
for path in /models /responses /responses/compact /chat/completions /embeddings /images/generations /images/edits /backend-api/codex/responses /backend-api/codex/responses/compact; do
  curl -sS -o /tmp/sub2api-nginx-root.json -w "root${path}=%{http_code}\n" \
    -X POST "http://127.0.0.1:8080${path}" \
    -H 'Host: aaccx.pw' \
    -H 'Content-Type: application/json' \
    --data '{"model":"gpt-5"}'
  grep -q '"code":"INVALID_BASE_URL"' /tmp/sub2api-nginx-root.json
done
```

Expected: 每行状态码都是 `400`，`grep` 不失败。

- [ ] **Step 6: 验证正式 `/v1/*` 仍走 Sub2API**

Run:

```bash
curl -sS -o /tmp/sub2api-v1-responses.json -w 'v1_responses=%{http_code}\n' \
  -X POST http://127.0.0.1:8080/v1/responses \
  -H 'Host: api.aaccx.pw' \
  -H 'Content-Type: application/json' \
  --data '{"model":"gpt-5"}'

cat /tmp/sub2api-v1-responses.json
```

Expected: 不返回 `400 INVALID_BASE_URL`。未带 Key 时应返回 Sub2API 鉴权错误，通常是 `401` 或受控鉴权错误。

- [ ] **Step 7: 验证 health 和控制台不受影响**

Run:

```bash
curl -fsS -o /dev/null -w 'health=%{http_code}\n' -H 'Host: api.aaccx.pw' http://127.0.0.1:8080/health
curl -fsS -o /dev/null -w 'dashboard=%{http_code}\n' -H 'Host: aaccx.pw' http://127.0.0.1:8080/dashboard
curl -fsS -o /dev/null -w 'usage_guide=%{http_code}\n' -H 'Host: aaccx.pw' http://127.0.0.1:8080/usage-guide
```

Expected:

```text
health=200
dashboard=200
usage_guide=200
```

## Task 6: 记录结果

**Files:**
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-formal-v1-only-api-result_CN.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: 新建结果文档**

文档必须包含：

```markdown
# 只保留 /v1/* 为正式模型 API 实施结果

## 改动

- 后端裸 OpenAI/Codex 路径返回 `400 INVALID_BASE_URL`。
- Nginx `api.aaccx.pw` 与 `aaccx.pw` 裸 OpenAI/Codex 路径返回 `400 INVALID_BASE_URL`。
- 正式 `/v1/*` 不变。

## 验证

- `go test -count=1 -tags=unit ./internal/server/routes`
- `go test -count=1 -tags=unit ./internal/handler -run 'TestInboundIsResponses|TestOpenAIHandleStreamingAwareError'`
- `go test -count=1 ./cmd/server`
- `nginx -t`
- `nginx -s reload`
- 本地 `8080` Host 验证
- 后端 `18084` 直连验证

## 风险

- 已配置裸 Base URL 的客户端会立即收到 `400 INVALID_BASE_URL`。
- 用户需要把 Base URL 改为 `https://api.aaccx.pw/v1`。
```

- [ ] **Step 2: 更新 AGENTS.md**

在“最高优先级定论”顶部追加一条：

```markdown
- 2026-07-08 已实施 API 入口规范化：正式模型 API 只保留 `/v1/*`；裸 `/responses`、`/chat/completions`、`/embeddings`、`/images/*`、`/models` 和 `/backend-api/codex/responses` 在 Nginx 与后端均返回 `400 INVALID_BASE_URL`，提示使用 `base_url=https://api.aaccx.pw/v1`。验证见 `docs/ai/context/YYYYMMDD-HHMMSS-formal-v1-only-api-result_CN.md`。
```

- [ ] **Step 3: 最终格式检查**

Run:

```bash
git diff --check
```

Expected: 无输出，退出码 0。

## Self-Review

- Spec coverage: 覆盖了 Nginx 和后端双层禁用、`400 INVALID_BASE_URL`、`/backend-api/codex/responses` 禁用、`/v1/*` 保留、验证与文档记录。
- Placeholder scan: 本计划没有 `TBD`、`TODO` 或“类似上一步”式占位。
- Type consistency: 错误码统一为 `INVALID_BASE_URL`；HTTP 状态统一为 `400`；正式 Base URL 统一为 `https://api.aaccx.pw/v1`。
