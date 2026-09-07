# 用户 2197057077@qq.com 提前刷新周额度记录（第三期，VPS 生产库）

## 授权与范围

按管理员要求，提前刷新用户 `2197057077@qq.com`（`users.id=565`）当前余额套餐的下一周额度（第 3 期）。

本次仅处理当前有效余额套餐 `user_balance_packages.id=121` 及关联订单 `payment_orders.id=763`。未修改历史套餐（`47` cancelled、`70`/`80` refunded）、订单金额、订单状态、退款金额、套餐有效期、历史用量、API Key 配额或流量卡。

执行目标为 **VPS 生产库**（容器 `sub2api-postgres` / `sub2api-redis`，`root@${OPS_VPS_HOST}`），方法见 `20260905-173123-vps-ssh-db-operations-runbook_CN.md`。

## 执行前状态（VPS 生产库只读核查）

- 当前套餐：`id=121`（订单 763，plan 28），状态 `active`，已到账 `2/4` 期。该套餐为 2026-09-02 续费重置后的周期（审计 `BALANCE_PACKAGE_RENEWAL`）。
- 每期额度：`520 USD`；`refresh_count=4`，`refresh_interval_days=7`。
- 原定下一次刷新：`2026-09-16 22:26:42.153829 +08:00`。
- 套餐有效期：至 `2026-09-30 22:26:42.153829 +08:00`。
- 订单 `763`：`COMPLETED`，退款金额 `0`，快照 plan 28 / weekly 520 / rc 4 / ivl 7 / vd 28。
- 订单 `763` 已有到账审计：期1 `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_1`（id=2376）、期2 `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2`（id=2447），均为手动 EARLY；无期 3 审计，满足幂等前置检查。
- 该用户存在 `2` 把未删除 API Key。
- 执行前普通余额为 `-2.21415397 USD`（欠费），套餐窗口剩余 `0`。

## 执行方式

经 SSH 在 VPS `sub2api-postgres` 内以 `docker exec -i ... psql` + stdin 运行。使用 PostgreSQL `SERIALIZABLE` 事务，锁用户/套餐/订单（`FOR UPDATE`），断言结构性稳定字段（每期额度 `520`、`credited_count=2`、`refresh_count=4`、`status=active`、`next_credit_at=2026-09-16 22:26:42.153829+08`、有效期未过、`remaining ∈ [0, weekly]`、订单 `COMPLETED`/退款 `0`、期 3 幂等审计不存在）。余额与剩余按锁内实时值即时计算，沿用生产 `creditDueBalance` 语义。

锁内实时值：`balance=-2.21415397`、`remaining=0` → `base=-2.21415397`，`new_remaining=517.78584603`，`balance_delta=520`，`new_balance=517.78584603`，`debt_repaid=2.21415397`。

## 执行结果

- 套餐 `121`：到账 `2/4` -> `3/4`，剩余 `0` -> `517.78584603 USD`，状态保持 `active`。
- `next_credit_at`：`2026-09-16 22:26:42.153829 +08:00` -> `2026-09-23 22:26:42.153829 +08:00`。
- 套餐有效期保持 `2026-09-30 22:26:42.153829 +08:00` 不变。
- 用户普通余额：`-2.21415397` -> 提交时 `517.78584603 USD`（本周额度先偿还 `2.21415397` 欠费，剩余进入套餐窗口；此后随实时计费变动）。
- `users.total_recharged`：`3290.00000000` -> `3810.00000000 USD`（按本期额度 `+520`）。
- 订单 `763` 仍为 `COMPLETED`，退款金额保持 `0`。
- 新增支付审计 `payment_audit_logs.id=2470`：`BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_3`，`detail={"credit_usd":520,"credited_count":3}`，操作者 `admin:authorized_manual_settlement`，DB 时间 `2026-09-06 13:58:13+08`。
- 新增欠费还款流水 `balance_debt_ledger.id=72`：`repayment` `2.21415397 USD`，`balance_before=-2.21415397`，`balance_after=517.78584603`，`source_ref=package:121:credit:3`。

## 缓存与核验

- 事务内经 `enqueue_auth_cache_invalidation(key)` 为该用户 `2` 把在用 API Key 写入 `auth_cache_invalidation_outbox`（`cache_key=encode(sha256(key),'hex')`，不接触明文）。worker 两遍失效完成，回读 outbox 待处理数为 `0`。
- Redis `billing:balance:565`：执行前后均不存在（`DEL` 返回 `0`）。
- 两把 API Key 的 L2 鉴权缓存失效后当前不存在（`EXISTS=0`），下次鉴权按新余额重建。
- 套餐进度、剩余、下次刷新、有效期、订单状态、支付审计与欠费流水均从 VPS 生产库回读确认。
- VPS 应用/PostgreSQL/Redis 容器均 `healthy`；容器内 `http://127.0.0.1:8080/health` 返回 `HTTP/1.1 200 OK`。

## 备注

- 该套餐 4 期中已发放 3 期（期1/期2/期3 均为手动 EARLY），剩最后 1 期，`next_credit_at=2026-09-23`。
- 本次未触碰代码、容器或 ZPay 账户；仅对 VPS 生产数据库执行最小必要写入，并复用应用自身的缓存失效机制。
