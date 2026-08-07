# 用户 524 余额套餐 140 天有效期赠送

## 请求与范围

- 按管理员请求，为用户 `524`（`caogang@sdufe.edu.cn`）当前的余额套餐按 28 天周期赠送 `140` 天，即 `5` 个完整周期。
- 目标为余额套餐记录 `107`，套餐为 `balance-99`（余额套餐 ¥99），关联订单 `571`。
- 仅延长套餐有效期；不新增余额、不重置周额度、不修改流量卡。

## 执行口径

- 当前套餐已完成全部 `4/4` 次周刷新，状态为 `completed`，没有待发放的后续周额度。
- 现有同档套餐续费规则只延长 `expires_at`，不增加 `refresh_count`、不重置 `credited_count`，本次沿用该口径。
- 在 PostgreSQL 串行化事务中锁定套餐后，仅更新 `user_balance_packages.expires_at` 与 `updated_at`。
- 同事务新增 `payment_audit_logs` 审计，动作 `BALANCE_PACKAGE_MANUAL_EXTENSION`，审计 ID `1214`，记录赠送天数、周期数、修改前后到期时间及不变字段。

## 结果与核验

- 到期时间从 `2026-08-07 14:26:32 +08` 延长至 `2026-12-25 14:26:32 +08`，精确增加 `140` 天。
- 套餐的 `credited_count=4`、`refresh_count=4`、`refresh_interval_days=7`、`next_credit_at=NULL` 保持不变，因此不会额外产生套餐额度。
- 事务未修改 `users.balance`、`remaining_usd` 或 `user_traffic_credits`。核对窗口内用户存在实时请求，余额和本周套餐剩余额度由正常结算从 `154.12936150 USD` 变为 `153.91400500 USD`，与延期操作无关。
- 有效 OpenAI 流量卡仍为 `2` 张，合计剩余 `20.0000000000 USD`。
