# tongji_lishouqi@163.com 手动补发 59 元套餐与分配失败排查计划

## 背景

- 用户邮箱：`tongji_lishouqi@163.com`。
- 当前公网 health 正常：`https://api.aaccx.pw/health` 返回 `{"status":"ok"}`。
- 当前本机运行态与历史记忆不同：`sub2api-candidate:18084` 不存在，当前公网链路为 `Cloudflare Tunnel -> sub2api-public-nginx-local:8080 -> sub2api-dev:18080 -> sub2api-postgres-dev/sub2api-redis-dev`。
- 用户 `users.id=27`，状态 active，存在两个 active API Key；不记录完整 Key。
- 59 元套餐：`subscription_plans.id=3 -> group_id=4 -> codex-pool-49-usd`，当前为 28 天周窗口套餐，`weekly_limit_usd=118`。
- 用户已有旧订阅 `user_subscriptions.id=47`，同分组 `group_id=4`，状态 expired，有效期 `2026-06-19 10:42:01 +08` 至 `2026-07-19 10:42:01 +08`，但没有 `subscription_entitlement_periods` 记录。

## 目标

1. 先手动为该用户补发当前 59 元套餐，让真实 API Key 可以使用公网 `/v1/*`。
2. 用用户真实 Key 验证 `/v1/models`、`/v1/responses`、`/v1/chat/completions`，并核对 `usage_facts` 与 `usage_logs` 是否落到本次权益周期。
3. 再排查后台“分配订阅失败”的根因，重点核对旧日额度订阅记录与新周额度 entitlement period 路径的交互。

## 手动补发方案

- 备份相关行：`users`、`api_keys`、`user_subscriptions`、`subscription_entitlement_periods`、`payment_orders`、`payment_audit_logs`、`user_allowed_groups`。
- 备份写入 `backups/`，验证文件可读、非空。
- 不伪造真实支付订单；本次只补服务权益事实。
- 复用旧订阅 `user_subscriptions.id=47`，把它从 expired 更新为 active：
  - `starts_at = now()`
  - `expires_at = now() + interval '28 days'`
  - `status = 'active'`
  - `daily_window_start / weekly_window_start / monthly_window_start / weekly_anchor_at = now()`
  - `daily_usage_usd / weekly_usage_usd / monthly_usage_usd = 0`
  - 追加 notes，说明是 `manual_admin_assignment` 补发 59 元周额度权益。
- 插入 `subscription_entitlement_periods`：
  - `user_id=27`
  - `subscription_id=47`
  - `group_id=4`
  - `source_type='manual_admin_assignment'`
  - `source_id='tongji_lishouqi_59_20260722'`
  - `daily_limit_usd=NULL`
  - `weekly_limit_usd=118`
  - `period_total_quota_usd=472`
  - `quota_window_unit='week'`
  - `quota_window_days=7`
  - `period_days=28`
  - `status='active'`
- 清理该用户订阅缓存：`billing:sub:27:4`，发布 `subscription:cache:invalidate -> 27:4`；不清全局调度缓存。

## 回滚边界

- 如补发后 API 仍不可用且需要回滚：先确认是否已有 usage fact 归属到本次 `subscription_entitlement_periods`。
- 若尚无用量归属：删除本次 `source_type/source_id` 的权益周期，把 `user_subscriptions.id=47` 恢复到备份中的 expired 状态，并清理该用户订阅缓存。
- 若已有用量归属：不直接删除权益周期，先保留事实并另做补偿方案，避免破坏计费账本。

## 初步根因假设

- 后台单人分配路径调用 `AssignSubscription`，当同用户同分组已有未删除旧订阅时，`assignSubscriptionWithReuse` 会复用旧记录，不走续期；旧 30 天订阅在迁移到 28 天周窗口后还可能触发 `validity_days_mismatch`。
- 即使没有触发冲突，复用路径也会跳过 `subscription_entitlement_periods` 创建，导致新周额度套餐没有权益周期事实。
- 需要用测试复现后再修复，不能只靠手动补库。
