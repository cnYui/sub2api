# 2026-07-07 自动 API Key 路由语义 P0 实施结果

## 范围

- 分支：`codex/code-redundancy-p0-auto-key`
- 来源设计：`docs/ai/context/20260707-091540-code-redundancy-design_CN.md`
- 实施计划：`docs/ai/context/20260707-092407-code-redundancy-p0-auto-key-implementation-plan_CN.md`
- 本轮只实施 P0 自动 API Key 路由语义，不处理 P1 API Key 认证核心去重、订阅窗口统一、部署脚本 target 收口，也不处理 P2/P3。

## 已完成

- 后端 `ResolveEffectiveGroup` 保留为兼容包装；新增 `ResolveEffectiveGroupForSupportedEndpoints` 与 `ResolveEffectiveGroupWithPolicy`。
- 新增 `DefaultAutomaticKeyEndpointPolicy`：
  - OpenAI/Claude 兼容入口里的 `/responses`、`/chat/completions`、`/embeddings`、`/images/*`、`/messages` 支持自动 Key，解析到 OpenAI effective group。
  - `/v1beta` 与 `/antigravity/*` 默认不支持自动 Key。
  - `ForcePlatform(openai)` 支持自动 Key，`ForcePlatform(antigravity)` 不支持自动 Key。
- 自动 Key 访问不支持入口时返回明确消息：`AUTO_KEY_UNSUPPORTED_ENDPOINT: Automatic API Key is not supported for this endpoint.`，不再落入传统未分组 Key 错误。
- `gateway.go` 已将 `/v1`、裸 OpenAI 入口、`/backend-api/codex`、`/v1beta`、`/antigravity/models`、`/antigravity/v1`、`/antigravity/v1beta` 接入新 wrapper；fixed group Key 仍直接跳过 resolver。
- 前端用户 Key 创建 API 移除 `groupId` 参数，`CreateApiKeyRequest` 不再包含用户创建用的 `group_id` 字段；`KeysView.vue` 调用已适配。
- 为旧测试 stub 补齐 `RefreshExpiredUsageWindows` 方法，修复近期接口变更后的测试编译缺口。

## TDD 记录

- 后端 RED：
  - 先新增 `TestResolveEffectiveGroupMiddlewareRejectsUnsupportedAutomaticKeyEndpoint` 与 `TestResolveEffectiveGroupMiddlewareWritesGoogleErrorForUnsupportedAutomaticKeyEndpoint`。
  - 初次在仓库根目录运行 Go 测试失败，原因是根目录不是 Go module；改在 `backend/` 下运行。
  - 修复旧测试 stub 后，失败原因收敛为 `ResolveEffectiveGroupForSupportedEndpoints` 未定义，符合预期。
- 前端 RED：
  - 新增 `frontend/src/api/__tests__/keys.spec.ts`。
  - 旧实现会把第二个参数 `customKey` 误当 `group_id`，测试按预期失败。

## 验证

- `cd backend && go test -count=1 -tags=unit ./internal/server/middleware`：PASS
- `cd backend && go test -count=1 -tags=unit ./internal/server/routes`：PASS
- `cd frontend && pnpm vitest run src/api/__tests__/keys.spec.ts`：PASS
- `cd frontend && pnpm run typecheck`：PASS

## 未触碰

- 未连接、读取、迁移、重启或替换公网 18084 运行态。
- 未修改数据库 schema 或 migration。
- 未改 API Key 认证核心去重。
- 未改订阅 usage window policy。
- 未改部署脚本、handler 计费准入、SettingsView、gateway service 工具层或套餐 seed 模板。

## 后续建议

1. 下一批先做 P1 API Key 认证核心去重，避免普通和 Google 中间件继续漂移。
2. 再做 P1 usage window policy 统一，保证展示、扣费、Retry-After 同口径。
3. 部署前另写发布计划；本轮代码完成不等于可直接替换公网容器。
