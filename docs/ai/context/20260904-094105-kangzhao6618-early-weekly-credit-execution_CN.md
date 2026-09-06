# 用户 kangzhao6618@gmail.com 提前刷新周额度记录（第二次）

## 授权与范围

按管理员要求，提前刷新用户 `kangzhao6618@gmail.com`（`users.id=484`）当前余额套餐的下一周额度。

本次仅处理当前有效余额套餐 `user_balance_packages.id=190` 及关联订单 `payment_orders.id=757`。未修改历史套餐、订单金额、订单状态、退款金额、套餐有效期、历史用量、API Key 配额或流量卡。

> 该用户 2026-08-29 已提前刷新过一次，对象为旧套餐 `id=175`/订单 `724`（现为 `cancelled`）。本次为其续费后的新套餐 `id=190`/订单 `757` 的独立处理。历史套餐 `173`（订单 717）同为 `cancelled`、`14/56` 为 `expired`，均不在本次范围内。

## 执行前状态

生产数据库执行前核查（`balance` 与 `remaining_usd` 随实时计费变动）：

- 当前套餐：`id=190`（订单 757，plan 25），状态 `active`，已到账 `1/4` 期。
- 每期额度：`206 USD`；`refresh_count=4`，`refresh_interval_days=7`。
- 原定下一次刷新：`2026-09-08 10:48:04.150897 +08:00`。
- 套餐有效期：至 `2026-09-29 10:48:04.150897 +08:00`。
- 订单 `757`：`COMPLETED`，退款金额 `0`，实付 `79.79 CNY`，快照（plan 25 / weekly 206 / rc 4 / ivl 7 / vd 28）与套餐一致。
- 订单 `757` 尚无 `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2` / `BALANCE_PACKAGE_WEEKLY_CREDIT_2` 审计，满足幂等前置检查。
- 该用户存在 `2` 把未删除 API Key。
- 执行前普通余额为 `-0.09705905 USD`（微小欠费），套餐窗口剩余 `0`。

## 执行方式

执行时间为 `2026-09-04 09:41:05 +08:00`。使用 PostgreSQL `SERIALIZABLE` 事务，事务内先锁用户行再锁当前套餐与关联订单（均 `FOR UPDATE`），与计费路径 `lockCurrentBalancePackage` 的套餐行锁串行化，锁内读到的余额/剩余即为写入依据。

断言仅覆盖结构性稳定字段（每期额度 `206`、`refresh_count=4`、`credited_count=1`、`status=active`、`next_credit_at` 等于原定时间、有效期未过、`remaining ∈ [0, weekly]`、订单 `COMPLETED` 且退款 `0`、幂等审计不存在）。余额与剩余按锁内实时值即时计算，沿用生产 `creditDueBalance` 语义：

- `base = balance - remaining`
- `new_remaining = base>=0 ? weekly : max(weekly+base, 0)`
- `balance_delta = weekly - remaining`（AddBalance）
- `debt_repaid = min(max(-base,0), weekly)`

锁内实时值：`balance=-0.09705905`、`remaining=0` → `base=-0.09705905`，`new_remaining=205.90294095`，`balance_delta=206`，`new_balance=205.90294095`，`debt_repaid=0.09705905`。

## 执行结果

- 套餐 `190`：到账 `1/4` -> `2/4`，剩余 `0` -> `205.90294095 USD`，状态保持 `active`。
- `next_credit_at`：`2026-09-08 10:48:04.150897 +08:00` -> `2026-09-15 10:48:04.150897 +08:00`。
- 套餐有效期保持 `2026-09-29 10:48:04.150897 +08:00` 不变。
- 用户普通余额：`-0.09705905` -> 提交时 `205.90294095 USD`（本周额度先偿还 `0.09705905` 欠费，剩余进入套餐窗口；此后随实时计费继续变动）。
- `users.total_recharged`：`692.32000000` -> `898.32000000 USD`（按本期额度 `+206`）。
- 订单 `757` 仍为 `COMPLETED`，退款金额保持 `0`。
- 新增支付审计 `payment_audit_logs.id=2418`：`BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2`，`detail={"credit_usd":206,"credited_count":2}`，操作者 `admin:authorized_manual_settlement`。
- 新增欠费还款流水 `balance_debt_ledger.id=67`：`repayment` `0.09705905 USD`，`balance_before=-0.09705905`，`balance_after=205.90294095`，`source_type=balance_package_weekly_credit`，`source_ref=package:190:credit:2`。

## 缓存与核验

- 事务内经数据库函数 `enqueue_auth_cache_invalidation(key)` 为该用户 `2` 把在用 API Key 写入 `auth_cache_invalidation_outbox`（`cache_key=encode(sha256(key),'hex')`，全程不接触 Key 明文）。worker 完成第一遍失效并在安全延迟后完成第二遍复删，回读 outbox 待处理数为 `0`。
- Redis 余额缓存 `billing:balance:484`：执行前后均不存在（`DEL` 返回 `0`），下次请求按新余额重算。
- 两把 API Key 的 L2 鉴权缓存失效后当前不存在（`EXISTS=0`），下次鉴权将从数据库按新余额重建，无旧余额残留。
- 套餐进度、剩余、下次刷新、有效期、订单状态、支付审计与欠费流水均已从生产数据库回读确认。
- 应用容器、PostgreSQL、Redis 均为 `healthy`；`http://127.0.0.1:18082/health` 返回 HTTP `200`。

## 备注

- 本次未触碰代码、容器或 ZPay 账户；仅对生产数据库执行上述最小必要写入，并复用应用自身的缓存失效机制。
