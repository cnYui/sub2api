# 377293029@qq.com 手动补发 29 元套餐结果

## 结论

- 已为 `377293029@qq.com` 手动补发 29 元套餐。
- 用户现在可以使用公网 API。
- 原因是当前公网事实源中没有该用户的支付订单、订阅、权益周期或流量包记录，API 鉴权层因此返回 403“请先购买套餐或 GPT 流量包”。

## 补发事实

- 用户：`users.id=117 / 377293029@qq.com`
- API Key：`api_keys.id=173`，状态 active，未记录完整 Key。
- 套餐：`subscription_plans.id=1 -> group_id=2 -> codex-pool-19-usd`
- 手动来源：`source_type=manual_zpay / source_id=12344239`
- 新订阅：`user_subscriptions.id=127`
- 新权益周期：`subscription_entitlement_periods.id=211`
- 有效期：`2026-07-22 15:15:05 +08` 到 `2026-08-19 15:15:05 +08`
- 周额度：`58 USD`
- 周期总额度：`232 USD`
- 周窗口：`quota_window_unit=week / quota_window_days=7`
- 审计日志：`payment_audit_logs.id=638`

## 备份

- 备份文件：`backups/20260722-162000-user117-manual-subscription-prechange.sql`
- 大小：约 1.3 MB
- 验证：可读，包含 1633 条相关 INSERT。

## 缓存

- 删除 Redis 订阅缓存：`billing:sub:117:2`，删除结果为 0，表示当时无 Redis 缓存键。
- 发布 L1 失效消息：`subscription:cache:invalidate -> 117:2`，订阅者数量为 1。

## 公网验证

- 补发前：
  - `/v1/models`：403，“请先购买套餐或 GPT 流量包”
  - `/v1/responses`：403，“请先购买套餐或 GPT 流量包”
- 补发后：
  - `/v1/models`：200，返回模型列表。
  - `/v1/responses`：200，`x_client_request_id=e89b98ee-8b02-4983-8bee-05f25edba62c`。
  - `/v1/chat/completions`：200，`x_client_request_id=3a1b1cbe-678e-4fcf-b9fc-cfb5bf442eeb`。

## 计费验证

- `/v1/responses` 验证请求：
  - `usage_facts.id=11759`
  - `billing_status=settled`
  - `entitlement_period_id=211`
  - `usage_logs.id=169375`
  - `subscription_id=127`
  - `actual_cost=0.006375 USD`
- `/v1/chat/completions` 验证请求：
  - `usage_facts.id=11789`
  - `billing_status=settled`
  - `entitlement_period_id=211`
  - `usage_logs.id=169405`
  - `subscription_id=127`
  - `actual_cost=0.00175 USD`
- 最终订阅周用量：`0.139846 USD`。

## 备注

- 本次没有伪造 `payment_orders` 记录，因为当前库确实没有 `12344239`，也没有可直接关联的 ZPay 回调记录。
- 手动补发通过权益来源唯一约束防重复；后续如果网关侧补单入库，需要注意不要重复发放同一笔权益。
