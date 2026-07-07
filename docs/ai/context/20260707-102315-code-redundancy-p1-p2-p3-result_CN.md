# 代码冗余治理 P1/P2/P3 实施结果

时间：2026-07-07 10:23 JST
分支：`codex/code-redundancy-p0-auto-key`

## 范围

本轮继续执行 `20260707-093635-code-redundancy-p1-p2-p3-implementation-plan_CN.md`，完成 P1/P2/P3 的本地代码治理；未连接公网 DB/Redis，未重启、替换或发布 18084 公网容器。

## 已完成

- P1 usage window policy：新增 `backend/internal/service/usagewindow/`，并将 quota view、billing cache、user subscription、subscription service 的窗口刷新/展示逻辑切到统一 helper。
- P1 部署 target：新增 `deploy/lib/common.sh`、`deploy/lib/targets.sh`，重写 `deploy/promote-sub2api-candidate.sh`，默认目标固定为 `public_candidate_18084`；`legacy_18080` 需要显式 `SUB2API_ALLOW_LEGACY_TARGET=true` 才允许。
- P1 API Key 认证核心：新增 `backend/internal/server/middleware/api_key_auth_core.go`，普通与 Google 中间件共享 Key 状态、用户状态、IP ACL、分组可用性等校验逻辑；Google IP ACL 有回归测试。
- P2 OpenAI-only route helper：`backend/internal/server/routes/gateway.go` 收敛 embeddings/images OpenAI-only gate，并补路由测试。
- P2 handler billing helper：新增 `backend/internal/handler/gateway_billing_helper.go`，HTTP gateway handler 复用计费准入与 `Retry-After` 处理；WebSocket close frame 语义不同，保留原路径。
- P2 前端账号弹窗：新增 `useAccountTestStream` 与 `useAccountStatsFormatters`，用户侧/管理侧账号测试弹窗和统计弹窗复用 SSE parser、request body builder 和统计格式化函数。
- P3 套餐 seed 模板化：新增 `backend/migrations/tools/generate-subscription-plan.sh` 与测试，只输出 SQL，不写入 migration 文件；支持按模板 group 生成套餐分组、订阅计划，并可选绑定 OpenAI 上游账号。
- P3 gateway header helper：新增 `backend/internal/handler/upstream_response_headers.go`，`/v1beta` 上游响应头透传复用 hop-by-hop 过滤 helper，保持原有“保留自定义头、跳过连接控制头”的行为。
- P3 Settings payload helper：新增 `buildOpenAIFastPolicySettingsPayload()`，SettingsView 只决定是否回写 OpenAI fast/flex policy，具体白名单 trim、fallback 字段条件化由纯函数处理。
- 顺手修复前端全量测试暴露的本分支相关问题：`KeysView` 自动 Key 测试同步新 create 签名；`AccountUsageCell` 不再无意义传入 `undefined` source，并在账号行 `updated_at` 变化时绕过缓存重拉 usage。

## 验证

已通过：

- `cd backend && go test -count=1 -tags=unit ./internal/server/middleware ./internal/server/routes ./internal/service ./internal/repository ./internal/handler/quotaview ./internal/handler`
- `cd frontend && pnpm run typecheck`
- `cd frontend && pnpm vitest run src/components/account/__tests__/AccountUsageCell.spec.ts src/views/user/__tests__/KeysView.spec.ts`
- `cd frontend && pnpm vitest run src/api/__tests__/settings.openaiFastPolicy.spec.ts src/views/admin/__tests__/SettingsView.spec.ts`
- `cd frontend && pnpm vitest run src/composables/__tests__/useAccountTestStream.spec.ts src/composables/__tests__/useAccountStatsFormatters.spec.ts src/components/account/__tests__/AccountTestModal.spec.ts src/components/admin/account/__tests__/AccountTestModal.spec.ts`
- `bash -n deploy/*.sh deploy/lib/*.sh backend/migrations/tools/*.sh`
- `bash deploy/promote-sub2api-candidate.test.sh`
- `bash backend/migrations/tools/generate-subscription-plan.test.sh`

完整前端 `cd frontend && pnpm vitest run` 仍未全绿：复跑后为 5 个文件、10 个测试失败，失败集中在未触碰的 `usePersistedPageSize`、`GroupDistributionChart`、`ModelDistributionChart`、`UsageView` 和 `UsageTable`。其中 AccountUsageCell 与 KeysView 的本分支相关失败已修复并通过。剩余失败未纳入本轮 P1/P2/P3 冗余治理，避免把无关行为修复混入本分支。

## 后续建议

- 单独开小分支处理完整 Vitest 剩余失败：page-size 应确认“系统默认值是否覆盖用户 localStorage”；图表应确认测试 fixture 缺少的 cost 字段是组件兼容问题还是测试数据过旧；UsageView/UsageTable 应确认历史图片行 `billing_mode` 推导契约。
- 若要发布本分支，仍建议在合并前先清掉上述前端全量测试红项，或者在 PR 中明确标注它们为既有失败并附本结果文档。
