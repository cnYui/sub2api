# 2026-07-22 周滚动订阅额度可见缺口补齐结果

## 已完成

- 后端 `/api/v1/payment/plans` 与 `/api/v1/payment/checkout-info` 对公共 Codex 套餐统一返回 28 天 / 周额度 / 每 7 天刷新文案，旧 description/features/product_name 不再直接泄漏。
- 后端 `/v1/usage` 的公共 Codex 周滚动订阅不再回退到 group 完整周额度；没有当前窗口事实时按不可用/0 处理。
- `backend/migrations/tools/generate-subscription-plan.sh` 已切到 weekly/28 天口径，并保留 `--daily-limit-usd` 兼容别名。
- 前端购买确认页不再直接展示 `selectedPlan.description`，改为公共 Codex 周订阅统一文案兜底。
- 前端管理端套餐列表、编辑弹窗已收口到公共 Codex 28 天 / 周额度显示，旧 30 天/日限额不会继续作为公共 Codex 默认文案展示。
- 新增/更新了后端与前端测试，覆盖旧文案输入下的 API 输出、购买确认页、管理端计划列表与编辑弹窗。

## 验证

- `bash backend/migrations/tools/generate-subscription-plan.test.sh`
- `go test ./internal/handler -run "TestPaymentHandlerGetPlansIncludesGroupLimits|TestPaymentHandlerGetCheckoutInfoUsesPublicCodexQuotaSnapshot|TestCalculateSubscriptionRemainingRollingWeekly"`
- `go test ./internal/service -run "TestCreatePlanNormalizesPublicCodexValidity|TestUpdatePlanNormalizesPublicCodexValidity"`
- `go test ./...`
- `pnpm test:run src/views/user/__tests__/PaymentView.spec.ts src/views/admin/orders/__tests__/AdminPaymentPlansView.spec.ts src/views/admin/orders/__tests__/PlanEditDialog.spec.ts`
- `pnpm typecheck`
- `pnpm lint:check`
- `pnpm build`

## 备注

- `pnpm lint:check` 首次与并行 vitest 临时文件冲突，单独重跑后通过。
- 未执行提交、推送或运行态变更。
- 当前工作树仍包含大量历史脏文件，本轮只新增/修改了与周滚动套餐可见缺口相关的部分。
