# 2197057077@qq.com 续费后手动刷新执行记录

## 请求与背景

- 执行时间：`2026-09-02 21:10 +08`（Asia/Shanghai，生产数据库事务时间）。
- 管理员要求：把用户 `2197057077@qq.com`（`users.id=565`）的额度提前刷新。
- 背景：该用户当日 `2026-09-02 20:35` 完成同档续费（`payment_orders.id=763`，plan 28，周额度 `520 USD`，实付 `200.99 CNY`）。但当前生产容器仍运行**旧续费逻辑**——续费只延长有效期、不重置周期（审计 `2373 BALANCE_PACKAGE_RENEWAL` 仅含 `validity_days_added=28`，无 `weekly_credit_usd`/`refresh_count`；生产未部署迁移 212，`user_balance_packages` 无 `renewal_count` 列）。因此套餐 `121` 停在 `completed`、`4/4`、`remaining=0`，用户付费续费后未拿到任何新额度，且普通余额为负被硬拦截。
- 处理方式与前次同类场景一致（见 `20260831-092705-859591608-renewal-refresh-execution_CN.md`，订单 750）：人工事务把续费套餐周期重置为新一轮第 `1/4` 期并立即发放。

## 范围

仅处理 `users.id=565`、续费套餐 `user_balance_packages.id=121`、续费订单 `payment_orders.id=763`。未修改订单金额/状态/退款、套餐有效期、历史套餐（`47/70/80`）、历史用量、API Key 配额或流量卡。

## 执行前状态（生产回读）

- 普通余额：`-5.01813080 USD`（负，硬拦截中）；`total_recharged=2250.00000000 USD`。
- 套餐 `121`：`completed`、到账 `4/4`、周额度 `520`、`remaining_usd=0`、`next_credit_at=NULL`、有效期 `2026-09-30 22:26:42.153829 +08`、`payment_order_id=763`。
- 续费前到期点（审计 2373 `previous_expires_at`）：`2026-09-02 22:26:42.153829 +08`。
- 订单 `763`：`COMPLETED`，退款金额 `0`。
- 幂等：订单 `763` 无 `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_1` 审计（`payment_audit_logs` 对 `(order_id, action)` 唯一）。

## 执行方式

单个 `SERIALIZABLE` 事务，`FOR UPDATE` 依次锁定用户、套餐、订单，并校验余额、`total_recharged`、套餐状态/进度/周额度/剩余/`next_credit_at`/有效期、订单状态/退款金额及幂等审计不存在；任一前置不符即回滚（未提交）。

余额口径沿用生产 `creditDueBalance` 语义：`base = balance - old_remaining = -5.01813080`；周额度先偿还负余额，剩余进入套餐窗口。以续费前到期点作为新周期起点，保持每周 `22:26:42` 的到账节奏。

## 执行结果

- 普通余额：`-5.01813080 -> 514.98186920 USD`。
- `total_recharged`：`2250 -> 2770.00000000 USD`。
- 套餐 `121`：`completed -> active`，`4/4 -> 1/4`，`remaining 0 -> 514.98186920`，`starts_at -> 2026-09-02 22:26:42.153829 +08`，`next_credit_at NULL -> 2026-09-09 22:26:42.153829 +08`；有效期保持 `2026-09-30 22:26:42.153829 +08` 不变（旧续费已 +28 天，未再叠加）。`refresh_count` 保持 `4`（旧周期已走完，顺延期数为 0）。
- 欠费还款账本 `balance_debt_ledger.id=64`：`repayment 5.01813080`，`before=-5.01813080`、`after=514.98186920`，`source_type=balance_package_early_weekly_credit`，`source_ref=package:121:credit:1`。
- 支付审计 `payment_audit_logs.id=2376`：`BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_1`，操作者 `admin:authorized_manual_settlement`，明细含 `cycle_reset=true`、`renewal_order_id=763` 等。

## 缓存与核验

- Redis 余额缓存 `billing:balance:565` 执行前后均不存在（下次请求按新余额重算）。
- 该用户 `2` 把未删除 API Key 写入认证缓存失效 outbox（`cache_key=hex(sha256(key))`，原文未记录/输出）；worker 已完成两遍失效并删除 outbox 行，回读待处理数 `0`。两把 Key 的 L2 认证快照回读 `EXISTS=0`（原本亦不存在），确保不残留旧负余额快照导致继续拦截。
- 余额、`total_recharged`、套餐状态/进度/剩余/起始/下次刷新/有效期、订单状态、欠费账本与支付审计均已从生产数据库回读确认。
- 应用、PostgreSQL、Redis 容器均 `healthy`；`http://127.0.0.1:18082/health` 返回 HTTP `200`。

## 备注

- 未触碰代码、容器、部署或 ZPay 账户；仅对生产数据库执行上述最小必要写入。
- 历史被旧续费逻辑仅延长有效期的套餐仍需按需人工刷新，直至迁移 212 + 新续费逻辑上线。
