# 自动 API Key 路由语义 P0 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: 使用 test-driven-development 执行本计划；每个生产代码改动前必须先写失败测试。步骤使用 checkbox 追踪。

**目标：** 收口自动 API Key 的路由语义，让 `group_id = NULL` 的普通用户 Key 在支持入口解析 effective group，在不支持入口返回明确错误，并移除前端用户 Key 创建 API 的 `groupId` 参数。

**架构：** 本批次只处理 P0，不触碰 API Key 认证核心去重、订阅窗口统一、部署脚本重构。后端新增小型路由策略与中间件选项，保持 fixed group Key 行为不变；前端只收口用户 API layer 的创建参数，不改管理员固定分组能力。

**技术栈：** Go + Gin + testify；Vue 3 + TypeScript；项目上下文文档位于 `docs/ai/context/`。

---

## 文件结构

- 修改：`backend/internal/server/middleware/effective_group.go`
  - 责任：为自动 Key 解析增加“当前入口是否支持自动 Key”的策略入口，并输出明确错误。
- 修改：`backend/internal/server/middleware/effective_group_test.go`
  - 责任：覆盖自动 Key 成功、fixed Key 跳过、未支持入口错误、force platform 场景。
- 修改：`backend/internal/server/routes/gateway.go`
  - 责任：对不支持自动 Key 的 Google/Gemini 原生入口挂明确拦截，避免落入传统未分组错误。
- 修改：`frontend/src/api/keys.ts`
  - 责任：用户创建 Key 不再接受或提交 `groupId`。
- 修改：`frontend/src/views/user/KeysView.vue`
  - 责任：适配新的 `keysAPI.create` 签名。
- 新增：`docs/ai/context/YYYYMMDD-HHMMSS-code-redundancy-p0-auto-key-result_CN.md`
  - 责任：记录实现、验证和未做事项。

## Task 1：自动 Key 未支持入口错误语义

- [ ] **Step 1：写失败测试**

在 `backend/internal/server/middleware/effective_group_test.go` 增加测试，表达自动 Key 在不支持入口被明确拒绝。

```go
func TestResolveEffectiveGroupMiddlewareRejectsUnsupportedAutomaticKeyEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := &service.APIKey{ID: 1, UserID: 62, Key: "auto-key", GroupID: nil, User: &service.User{ID: 62, Status: service.StatusActive}}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), original)
		c.Next()
	})
	router.Use(ResolveEffectiveGroupForSupportedEndpoints(&effectiveGroupResolverStub{
		result: &service.EffectiveGroupResult{Group: &service.Group{ID: 77, Platform: service.PlatformOpenAI}, Source: service.EffectiveGroupSourceTrafficPack},
	}, AnthropicErrorWriter))
	router.GET("/v1beta/models", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1beta/models", nil))

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "AUTO_KEY_UNSUPPORTED_ENDPOINT")
}
```

- [ ] **Step 2：确认测试失败**

运行：

```bash
cd backend && go test -count=1 -tags=unit ./internal/server/middleware -run TestResolveEffectiveGroupMiddlewareRejectsUnsupportedAutomaticKeyEndpoint -v
```

预期：失败，原因是 `ResolveEffectiveGroupForSupportedEndpoints` 尚不存在。

- [ ] **Step 3：最小实现**

在 `effective_group.go` 增加：

```go
type AutomaticKeyEndpointPolicy func(*gin.Context) (platform string, supported bool)

func ResolveEffectiveGroupForSupportedEndpoints(resolver EffectiveGroupResolver, writeError GatewayErrorWriter) gin.HandlerFunc {
	return ResolveEffectiveGroupWithPolicy(resolver, writeError, DefaultAutomaticKeyEndpointPolicy)
}

func ResolveEffectiveGroupWithPolicy(resolver EffectiveGroupResolver, writeError GatewayErrorWriter, policy AutomaticKeyEndpointPolicy) gin.HandlerFunc {
	// 复用当前 ResolveEffectiveGroup 主体；对 GroupID == nil 时先调用 policy。
	// supported=false 时写 AUTO_KEY_UNSUPPORTED_ENDPOINT 并 Abort。
}

func DefaultAutomaticKeyEndpointPolicy(c *gin.Context) (string, bool) {
	forcePlatform, _ := c.Request.Context().Value(ctxkey.ForcePlatform).(string)
	if forcePlatform != "" {
		return forcePlatform, forcePlatform == service.PlatformOpenAI
	}
	path := c.Request.URL.Path
	switch {
	case strings.HasPrefix(path, "/v1beta"), strings.HasPrefix(path, "/antigravity/"):
		return "", false
	case strings.Contains(path, "/responses"),
		strings.Contains(path, "/chat/completions"),
		strings.Contains(path, "/embeddings"),
		strings.Contains(path, "/images/"),
		strings.Contains(path, "/messages"):
		return service.PlatformOpenAI, true
	default:
		return "", false
	}
}
```

