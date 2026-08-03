# 邀请返利自动入账与冻结扣费

## 需求

- 开启项目邀请返利。
- 全局返利比例固定默认 8%。
- 新返利冻结 24 小时，但余额立即可用于模型扣费。

## 实现决策

- 返利履约仍在 `user_affiliate_ledger` 台账和订单履约事务中完成，避免支付成功与返利入账分离。
- `AffiliateRepository.AccrueQuota` 在同一事务中更新邀请人 `users.balance`、`users.total_recharged` 和 `user_affiliates` 的返利累计/冻结额度；冻结时间写入 `frozen_until`。
- 模型余额扣费在 `usage_billing_repo.go` 中使用数据修改 CTE：先扣 `users.balance`，再按“普通余额优先、冻结返利补足”的差额减少 `aff_frozen_quota`。因此冻结期不会阻断模型消费，也不会让冻结台账超过实际余额。
- `ThawFrozenQuota` 只释放冻结标记并减少冻结快照，不再把额度转入余额，避免自动入账后重复加款。
- 保留旧转入接口以兼容历史客户端；新前端不再展示手动转入操作。
- 迁移 197 将旧版尚未转入的返利一次性计入余额，并清空旧 `aff_quota`；`aff_frozen_quota` 保留用于冻结状态追踪，通过 settings 标记防止重复执行。迁移同时将当前实例配置为 `affiliate_enabled=true`、`affiliate_rebate_rate=8`、`affiliate_rebate_freeze_hours=24`。

## 缓存与一致性

- standalone 充值路径在返利事务成功后刷新邀请人余额缓存。
- 支付履约在外层返利事务提交后再次按被邀请人查找邀请人并刷新缓存，避免事务提交前刷新导致缓存重新写入旧余额。

## 验证

- `pnpm typecheck` 通过。
- `pnpm exec vitest run src/views/user/__tests__/AffiliateView.spec.ts` 通过。
- `go test ./internal/repository -run '^TestDeductUsageBillingBalance|^TestApplyUsageBillingEffects_FlagsBalanceOverdraft' -count=1` 通过。
- `go test -tags unit ./internal/repository -run '^TestDeductUsageBillingBalance|^TestApplyUsageBillingEffects_FlagsBalanceOverdraft' -count=1` 通过。
- `go test -tags unit ./internal/service -run '^TestExecuteSubscriptionFulfillmentAppliesAffiliateRebate$' -count=1` 通过。
- 前端全量测试存在与本次改动无关的既有失败：`HomeView.compact`、`admin.system.rollback`；邀请返利页面测试通过。
- 集成测试使用 `-tags integration`，依赖本地 PostgreSQL；若环境未提供数据库，保留为部署前验证项。
