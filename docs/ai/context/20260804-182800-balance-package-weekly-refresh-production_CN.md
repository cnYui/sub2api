# 余额套餐按周刷新与生产核验

## 业务口径

- 余额套餐周期固定为 28 天，每 7 天刷新一次，共 4 个窗口。
- 每个窗口只可使用当周额度；上周未使用的套餐额度在刷新时清理，不累计到下一周。
- 普通充值、邀请返利和 18080 迁移时保留的非套餐余额不属于套餐窗口额度，不得因刷新被清理。
- 用户端在 `/subscriptions` 显示本周剩余额度和下次刷新时间；第 4 次刷新完成后显示不再刷新。

## 根因

旧实现把每次周到账直接追加到 `users.balance`，没有独立记录套餐窗口内尚未使用的额度。因此，套餐用户的钱包余额会累积多周额度，无法区分其中哪些部分应在周刷新时清理。

18080 迁移并没有把 28 天总额度一次性写入 18082 钱包；迁移脚本写入的是当前日额度剩余与当前周套餐额度剩余。问题出在 18082 后续周到账采用追加逻辑，而不是替换逻辑。

## 实现

- `user_balance_packages.remaining_usd` 只记录当前 7 天套餐窗口的未用额度，受 `0 <= remaining_usd <= weekly_credit_usd` 约束。
- 首周到账时，钱包和 `remaining_usd` 同时增加单周额度。
- 周刷新时将 `remaining_usd` 重置为单周额度，钱包只变更 `weekly_credit_usd - old_remaining_usd`，因此上周未用套餐额度不会结转。
- 套餐到期时仅从钱包扣除该套餐的 `remaining_usd`，随后清零并标记为过期。
- 用量结算和周刷新均先锁定用户行，再锁定套餐行；用量扣费同步消耗 `remaining_usd`，避免并发刷新覆盖已用额度。
- `200_balance_package_weekly_remaining.sql` 仅识别旧格式 `BALANCE_PACKAGE_WEEKLY_CREDIT` 审计事件。它按刷新前 7 天实际用量计算可识别的错误结转：`max(weekly_credit_usd - window_usage, 0)`；只回收这部分历史套餐残留，然后基于当前窗口用量重建 `remaining_usd`，并以修正后的钱包余额为上限。

## 生产结果

- 18082 已在 2026-08-04 执行 `199_normalize_balance_package_lifecycle.sql`、`200_balance_package_weekly_remaining.sql` 和 `201_normalize_balance_package_plan_snapshots.sql`。
- 当前有效余额套餐为 62 条；`remaining_usd` 越界为 0 条，`remaining_usd` 大于对应钱包余额为 0 条。
- ¥49 套餐的单周额度为 `$128`；当前 8 个有效 ¥49 套餐中，最大本周剩余额度为 `$119.51873000`，没有向用户展示 `$512` 的四周总额度。
- 旧追加逻辑可识别的历史残留已按审计窗口修正。示例套餐的扣回额为：套餐 11 `$15.5124084375`、套餐 12 `$340.6536250625`、套餐 18 `$68.6660593875`、套餐 38 `$76`；已用完窗口不扣回。
- 个别用户的钱包余额仍可能高于单周套餐额度，但套餐字段 `remaining_usd` 已受单周上限约束；这部分差额不会作为套餐余额展示或在周刷新时被批量扣除。
- 最新 18082 应用容器健康检查和本地 `/health` 均返回成功；访问日志确认 `/subscriptions` 会请求 `/api/v1/payment/balance-packages`。

## 验证

- `go test ./internal/service -run 'Test(CreditDueBalances|ListUserPackages|ValidateUserPurchase|CreditInitialBalance)' -count=1 -v` 通过。
- `go test ./internal/repository -count=1` 通过。
- `go test ./cmd/server -run '^$' -count=1` 通过。
- `pnpm vitest run src/views/user/__tests__/SubscriptionsView.spec.ts` 通过。
- `pnpm typecheck` 通过。
- 全量 Vitest 有 3 个既有失败，位于 `HomeView.compact.spec.ts` 与 `admin.system.rollback.spec.ts`，与余额套餐改动无关；新增订阅页用例通过。
