# 用户 441565547@qq.com 套餐移除与余额清零

## 授权与范围

按管理员要求处理用户 `441565547@qq.com`（`users.id=472`）：移除当前余额套餐并将普通余额清零，用户后续重新购买。

本次不修改支付订单、退款状态或退款金额，不修改历史用量、API Key、流量卡和其它用户数据。

## 执行前状态

- 普通余额：`-0.95582240 USD`
- 当前余额套餐：`user_balance_packages.id=10`，计划 `balance_package_plans.id=21`，订单 `541`
- 套餐状态：`debt_paused`
- 套餐本周剩余：`0 USD`
- 已到账期数：`2/4`，每周额度 `76 USD`
- 下一次刷新：`2026-08-15 13:01:56.296186 +08:00`
- 到期时间：`2026-08-29 13:01:56.296186 +08:00`
- 订单状态：`REFUND_FAILED`
- 退款金额：`12.43 CNY`
- 用户没有 `user_subscriptions` 记录；流量卡不属于本次处理范围。

## 执行方式

在 PostgreSQL 单事务中锁定用户、套餐及关联订单，并断言执行前余额、套餐状态、期数、关联订单和退款状态未变化。随后：

1. 将套餐 `id=10` 标记为 `cancelled`，`remaining_usd` 置零，清空 `next_credit_at`，保留历史期数、开始时间和到期时间。
2. 将用户 `users.id=472` 的普通余额从 `-0.95582240 USD` 原子更新为 `0 USD`。
3. 写入 `payment_audit_logs`：`BALANCE_PACKAGE_MANUAL_CANCELLATION`、`BALANCE_MANUAL_RESET`。
4. 清理 Redis `billing:balance:472` 缓存。

## 执行结果

- 用户普通余额：`0.00000000 USD`。
- 套餐 `id=10`：`cancelled`，`remaining_usd=0`，`next_credit_at=NULL`，不会继续自动到账。
- 订单 `541`：仍为 `REFUND_FAILED`，退款金额仍为 `12.43 CNY`，未调用支付网关或伪造退款成功。
- Redis 余额缓存核验：`billing:balance:472` 不存在（`EXISTS=0`）。
- 审计记录：`payment_audit_logs.id=1488`、`1489`。

