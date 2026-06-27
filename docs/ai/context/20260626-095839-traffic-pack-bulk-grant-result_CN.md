# 10 USD GPT 流量卡批量发放结果

## 执行时间

- 执行时间：2026-06-26 09:57 JST 左右。
- 生产库：Docker 容器 `sub2api-postgres` 中的 `sub2api` 数据库。
- 公网服务容器：`sub2api`，端口映射 `127.0.0.1:18080 -> 8080`。

## 发放批次

- 批次号：`manual_grant_gpt_10usd_20260626`
- 订单号前缀：`sub2_gift_gpt10_20260626_u`
- 目标卡：`traffic_packs.code = 'gpt_traffic_10usd_3cny'`
- 卡包 ID：`2`
- 每用户额度：10 USD
- 有效期：365 天
- 到期时间：`2027-06-26 08:57:24.31087+08`

## 发放范围

发放给：

- `users.deleted_at IS NULL`
- `users.status = 'active'`

实际有效用户数：44。

未发放给软删除或非 active 用户。

## 写入结果

事务写入结果：

- `payment_orders`：44 条
- `user_traffic_credits`：44 条
- `traffic_credit_ledger` purchase 流水：44 条
- `payment_audit_logs`：44 条

订单设置：

- `order_type = 'traffic_pack'`
- `payment_type = 'manual_grant'`
- `status = 'COMPLETED'`
- `amount = 0.00`
- `pay_amount = 0.00`

原因：这是赠送，不应计入真实支付收入。

## 验证结果

聚合验证：

- 发放额度记录：44 条
- `initial_usd` 合计：440 USD
- `remaining_usd` 合计：440 USD
- purchase 流水：44 条
- purchase 流水金额合计：440 USD
- 手工订单：44 条
- 手工订单 `amount` 合计：0
- 手工订单 `pay_amount` 合计：0
- 审计日志：44 条

覆盖验证：

- 有效用户缺失发放：0
- 发放额度不等于 10 USD 的有效用户：0
- 软删除或非 active 用户误发：0
- 卡包绑定全部为 `gpt_traffic_10usd_3cny`。

公网接口验证：

- 使用公网容器 `127.0.0.1:18080` 登录管理员后查询 `/api/v1/payment/checkout-info`。
- 返回 `traffic_credit_summary.total_remaining_usd = 10`。
- 返回 `traffic_credit_summary.next_expiring_usd = 10`。
- 返回 `traffic_credit_summary.next_expires_at = 2027-06-26T08:57:24.31087+08:00`。
- 返回 `traffic_packs` 仍为 3 个。

## 用户可见效果

用户可在 `/purchase` 页面，即“充值/订阅”页面看到：

- `GPT 流量包`
- `当前可用 10.00 刀`
- 最近到期日

用户订单页 `/orders` 会出现本次手工发放订单，但不显示流量卡剩余额度；剩余额度以 `/purchase` 页面汇总为准。

## 后续注意

- 当前流量卡没有显式 `used` 状态，耗尽后表现为 `remaining_usd = 0`，前端汇总不再统计该卡。
- 流量卡只对 OpenAI 平台扣费生效。
- 剩余额度不会按天刷新；每天继续沿用 `remaining_usd`，直到用完或过期。
