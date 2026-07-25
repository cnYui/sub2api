# 管理员订阅硬撤销设计

## 目标

- 将管理端订阅页的 `DELETE /api/v1/admin/subscriptions/:id` 从“撤销后软删除”改为物理删除。
- 清理历史软删除订阅 `user_subscriptions.id=53`，不处理 2026-07-22 发现的其他软删除记录。

## 现状与根因

- `SubscriptionHandler.Revoke` 调用 `SubscriptionService.RevokeSubscription`。
- 原实现先把订阅状态改为 `expired`，撤销未过期权益段，再调用带 `SoftDeleteMixin` 的 `UserSubscriptionRepository.Delete`，最终只写入 `deleted_at`。
- `user_subscriptions.id=53` 有 2 条权益段、1 条额度债务调整和 3386 条用量记录引用。订阅主记录不能直接物理删除：权益段与额度债务调整的外键为 `RESTRICT`。

## 设计决策

- 保持现有管理员路由与前端调用不变；“撤销”语义改为不可恢复的物理删除。
- 在 `UserSubscriptionRepository.HardDelete` 内部使用同一事务 client，按外键依赖顺序删除：
  1. `subscription_quota_debt_adjustments`
  2. `subscription_entitlement_periods`
  3. `user_subscriptions`，通过 `mixins.SkipSoftDelete` 绕过软删除 Hook
- `payment_orders`、`usage_logs` 与 `billing_usage_entries` 保留，现有外键 `ON DELETE SET NULL` 自动清空它们的 `subscription_id`。
- 不删除用户、API Key、流量卡、订单、用量、计费事实或当天其他软删除对象。
- 硬删除成功后沿用现有订阅缓存失效逻辑；前端文案明确为永久删除订阅记录。

## 验证

- 服务单元测试验证撤销不会再写 `expired` 或撤销权益段，而是调用硬删除仓储方法并失效缓存。
- 仓储集成测试验证物理删除会先清理权益段和额度债务调整，主记录在 `SkipSoftDelete` 查询下也不可见。
- 运行态 `id=53` 操作前创建 PostgreSQL 可恢复备份，操作后核对主记录和两个子表均不存在，订单/用量仍存在且关联字段已置空。

## 回滚边界

- 代码可通过回退分支提交恢复软删除行为。
- `id=53` 的运行态数据只能从变更前数据库备份恢复；硬删除后没有行级自动恢复路径。
