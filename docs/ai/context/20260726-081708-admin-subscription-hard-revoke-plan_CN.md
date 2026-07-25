# 管理员订阅硬撤销实施计划

## 范围

- 将管理端订阅撤销改为硬删除，并物理清理历史订阅 `id=53`。
- 保留订单、用量、计费记录、用户、API Key 与流量卡。

## 实施步骤

1. 在 `backend/internal/service/subscription_revoke_test.go` 将现有软删除断言改为失败测试：断言 `RevokeSubscription` 只执行读取与 `HardDelete`，不调用 `UpdateStatus`。
2. 在 `backend/internal/service/user_subscription_port.go` 为 `UserSubscriptionRepository` 增加 `HardDelete(ctx, id)`；测试 stub 记录该调用。
3. 在 `backend/internal/repository/user_subscription_repo.go` 实现 `HardDelete`：使用事务 client 删除 `subscription_quota_debt_adjustments`、`subscription_entitlement_periods`，再以 `mixins.SkipSoftDelete(ctx)` 删除 `user_subscriptions`。
4. 在 `backend/internal/repository/soft_delete_ent_integration_test.go` 增加集成测试，创建订阅、权益段和额度债务调整后调用 `HardDelete`，断言三类记录均不可见。
5. 在 `backend/internal/service/subscription_service.go` 删除撤销路径中的状态更新和权益段撤销，调用 `HardDelete` 后沿用提交后缓存失效。
6. 更新 `frontend/src/i18n/locales/zh.ts` 与 `frontend/src/i18n/locales/en.ts` 的撤销确认和说明文案，明确会永久删除订阅记录。
7. 运行服务层与仓储集成测试，以及前端类型检查和 lint。
8. 对 `sub2api-postgres-dev/sub2api` 创建变更前 PostgreSQL 备份并验证备份目录可读；在单一事务中删除 `id=53` 的额度债务调整、权益段和订阅主记录；删除订阅缓存并发布跨进程失效消息。
9. 核对 `id=53`、其权益段 `142/209` 和关联额度债务调整不存在；核对用户、Key、订单和用量行仍存在；记录结果到新的上下文文档。

## 回滚

- 代码回滚到本分支修改前即可恢复后续撤销的软删除语义。
- 运行态 `id=53` 只能通过变更前数据库备份恢复。
