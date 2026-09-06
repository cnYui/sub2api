# 859591608@qq.com 续费后手动刷新执行记录

## 请求与前置核查

- 执行时间：2026-08-31 09:27:05（Asia/Tokyo）；生产数据库事务实际提交时间约为 2026-08-31 09:26:04（Asia/Shanghai）。
- 用户：`859591608@qq.com`，用户 ID `492`。
- 续费订单：`payment_orders.id=750`，状态 `COMPLETED`，关联余额套餐 `user_balance_packages.id=114`。
- 执行前套餐为 `completed`、到账 `4/4`、剩余 `98.33080300 USD`、`next_credit_at=NULL`；续费仅将有效期延长至 `2026-09-28 07:45:11.629679 +08`。
- 执行前确认不存在 `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_1` 审计，避免重复刷新。

## 事务执行

- 以续费前到期点 `2026-08-31 07:45:11.629679 +08` 作为新周期起点，发放第 `1/4` 期周额度 `520 USD`。
- 用户普通余额由 `98.33080300 USD` 调整为 `520.00000000 USD`；套餐当前剩余额度同步为 `520.00000000 USD`。
- `users.total_recharged` 由 `1560.00000000 USD` 增至 `2080.00000000 USD`。
- 套餐状态由 `completed` 改为 `active`，`credited_count` 重置为 `1`，`next_credit_at` 设置为 `2026-09-07 07:45:11.629679 +08`；有效期保持不变。
- 当前余额扣除原套餐剩余后基础余额为 `0`，因此本次未产生 `balance_debt_ledger` 还款流水。
- 新增支付审计 `payment_audit_logs.id=2297`，动作 `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_1`，操作者 `admin:authorized_manual_settlement`。
- 事务内为该用户 13 个未删除 API Key 写入认证缓存失效 outbox；未记录或输出 API Key 原文。

## 缓存与验证

- Redis 余额缓存 `billing:balance:492` 删除返回 `0`，回读 `EXISTS=0`。
- 认证缓存失效 outbox 待处理数由 `13` 降为 `0`，最大重试次数为 `0`；该用户对应事件已全部消费。
- 应用、PostgreSQL、Redis 容器均为 `healthy`；`http://127.0.0.1:18082/health` 返回 HTTP `200`。
- 最终数据库回读：余额 `520.00000000 USD`，套餐 `active`、`1/4`，下次刷新 `2026-09-07 07:45:11 +08`，有效期 `2026-09-28 07:45:11 +08`。

