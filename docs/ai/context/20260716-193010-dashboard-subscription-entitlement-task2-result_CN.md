# Dashboard 套餐权益周期 Task 2 实施结果

## 背景

本轮继续 `codex/dashboard-subscription-quota-realtime` 分支的 Dashboard 套餐额度实时展示实现。Task 1 已提交权益周期 schema；本轮完成 Task 2：把订阅发放和撤销收敛到带来源幂等的事务原语，并补齐 review 指出的事务与缓存边界。

## 已完成

- 新增 `SubscriptionEntitlementPeriodRepository`，通过 Ent 读写 `subscription_entitlement_periods`，并将 source 唯一冲突映射为领域错误。
- 新增 `GrantSubscriptionEntitlement`：
  - 先按 `source_type/source_id` 快速重放，未命中才锁用户行。
  - 锁后再次按 source 复查，避免等待锁期间重复续期。
  - 在同一事务内读取套餐、创建或续期订阅、写入不可变权益周期。
  - 发放时快照 `groups.daily_limit_usd`，同套餐提前续费从旧 `expires_at` 连续开始。
- `AssignSubscriptionInput` 增加 `EntitlementSource`；无 source 的调用保持旧逻辑，不创建权益周期。
- `withSubscriptionUpdateTx` 复用外层 Ent transaction，避免嵌套事务。
- `RevokeSubscription` 在同一事务内撤销未过期权益周期，再标记并软删订阅。
- `groupRepository.GetByIDLite()` 改为 `clientFromContext(ctx, r.client)`，确保外层事务未提交的套餐额度变更可被权益周期快照读取。
- 订阅缓存失效改为提交后执行：
  - 无外层事务时在自有事务提交后立即失效 L1/Redis。
  - 有外层事务时注册 `Tx.OnCommit`，只在外层 commit 成功后失效。
  - 外层 rollback 时只清理待执行回调，不失效缓存。
  - 同一事务内相同 user/group 的失效请求会去重。
- Wire 注入新增 `ProvideSubscriptionService`，让生产路径获得 `SubscriptionEntitlementPeriodRepository`，同时不破坏既有 `NewSubscriptionService(...)` 测试调用。

## Review 修复点

- P1：同 source 重放不应先拿用户锁。已通过 `TestGrantSubscriptionEntitlement_ReplaysExistingSourceBeforeTakingUserLock` 覆盖。
- P1：套餐额度快照必须来自当前 Ent transaction。已通过 `TestSubscriptionService_GrantSnapshotsDailyLimitFromOuterTransaction` 覆盖。
- P2：外层事务 commit 后必须失效订阅缓存，rollback 不失效。已通过 `TestRevokeSubscription_DefersCacheInvalidationToOuterTransactionOwner`、`TestRevokeSubscription_InvalidatesCacheAfterOwnedTransactionCommits`、`TestRevokeSubscription_RegistersOneCacheInvalidationAfterOuterTransactionCommits` 覆盖。
- P2：period 创建失败时，订阅新增或续期必须一起回滚。已通过 `TestSubscriptionService_GrantRollsBackSubscriptionWhenPeriodCreateFails` 和 `TestSubscriptionService_GrantRollsBackNewSubscriptionWhenPeriodCreateFails` 覆盖。

## 验证

在 `backend/` 下已通过：

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run 'TestSubscriptionEntitlementPeriodRepository|TestSubscriptionService_Grant'
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
git diff --check
```

## 未做

- 尚未把支付、兑换码、后台发放和默认发放入口切到稳定 `EntitlementSource`；这是 Task 3 范围。
- 尚未实现 Dashboard quota read model、handler 和前端展示；这是后续 Task 范围。
- 未改生产数据库、Redis、容器、Nginx 或运行态。
