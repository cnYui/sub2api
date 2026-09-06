# 用户 731205940@qq.com 提前刷新周额度记录

## 授权与范围

按管理员要求，提前刷新用户 `731205940@qq.com`（`users.id=516`）当前余额套餐的下一周额度。

本次仅处理当前有效余额套餐 `user_balance_packages.id=192` 及关联订单 `payment_orders.id=768`。未修改历史套餐（旧套餐 `id=25` 已 `expired`）、订单金额、订单状态、退款金额、套餐有效期、历史用量、API Key 配额或流量卡。

## 执行前状态

生产数据库执行前核查（该用户执行时仍在持续调用，`balance` 与 `remaining_usd` 为实时变动值）：

- 当前套餐：`id=192`（订单 768，plan 27），状态 `active`，已到账 `1/4` 期。
- 每期额度：`389 USD`；`refresh_count=4`，`refresh_interval_days=7`。
- 原定下一次刷新：`2026-09-10 18:19:03.270051 +08:00`。
- 套餐有效期：至 `2026-10-01 18:19:03.270051 +08:00`。
- 订单 `768`：`COMPLETED`，退款金额 `0`，实付 `150.49 CNY`，快照（plan 27 / weekly 389 / rc 4 / ivl 7 / vd 28）与套餐一致。
- 订单 `768` 尚无 `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2` / `BALANCE_PACKAGE_WEEKLY_CREDIT_2` 审计，满足幂等前置检查。
- 该用户存在 `1` 把未删除 API Key（`api_keys.id=111`）。

> 说明：首次只读快照时 `balance=386.31172430`、`remaining=385.52765780`；因用户持续消费，正式执行的锁内实时值已降至 `balance=remaining=379.62627140`。这印证了余额与套餐 `remaining_usd` 会随计费同步递减，故二者均不能作为固定前置断言。

## 执行方式

执行时间为 `2026-09-03 18:33:34 +08:00`。使用 PostgreSQL `SERIALIZABLE` 事务，事务内先锁用户行（`FOR UPDATE`）再锁当前套餐与关联订单（`FOR UPDATE`），与计费路径 `lockCurrentBalancePackage` 的套餐行锁串行化，保证锁内读到的余额/剩余即为写入依据。

断言仅覆盖结构性稳定字段：每期额度 `389`、`refresh_count=4`、`credited_count=1`、`status=active`、`next_credit_at` 等于原定时间、有效期未过、`remaining ∈ [0, weekly]`、订单 `COMPLETED` 且退款 `0`、幂等审计不存在。余额与剩余按锁内实时值即时计算，严格沿用生产 `creditDueBalance` 语义：

- `base = balance - remaining`
- `new_remaining = base>=0 ? weekly : max(weekly+base, 0)`
- `balance_delta = weekly - remaining`（AddBalance）
- `debt_repaid = min(max(-base,0), weekly)`

锁内实时值：`balance=remaining=379.62627140` → `base=0`，`new_remaining=389`，`balance_delta=9.37372860`，`new_balance=389`，`debt_repaid=0`。

## 执行结果

- 套餐 `192`：到账 `1/4` -> `2/4`，剩余 `379.62627140` -> `389.00000000 USD`，状态保持 `active`。
- `next_credit_at`：`2026-09-10 18:19:03.270051 +08:00` -> `2026-09-17 18:19:03.270051 +08:00`。
- 套餐有效期保持 `2026-10-01 18:19:03.270051 +08:00` 不变。
- 用户普通余额：锁内 `379.62627140` -> 提交时 `389.00000000 USD`（此后随实时计费继续变动）。
- `users.total_recharged`：`1163.00000000` -> `1552.00000000 USD`（按本期额度 `+389`）。
- 订单 `768` 仍为 `COMPLETED`，退款金额保持 `0`。
- 新增支付审计 `payment_audit_logs.id=2403`：`BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2`，`detail={"credit_usd":389,"credited_count":2}`，操作者 `admin:authorized_manual_settlement`。
- 本次 `base=0`，欠费抵扣为 `0`，未新增 `balance_debt_ledger` 还款流水。

## 缓存与核验

- 事务内经数据库函数 `enqueue_auth_cache_invalidation(key)` 为该用户 `1` 把在用 API Key 写入 `auth_cache_invalidation_outbox`（`cache_key=encode(sha256(key),'hex')`，全程不接触 Key 明文）。worker 完成第一遍失效（删 L2 + 发布 `auth:cache:invalidate` + 清 L1）并在安全延迟后完成第二遍复删，回读 outbox 待处理数为 `0`、最大重试 `0`。
- Redis 余额缓存 `billing:balance:516`：执行前后均不存在（`DEL` 返回 `0`），下次请求按新余额从数据库重算。
- API Key `id=111` 的 L2 鉴权缓存在失效后被重新生成，快照 `user.balance=389`、`total_recharged=1552`，为新值，非旧余额。
- 套餐进度、剩余、下次刷新、有效期、订单状态、支付审计均已从生产数据库回读确认。
- 应用容器、PostgreSQL、Redis 均为 `healthy`；`http://127.0.0.1:18082/health` 返回 HTTP `200`。

## 备注

- 本次未触碰代码、容器或 ZPay 账户；仅对生产数据库执行上述最小必要写入，并复用应用自身的缓存失效机制。
- 该用户此前于 2026-08-11 处理过一次（旧套餐 `id=25`/订单 `582`），本次为续费后新套餐 `id=192`/订单 `768` 的独立处理。