保留 `ResolveEffectiveGroup` 作为兼容包装。

- [ ] **Step 4：确认测试通过**

运行同一条 `go test`，预期 PASS。

## Task 2：路由挂载策略

- [ ] **Step 1：写失败测试**

在 `effective_group_test.go` 增加 Google writer 场景，证明 `/v1beta` 自动 Key 返回 Google 风格明确错误。

```go
func TestResolveEffectiveGroupMiddlewareWritesGoogleErrorForUnsupportedAutomaticKeyEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := &service.APIKey{ID: 1, UserID: 62, Key: "auto-key", GroupID: nil, User: &service.User{ID: 62, Status: service.StatusActive}}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), original)
		c.Next()
	})
	router.Use(ResolveEffectiveGroupForSupportedEndpoints(&effectiveGroupResolverStub{}, GoogleErrorWriter))
	router.GET("/v1beta/models", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1beta/models", nil))

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "AUTO_KEY_UNSUPPORTED_ENDPOINT")
	require.Contains(t, rec.Body.String(), "PERMISSION_DENIED")
}
```

- [ ] **Step 2：确认测试失败**

运行：

```bash
cd backend && go test -count=1 -tags=unit ./internal/server/middleware -run 'TestResolveEffectiveGroupMiddleware.*UnsupportedAutomaticKeyEndpoint' -v
```

预期：Google 错误内容不符合新语义。

- [ ] **Step 3：最小实现**

调整 `effectiveGroupErrorResponse` 或新增 unsupported writer 分支，让 `AUTO_KEY_UNSUPPORTED_ENDPOINT` 出现在错误消息中。

在 `routes/gateway.go`：

- `resolveGroupAnthropic := middleware.ResolveEffectiveGroupForSupportedEndpoints(...)`
- 新增 `resolveGroupGoogle := middleware.ResolveEffectiveGroupForSupportedEndpoints(..., middleware.GoogleErrorWriter)`
- `/v1beta` 与 `/antigravity/v1beta` 在 `APIKeyAuthWithSubscriptionGoogle` 后先挂 `resolveGroupGoogle`，再挂 `requireGroupGoogle`。

- [ ] **Step 4：确认测试通过**

运行 middleware 单测，预期 PASS。

## Task 3：前端用户 Key 创建接口移除 groupId

- [ ] **Step 1：写失败检查**

运行 TypeScript 检查，预期当前代码仍允许旧签名，需要修改调用和定义后重新验证。

```bash
cd frontend && pnpm exec vue-tsc --noEmit
```

若项目没有该命令，使用现有前端测试命令。

- [ ] **Step 2：最小实现**

修改 `frontend/src/api/keys.ts`：

```ts
export async function create(
  name: string,
  customKey?: string,
  ipWhitelist?: string[],
  ipBlacklist?: string[],
  quota?: number,
  expiresInDays?: number,
  rateLimitData?: { rate_limit_5h?: number; rate_limit_1d?: number; rate_limit_7d?: number }
): Promise<ApiKey> {
  const payload: CreateApiKeyRequest = { name }
  // 不再写 group_id，普通用户 Key 统一由后端运行时解析 effective group。
}
```

修改 `frontend/src/views/user/KeysView.vue` 中 `keysAPI.create(...)` 调用，删除原第二个 `groupId` 参数，保持其他参数顺序正确。

- [ ] **Step 3：验证前端类型**

运行：

```bash
cd frontend && pnpm exec vue-tsc --noEmit
```

预期 PASS。

## Task 4：批次验证和结果上下文

- [ ] **Step 1：后端验证**

```bash
cd backend && go test -count=1 -tags=unit ./internal/server/middleware
```

预期 PASS。

- [ ] **Step 2：前端验证**

```bash
cd frontend && pnpm exec vue-tsc --noEmit
```

预期 PASS。

- [ ] **Step 3：文档自查**

```bash
rg -n "占位检查关键词|敏感密钥正则" docs/ai/context/20260707-092407-code-redundancy-p0-auto-key-implementation-plan_CN.md docs/ai/context/*code-redundancy* || true
git diff --check
```

预期无占位、无敏感 Key、无空白错误。

- [ ] **Step 4：结果上下文**

新增结果文档，记录：

- 分支名。
- 已实现范围。
- 未做范围：P1/P2/P3 暂未实施。
- 验证命令和结果。
- 是否触碰运行态：否。
