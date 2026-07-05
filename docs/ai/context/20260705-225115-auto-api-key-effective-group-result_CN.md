# 自动 API Key Effective Group 本地实现结果

## 结论

- 已在本地分支 `codex/auto-api-key-effective-group` 完成自动 API Key 代码实现。
- 普通用户创建/编辑 API Key 时不再选择或提交 `group_id`，新 Key 走 `group_id=NULL`。
- OpenAI 请求进入网关后，后端会按用户当前权益实时解析 request-scoped `effective_group`：
  - active OpenAI 套餐优先，按日限额高、创建时间新、ID 大排序。
  - 无 active OpenAI 套餐但有可用 GPT/OpenAI 流量包时，走内部 `traffic-pack-openai` 分组。
  - 无套餐且无流量包时返回 `NO_OPENAI_ENTITLEMENT`。
- 新增迁移 `159_auto_api_key_effective_group.sql`：
  - 幂等创建/修正 `traffic-pack-openai` 内部分组。
  - 幂等绑定所有未删除 OpenAI 上游账号到该分组。
  - 将未删除的旧 OpenAI API Key 迁移为 `group_id=NULL`。
- 管理员后台固定分组能力未改。

## 代码范围

- 后端：
  - `backend/internal/service/effective_group_resolver.go`
  - `backend/internal/server/middleware/effective_group.go`
  - `backend/internal/server/routes/gateway.go`
  - `backend/internal/server/router.go`
  - `backend/internal/server/http.go`
  - `backend/internal/handler/api_key_handler.go`
  - `backend/internal/service/wire.go`
  - `backend/cmd/server/wire_gen.go`
  - `backend/migrations/159_auto_api_key_effective_group.sql`
  - 对应 resolver、middleware、handler、routes、migration 测试。
- 前端：
  - `frontend/src/views/user/KeysView.vue`
  - `frontend/src/views/user/__tests__/KeysView.spec.ts`
  - `frontend/src/components/Guide/steps.ts`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/i18n/locales/en.ts`

## 验证

- `go test -count=1 -tags=unit ./internal/service ./internal/server/middleware ./internal/server/routes ./internal/handler ./cmd/server -run 'TestEffectiveGroupResolver|TestResolveEffectiveGroupMiddleware|TestGatewayRoutes|TestAutomatic.*APIKeyRequest|Test.*'`
- `go test -count=1 -tags=integration ./internal/repository -run 'TestMigrationsRunner_AutoAPIKeyEffectiveGroupSeed|TestMigrationsRunner_IsIdempotent'`
- `pnpm vitest run src/views/user/__tests__/KeysView.spec.ts`
- `pnpm vue-tsc --noEmit --pretty false`
- `pnpm build`
- `git diff --check`

以上均通过。`pnpm build` 仍有既有 Vite chunk/dynamic import 警告，不影响构建成功。

## 未执行事项

- 未连接、修改、迁移或重启公网服务。
- 未重建或替换任何 Docker 容器。
- 未修改 nginx、18084/8080、公网数据库或 Redis。
- 运行态候选验证和公网发布仍需单独设计并经确认后执行。
