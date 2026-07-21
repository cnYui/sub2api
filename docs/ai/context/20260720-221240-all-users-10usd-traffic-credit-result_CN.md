# 全用户 10 USD 流量卡发放结果

## 执行对象

- 数据库：`sub2api-candidate-postgres`
- 业务容器：`sub2api-candidate`
- 目标用户：`users.deleted_at IS NULL AND status = 'active'`
- 目标数量：113
- 目标卡：`traffic_packs.code = 'gpt_traffic_10usd_3cny'`
- 卡包 ID：`2`

## 备份

- 备份文件：`/candidate/dumps/sub2api-candidate-before-all-users-10usd-traffic-credit-20260720-221240.dump`
- 已通过 `pg_restore --list` 验证可读

## 批次

- 批次号：`grant-20260720-10usd-current-users`
- `out_trade_no` 前缀：`grant-20260720-10usd-current-users-u`
- `payment_type`：`manual_grant`
- `order_type`：`traffic_pack`
- `status`：`COMPLETED`
- 发放时间：`2026-07-20 21:17:32.500913+08`
- 到期时间：`2027-07-20 21:17:32.500913+08`

## 写入结果

- `payment_orders`：113
- `user_traffic_credits`：113
- `traffic_credit_ledger` purchase：113
- `payment_audit_logs`：113
- `traffic_credit_exhaustion_events` 额外确认：0

## 金额汇总

- 初始额度合计：1130 USD
- 剩余额度合计：1130 USD
- purchase 流水合计：1130 USD
- 订单金额合计：0
- 实付金额合计：0

## 验证

- 目标用户缺失发放：0
- 非目标用户误发：0
- 订单语义异常：0
- 流量卡额度异常：0
- `http://127.0.0.1:18084/health`：`ok`
- `http://127.0.0.1:8080/health`：`ok`
- `https://api.aaccx.pw/health`：`ok`

## 说明

- 这次是直接写事实表，不改用户余额，不改容器，不改路由。
- 历史同类批次的展示语义沿用 `manual_grant`，前端会显示为“赠送金额”。
