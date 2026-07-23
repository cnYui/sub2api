# 当前所有用户 10 USD 流量卡批量发放计划

## 背景

- 用户要求：给当前所有用户发一个“10 元的流量卡”。
- 当前系统没有售价 10 元的流量卡商品；已有 10 USD GPT/OpenAI 流量卡商品：
  - `traffic_packs.id=2`
  - `code=gpt_traffic_10usd_3cny`
  - `name=GPT 流量包 10 刀`
  - `price=3.00`
  - `credit_usd=10`
  - `validity_days=365`
  - `platform=openai`
- 历史批量赠送也使用同一张 10 USD 流量卡，订单金额写 0，不计入真实收入。

## 发放范围

- 目标用户口径：`users.deleted_at IS NULL AND users.status='active'`。
- 当前目标数量：119 个。
- 包含管理员账号，原因是历史批量发放同样覆盖 active 管理员，且本次需求是“所有用户”。
- 不发给软删除或非 active 用户。

## 批次与幂等

- 批次号：`grant-20260722-10usd-current-users`
- 每用户订单号：`grant-20260722-10usd-current-users-u<user_id>`
- 使用 `payment_orders.out_trade_no` 的唯一索引防重复。
- 若重复执行，同一用户已有该批次订单则跳过。

## 写入事实

- `payment_orders`：
  - `order_type='traffic_pack'`
  - `payment_type='manual_grant'`
  - `status='COMPLETED'`
  - `amount=0`
  - `pay_amount=0`
  - `provider_snapshot` 记录批次号、赠送原因和流量卡快照
- `user_traffic_credits`：
  - `pack_id=2`
  - `platform='openai'`
  - `initial_usd=10`
  - `remaining_usd=10`
  - `reserved_usd=0`
  - `expires_at=credited_at + 365 days`
- `traffic_credit_ledger`：
  - `entry_type='purchase'`
  - `amount_usd=10`
  - `balance_after_usd=10`
- `payment_audit_logs`：
  - `action='TRAFFIC_PACK_MANUAL_GRANT'`
  - 记录批次号、卡包和赠送原因

## 安全边界

- 先备份 `users`、`traffic_packs`、`payment_orders`、`user_traffic_credits`、`traffic_credit_ledger`、`payment_audit_logs`。
- 不修改用户余额、订阅、API Key、订单真实支付通道、Redis、Nginx 或容器。
- 不记录任何 API Key、支付密钥或内部 token。

## 回滚边界

- 若发放后未被消费，可按批次订单号删除本批次 `traffic_credit_ledger`、`user_traffic_credits`、`payment_audit_logs`、`payment_orders`。
- 若已有用户开始消费本批次流量卡，回滚前必须先核对 `traffic_credit_ledger` deduction 和 `usage_logs`，不能直接删除已参与计费的事实。

## 验证

- 发放后检查本批次订单、额度、purchase ledger、审计日志数量均为 119。
- 检查本批次 `initial_usd` 与 `remaining_usd` 合计均为 1190 USD。
- 检查目标 active 用户缺失发放数量为 0。
- 检查非 active 或软删除用户误发数量为 0。
