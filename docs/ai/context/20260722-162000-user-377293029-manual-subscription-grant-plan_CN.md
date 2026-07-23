# 377293029@qq.com 手动补发 29 元套餐计划

## 背景

- 用户邮箱：`377293029@qq.com`，当前 `users.id=117`，状态 active。
- 用户提供 ZPay/易支付订单号：`12344239`。
- 当前公网事实源 `sub2api-dev -> sub2api-postgres-dev` 中未找到该订单、支付审计、订阅、权益周期或流量包记录。
- 使用该用户真实 API Key 公网测试：
  - `/v1/models` 返回 403。
  - `/v1/responses` 返回 403。
  - 错误均为“请先购买套餐或 GPT 流量包”。

## 修复目标

- 手动为 `377293029@qq.com` 补发 29 元套餐。
- 套餐对应 `subscription_plans.id=1 -> group_id=2 -> codex-pool-19-usd`。
- 有效期：从操作时刻开始 28 天。
- 额度：按当前系统定论使用周窗口额度，`weekly_limit_usd=58`，`period_total_quota_usd=232`，`quota_window_unit=week`，`quota_window_days=7`。

## 操作边界

- 先备份 `users`、`api_keys`、`user_subscriptions`、`subscription_entitlement_periods`、`payment_orders`、`payment_audit_logs`、`user_traffic_credits`、`user_allowed_groups` 的相关行。
- 备份写入本机 `backups/`，验证可读和非空。
- 用一个数据库事务插入订阅主表和权益周期事实。
- 权益来源使用 `source_type='manual_zpay'`、`source_id='12344239'`，依赖唯一约束防重复补发。
- 不伪造完整支付订单；当前只补服务权益，并在订阅备注中记录缺失订单号与原因。
- 修改后只删除该用户相关订阅/计费 Redis 缓存，不清全局调度缓存。

## 回滚边界

- 如验证失败且需要回滚：删除本次 `source_type='manual_zpay' AND source_id='12344239'` 对应的权益周期，再软删除或撤销其关联 `user_subscriptions`，并清理该用户订阅缓存。
- 如果用户已经开始产生用量，回滚前必须先评估 `usage_facts/usage_logs` 是否已归属到该权益周期，避免破坏计费事实。
