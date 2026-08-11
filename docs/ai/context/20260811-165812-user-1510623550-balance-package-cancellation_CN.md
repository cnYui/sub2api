# 用户 1510623550@qq.com 余额套餐人工取消

## 授权与范围

按管理员要求取消用户 `1510623550@qq.com`（`users.id=476`）的当前余额套餐，使其可以重新购买。

本次不调整普通余额、流量卡、支付订单、退款状态、历史用量或 API Key，也不调用支付网关。

## 执行前状态

- 普通余额：`-0.48059154 USD`。
- 当前套餐：`user_balance_packages.id=136`，关联订单 `625`。
- 套餐状态：`debt_paused`，每周额度 `206 USD`，当前窗口剩余 `0 USD`，已到账 `1/4`。
- 下一次到账：`2026-08-17 15:35:56.911376 +08:00`。
- 订单状态：`REFUND_FAILED`，退款金额 `59.20 CNY`。
- 有效 OpenAI 流量卡额度：`0.00065645 USD`。

## 执行方式

在 PostgreSQL 单事务内锁定用户、套餐和关联订单，并断言上述关键状态未变化后：

- 将套餐更新为 `cancelled`。
- 清零 `remaining_usd` 并清空 `next_credit_at`，停止后续周额度到账。
- 写入 `BALANCE_PACKAGE_MANUAL_CANCELLATION` 支付审计。
- 提交后失效 Redis `billing:balance:476` 缓存。

## 执行结果

- 套餐 `136` 已取消，`remaining_usd=0`、`next_credit_at=NULL`。
- 当前有效余额套餐数量为 `0`，用户已可重新购买。
- 普通余额保持 `-0.48059154 USD`；有效流量卡额度保持 `0.00065645 USD`。
- 订单 `625` 仍为 `REFUND_FAILED`，退款金额仍为 `59.20 CNY`；未伪造退款成功。
- 审计记录：`payment_audit_logs.id=1564`。
- Redis 余额缓存失效后不存在（`DEL=0`、`EXISTS=0`，表示执行前后均无该键）。
