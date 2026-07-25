# 管理端订阅硬撤销结果

## 目标

将管理端 `admin/subscriptions` 的“撤销订阅”改为物理删除，避免软删除订阅仍占用用户与分组的唯一订阅关系；同时按确认范围物理清理历史软删除订阅 `user_subscriptions.id=53`。

## 代码改动

- `UserSubscriptionRepository` 新增 `HardDelete`：先删除 `subscription_quota_debt_adjustments`、`subscription_entitlement_periods`，再借助 `mixins.SkipSoftDelete` 物理删除 `user_subscriptions`。
- `SubscriptionService.RevokeSubscription` 改为在事务内调用 `HardDelete`，事务提交后继续失效 L1、Redis 和跨实例 Pub/Sub 订阅缓存。
- 管理端确认文案明确为永久删除订阅，订单与用量记录保留。
- 新增仓储集成测试，覆盖依赖记录与订阅主记录均被物理删除；服务测试覆盖不再更新状态或走软删除。

## 运行态清理

- 修改前已创建并校验 PostgreSQL custom-format 备份：`D:\CodeWorkSpace\sub2api\backups\20260726-083020-before-admin-subscription-53-hard-delete.dump`。该备份可用 `pg_restore` 恢复整库；本次回滚边界是恢复该备份并重新使订阅缓存失效。
- 在单一事务中删除订阅 `id=53` 的 1 条额度债务调整、2 条权益周期和订阅主记录。
- 删除 Redis 键 `billing:sub:35:2`，并向 `subscription:cache:invalidate` 发布 `35:2`。
- 删除后核对：订阅、权益周期和债务调整计数均为 0；用户 `id=35` 仍存在且状态为 `disabled`，其 1 个 API Key、6 笔订单和 4 张流量卡均保留。
- `usage_logs` 的 `subscription_id` 外键为 `ON DELETE SET NULL`：原关联订阅 `id=53` 的 3386 条用量历史未删除，订阅关联已自动清空。

## 当天其余软删除

2026-07-22 其余软删除记录按确认范围未处理：

- 订阅：`user_subscriptions.id=93`（用户 `83`、分组 `2`）。
- API Key：`api_keys.id=80`（用户 `45`）、`110`（用户 `47`）、`133`（用户 `45`）、`101`（用户 `67`）。
- 其他带 `deleted_at` 的表当天没有软删除记录。

## 验证

- `go test ./...` 通过。
- `pnpm typecheck`、`pnpm lint:check`、`pnpm test:run`、`pnpm build` 通过。
- `git diff --check` 通过。
- `go test -tags integration ./internal/repository` 受既有迁移 `178_seed_codex_249_299_subscription_plans.sql` 阻塞：目标数据库缺少 `groups.image_rate_independent` 列；该问题在本次改动前已存在，未修改无关 schema。
