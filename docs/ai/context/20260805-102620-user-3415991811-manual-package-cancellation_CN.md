# 用户 3415991811@qq.com 的 29 元余额套餐人工取消

## 核对结果

- 生产实例：`sub2api-official-18082`。
- 用户 ID：`523`，操作前普通余额为 `0.45314656 USD`。
- 当前有效余额套餐：套餐记录 `29`，计划 `balance-29`（余额套餐 ¥29），订单 `149`，状态为 `completed`，本周剩余额度为 `0.45314656 USD`。
- 订单 `149` 已有退款请求，退款金额为 `2.31`，当前状态为 `REFUND_FAILED`；失败原因为支付网关代理连接被拒绝。
- 用户没有 `user_subscriptions` 模型订阅；独立流量卡额度不属于本次取消范围。

## 决策

- 仅取消订单 `149` 对应的余额套餐权益：套餐状态改为 `cancelled`，`remaining_usd` 清零，`next_credit_at` 清空，阻止后续到账并从订阅页隐藏。
- 不修改 `users.balance`，因为用户只要求取消订阅，已到账的普通余额不跨订单追溯扣除。
- 不修改退款订单状态，不伪造网关退款成功；保留 `REFUND_FAILED` 和原退款审计，后续仍可按原订单重试退款。
- 不修改 `user_traffic_credits` 或 `traffic_credit_ledger`。
- 写入 `BALANCE_PACKAGE_MANUAL_CANCELLATION` 审计，记录操作前后套餐状态和退款未执行事实。

## 执行与核验

- 在 PostgreSQL 事务中锁定并校验用户 `523`、套餐 `29` 和订单 `149` 的预期状态后完成更新。
- 套餐 `29` 已从 `completed` 改为 `cancelled`，`remaining_usd` 从 `0.45314656` 置零，`next_credit_at` 保持为空；当前可展示余额套餐数为 `0`。
- 普通余额仍为 `0.45314656 USD`，未被取消操作扣除。
- 订单 `149` 仍为 `REFUND_FAILED`，退款金额仍为 `2.31`，未伪造退款成功。
- 流量卡记录 `170`、`297` 未修改，剩余额度仍分别为 `0` 和 `3.5835821875 USD`。
- 已写入 `payment_audit_logs`：`id=1212`，操作为 `BALANCE_PACKAGE_MANUAL_CANCELLATION`，操作人为 `admin`。
- 已检查 Redis `billing:balance:523`，键不存在（`EXISTS=0`）。
