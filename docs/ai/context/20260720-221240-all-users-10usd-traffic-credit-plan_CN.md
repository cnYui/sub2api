# 全用户 10 USD 流量卡发放计划

## 背景

- 用户要求：给当前项目所有人发送 10 刀流量卡。
- 当前运行态：本机 Docker 正在运行 `sub2api-candidate` 与 `sub2api-candidate-postgres`，公网候选栈监听 `127.0.0.1:18084`。
- 流量卡事实表：`user_traffic_credits`，每条权益必须绑定唯一 `payment_orders.id`。
- 审计链：`payment_orders` 记录订单生命周期，`traffic_credit_ledger` 记录 `purchase` 入账。

## 执行原则

- 默认发放对象：`users.deleted_at IS NULL AND status = 'active'` 的当前项目用户；不包括软删除或非 active 用户。
- 发放内容：每人新增 10.0000000000 USD OpenAI 流量卡，365 天有效期。
- 不改用户钱包余额；模型请求仍按既有“套餐额度 -> 流量卡额度”规则扣费。
- 使用固定批次号 `grant-20260720-10usd-current-users`，保证重复执行不会重复发放，且 `out_trade_no` 不接近 64 字段上限。
- 写入前必须备份 PostgreSQL，并验证备份可读。

## 写入方案

1. 查询用户数量、目标用户数量、现有流量卡 pack、现有同批次订单数量。
2. 对数据库做 custom dump 备份，输出到 `deploy/candidate/dumps/`。
3. 通过 `pg_restore --list` 验证备份文件可读。
4. 在单事务内：
   - 读取现有 `traffic_packs.code = gpt_traffic_10usd_3cny` 作为流量卡快照来源。
   - 为每个目标用户创建一笔已完成 `payment_orders`：
     - `order_type = traffic_pack`
     - `payment_type = manual_grant`
     - `status = COMPLETED`
     - `amount/pay_amount/balance_amount/gateway_amount = 0`
     - `provider_snapshot` 固化 10 USD、365 天、OpenAI 平台。
     - `out_trade_no = grant-20260720-10usd-current-users-u{user_id}`
   - 以 `payment_orders.out_trade_no` 和 `user_traffic_credits.order_id` 幂等。
   - 为新建订单写入 `user_traffic_credits`、`traffic_credit_ledger.purchase` 和 `TRAFFIC_PACK_MANUAL_GRANT` 审计日志。
   - 将该用户未确认的流量卡耗尽提示设为已确认，避免新卡发放后仍展示旧耗尽提示。
5. 写入后验证：
   - 同批次订单数量。
   - 同批次流量卡数量。
   - 同批次入账 ledger 数量。
   - 同批次总初始金额和剩余额。

## 回滚边界

- 首选回滚：按本批次 `out_trade_no` 删除未被扣费的 `user_traffic_credits`、对应 `traffic_credit_ledger.purchase`、`payment_audit_logs` 和 `payment_orders`。
- 若发放后已经发生扣费，不能直接删除；应基于备份和 ledger 差异做人工补偿/核销。
- 已确认的耗尽提示若需恢复，需要从备份比对 `traffic_credit_exhaustion_events.acknowledged_at`。
