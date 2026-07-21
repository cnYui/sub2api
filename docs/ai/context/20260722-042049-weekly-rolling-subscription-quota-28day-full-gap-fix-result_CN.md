# 2026-07-22 周滚动订阅额度与 28 天周期收口实施结果

## 范围

本轮继续按 `docs/ai/context/20260721-222923-weekly-rolling-subscription-quota-28day-design_CN.md` 收口公共 Codex 订阅“28 天有效期 + 每 7 天按订阅锚点刷新额度”的本地实现缺口。

未操作公网、生产数据库、Nginx、Cloudflare、CLIProxyAPI，未执行 cutover apply，未提交、未推送。

## 本轮补齐项

1. 后端周额度耗尽错误补齐精确重置时间。
   - `backend/internal/service/subscription_service.go`
   - 公共 Codex rolling weekly 超额时返回 `ErrWeeklyLimitExceeded`，metadata 携带 `window_resets_at=<精确窗口 ResetsAt RFC3339>`。
   - 新增回归测试 `TestValidateAndCheckLimits_RollingWeeklyLimitExceededIncludesResetMetadata`。

2. 退款只撤销目标权益段，保留未来续费权益。
   - `backend/internal/service/payment_refund_state.go`
   - `revokeRefundSubscriptionInTransaction` 改为只撤销目标订单对应的 active entitlement period。
   - 若仍存在后续 active rolling weekly entitlement period，`user_subscriptions.status` 保持 `active`，`expires_at` 更新为剩余权益最大过期时间。
   - 当前空档期由“没有当前生效 entitlement period”触发 `ErrSubscriptionInvalid`，后续权益仍按原计划生效。
   - 新增/强化测试 `TestGatewaySubscriptionRefundRevokesOnlyTargetEntitlementPeriod`。

3. 前端 rolling weekly 窗口禁止自行推导重置时间。
   - `frontend/src/utils/subscriptionQuota.ts`
   - `frontend/src/views/admin/SubscriptionsView.vue`
   - 新增 `isPublicCodexSubscriptionGroupName`、`isRollingWeeklySubscription`。
   - 管理端订阅页对 rolling weekly 缺少后端 `weekly_window_resets_at` 时显示窗口未激活，不再用 `weekly_window_start + 7 天` 猜测。

4. 前端空档期提示补齐。
   - `frontend/src/views/user/SubscriptionsView.vue`
   - `frontend/src/components/common/SubscriptionProgressMini.vue`
   - `frontend/src/views/KeyUsageView.vue`
   - rolling weekly 当前无后端窗口事实时，用户订阅页、顶部进度、Key 用量页展示“当前周额度窗口尚未激活/未激活”，避免把未来权益误展示为当前可用。

5. 购买确认区继续锁定周额度与 28 天文案。
   - `frontend/src/views/user/__tests__/PaymentView.spec.ts`
   - 29 元套餐确认区断言展示 `weeklyLimit $72`、`periodTotalQuota $288`、`weeklyRefresh`、`28 days`。
   - 同时断言不展示 `dailyLimit`、`dailyRefresh`、`24点`、`30天`。

6. 增加前端回归测试。
   - `frontend/src/utils/__tests__/subscriptionQuota.spec.ts`
   - `frontend/src/views/user/__tests__/SubscriptionsView.spec.ts`
   - `frontend/src/components/common/__tests__/SubscriptionProgressMini.spec.ts`
   - `frontend/src/views/__tests__/KeyUsageView.spec.ts`

## 验证结果

已通过以下验证：

```bash
cd backend && go test -tags unit ./internal/service -run "TestValidateAndCheckLimits_RollingWeeklyLimitExceededIncludesResetMetadata|TestGatewaySubscriptionRefundRevokesOnlyTargetEntitlementPeriod"
cd frontend && pnpm test:run src/utils/__tests__/subscriptionQuota.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/SubscriptionsView.spec.ts src/components/common/__tests__/SubscriptionProgressMini.spec.ts src/views/__tests__/KeyUsageView.spec.ts
cd backend && go test ./...
cd frontend && pnpm typecheck
cd frontend && pnpm lint:check
cd frontend && pnpm test:run
cd frontend && pnpm build
git diff --check
```

`git diff --check` 仅提示 `AGENTS.md` 与 `backend/go.sum` 后续 Git 触碰时会发生 LF/CRLF 转换，没有空白错误。

前端测试和构建存在既有 warning/stderr：Browserslist 数据过期、Vue 测试未 stub `router-link`/`el-tooltip`、预期错误日志、chunk size 与 dynamic import warning；命令退出码均为 0。

## dry-run 与 cutover 状态

本地 cutover 仍只允许 dry-run，不执行 `weekly-quota-cutover.sh --apply`。

既有审计结果：

- 公共 Codex active 订阅：63 条。
- 阻塞对象：51 个。
- 分类：
  - `completed_without_entitlement`: 5
  - `overlapping_entitlement`: 43
  - `refund_in_progress_order`: 5
  - `usage_fact_unallocated`: 3

设计要求异常对象禁止自动迁移或自动退款，因此本轮保持代码与测试收口，不自动修改这些历史对象。

## 未提交文件状态

当前工作树仍是大范围脏状态，包含前序日额度顺延、图片/用量修复、周滚动订阅、迁移工具、前端文案与测试等多轮改动。

`docs/ai/context/` 下仍有多份未跟踪上下文文档；这些文档需要在最终提交前统一复核是否含敏感信息，再随功能提交或单独 `docs: archive ai context` 提交。

## 后续门禁

1. 先人工处理或隔离 dry-run 阻塞对象。
2. 只在 dry-run 清洁后执行本地 `weekly-quota-cutover.sh --apply`。
3. apply 后复核订阅、权益段、债务审计、usage facts、Redis/L1 缓存一致性。
4. 生产切换前必须重新备份 PostgreSQL 与 Redis，并重新按生产事实 dry-run，不能复用本地结论。
