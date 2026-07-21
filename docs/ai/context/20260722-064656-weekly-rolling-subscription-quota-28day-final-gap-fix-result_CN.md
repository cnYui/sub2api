# 2026-07-22 周滚动订阅额度与 28 天周期最终缺口修复结果

## 范围

- 承接 `20260721-222923-weekly-rolling-subscription-quota-28day-design_CN.md` 与前序实现，继续在当前工作树补齐本地可验证缺口。
- 仅修改本地代码与上下文文档；未操作公网、生产数据库、Nginx、Cloudflare、CLIProxyAPI 或 Docker 运行态。
- 未执行 `backend/migrations/tools/weekly-quota-cutover.sh --apply`。
- 未 stage、未 commit、未 push。

## 本轮补齐项

1. 余额订阅退款事务内重算结果回传与审计字段补齐
   - 管理端余额订阅退款执行时，事务内重新计算 quote 后返回真实 `refund_amount`。
   - 持久化 `refund_balance_amount` 与 `refund_balance_status`，避免余额退款链路只有余额变化、缺少订单审计事实。
   - 测试覆盖 quote 从 29 元变为 21.75 元的并发用量场景。

2. 周窗口锚点一致性
   - `CalculateSubscriptionWeeklyWindow` 不再信任偏移的 `weekly_window_start`。
   - 只有当持久化窗口起点严格等于 `weekly_anchor_at + 7n 天` 且不晚于当前时间时，才允许作为快进起点。
   - 首次激活 rolling weekly 订阅时写入锚点窗口，而不是自然日零点，避免 DB 窗口事实变脏。

3. rolling weekly 前端完整周额度 fallback 收口
   - 顶部迷你进度、用户订阅页、管理端订阅页已避免在后端有效窗口事实缺失时回退到完整周额度。
   - 本轮继续修复 Key 用量页：`effective_weekly_limit_usd` 缺失且窗口未激活时，不再显示 `$0 / $72`，只显示“当前周额度窗口未激活”。
   - 保留非公共旧周额度订阅的 fallback，避免误伤非公共订阅原有展示。

4. 退款 quote 展示顺序
   - 用户订单、管理端退款弹窗、管理端订单列表详情、订单详情组件统一改为“已用 / 28 天总额度”。
   - 对应中英文文案从“28 天总额度 / 已用”调整为“已用 / 28 天总额度”。

5. 管理端订阅分配默认天数
   - 复用 `defaultValidityDaysForGroup` 重置分配表单。
   - 公共 Codex group 仍由选择 group 时自动变为 28 天；无 group 与非公共订阅默认保持 30 天。

## 验证

已通过：

```powershell
cd D:\CodeWorkSpace\sub2api\backend
go test -tags unit ./internal/service -run "TestAdminBalanceSubscriptionRefundCreditsBalanceAndRevokesSubscription|TestExecuteRefundRecalculatesAdminSubscriptionQuoteInsideTransaction|TestPrepareRefundUsesSubscriptionQuoteAndPersistsBasis"
go test ./internal/service -run "TestCalculateSubscriptionWeeklyWindow|TestUserSubscriptionWeeklyWindowUsesPersistedAnchor|TestCheckWeeklyLimit_RollingWeeklyIgnoresStaleWindowUsage|TestCurrentRollingWeeklyWindowUsesEntitlementExpiry|TestValidateAndCheckLimits_RollingWeeklyLimitExceededIncludesResetMetadata|TestCheckAndActivateWindow_RollingWeeklyUsesAnchoredWindowStart"
go test ./...
go test -tags unit ./internal/service
```

```powershell
cd D:\CodeWorkSpace\sub2api\frontend
pnpm exec eslint src/views/KeyUsageView.vue src/views/user/UserOrdersView.vue src/components/admin/payment/AdminRefundDialog.vue src/views/admin/orders/AdminOrdersView.vue src/components/admin/payment/AdminOrderDetail.vue src/views/admin/SubscriptionsView.vue
pnpm test:run src/views/__tests__/KeyUsageView.spec.ts src/components/common/__tests__/SubscriptionProgressMini.spec.ts src/views/user/__tests__/SubscriptionsView.spec.ts src/views/user/__tests__/UserOrdersView.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts src/views/admin/orders/__tests__/PlanEditDialog.spec.ts
pnpm typecheck
pnpm lint:check
pnpm test:run
pnpm build
```

```powershell
cd D:\CodeWorkSpace\sub2api
bash -n backend/migrations/tools/weekly-quota-cutover.sh
git diff --check
git status --short
git ls-files --others --exclude-standard docs/ai/context
```

说明：

- `git diff --check` 通过；仅出现 `AGENTS.md` 与 `backend/go.sum` 的 LF/CRLF 工作区换行提示。
- 前端测试输出包含既有测试用例故意触发的错误日志、Vue stub warning、Browserslist 过期提示。
- 前端构建输出包含既有 Vite 动态/静态混合导入与 chunk size warning。

## 未提交文件状态

- 当前工作树仍包含前序周额度实现、迁移、Ent 生成文件、前端页面与测试的大量修改。
- `docs/ai/context/` 下有多份未跟踪历史上下文文档，本轮按规则新增本文档，没有覆盖、重命名或删除旧文档。
- 未提交、未推送，后续如进入 PR/提交流程，需要先按项目规则复核未跟踪上下文文档是否可纳入提交。

## 明确未做

- 未对本地数据库执行 `weekly-quota-cutover.sh --apply`。
- 未运行生产备份或恢复。
- 未触碰公网候选环境、生产数据库、Redis、Nginx、Cloudflare、CLIProxyAPI 或账号池配置。
