# 用户 380361319@qq.com 余额套餐人工取消

## 授权与范围

按管理员要求取消用户 `380361319@qq.com`（`users.id=603`）的当前余额套餐。

本次不清零或调整普通余额，不修改支付订单、退款状态、退款金额、历史用量、API Key、流量卡或其它用户数据。

## 执行前状态

- 普通余额：`-0.26459774 USD`
- 当前余额套餐：`user_balance_packages.id=128`，关联订单 `617`
- 套餐状态：`debt_paused`
- 每周额度：`154 USD`
- 当前窗口剩余：`0 USD`
- 已到账/总期数：`1/4`
- 下一次刷新：`2026-08-15 20:28:16.630742 +08:00`
- 到期时间：`2026-09-05 20:28:16.630742 +08:00`
- 订单状态：`REFUND_FAILED`，退款金额 `44.22 CNY`

## 执行方式

在 PostgreSQL 单事务中锁定用户、套餐与关联订单，并断言余额、套餐状态、剩余额度、已到账期数、订单状态和退款金额与执行前核对一致。

- 将套餐状态更新为 `cancelled`。
- 将 `remaining_usd` 置零，清空 `next_credit_at`，避免后续自动到账。
- 写入 `BALANCE_PACKAGE_MANUAL_CANCELLATION` 审计。
- 清理 Redis `billing:balance:603` 缓存。

## 执行结果

- 套餐 `id=128` 已为 `cancelled`，`remaining_usd=0`，`next_credit_at=NULL`。
- 普通余额保持 `-0.26459774 USD`。
- 订单 `617` 仍为 `REFUND_FAILED`，退款金额仍为 `44.22 CNY`；未调用支付网关或伪造退款成功。
- 审计记录：`payment_audit_logs.id=1505`。
- Redis 余额缓存核验：`billing:balance:603` 不存在（`EXISTS=0`）。

