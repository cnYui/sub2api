# 用户 2894823499@qq.com 提前刷新周额度记录（第二期，VPS 生产库）

## 授权与范围

按管理员要求，提前刷新用户 `2894823499@qq.com`（`users.id=525`）当前余额套餐的下一周额度（第 2 期）。

本次仅处理当前有效余额套餐 `user_balance_packages.id=191` 及关联订单 `payment_orders.id=765`。未修改历史套餐（`30`/`61` expired、`71` refunded）、订单金额、订单状态、退款金额、套餐有效期、历史用量、API Key 配额或流量卡。

执行目标为 **VPS 生产库**（容器 `sub2api-postgres` / `sub2api-redis`，`root@${OPS_VPS_HOST}`），方法见 `20260905-173123-vps-ssh-db-operations-runbook_CN.md`。

## 执行前状态（VPS 生产库只读核查，VPS 时间 2026-09-08 09:22）

- 当前套餐：`id=191`（订单 765，plan 21），状态 `active`，已到账 `1/4` 期。
- 每期额度：`76 USD`；`refresh_count=4`，`refresh_interval_days=7`。
- 原定下一次刷新：`2026-09-10 11:07:20.258688 +08:00`。
- 套餐有效期：至 `2026-10-01 11:07:20.258688 +08:00`。
- 订单 `765`：`COMPLETED`，退款金额 `0`，快照 plan 21 / weekly 76 / rc 4 / ivl 7 / vd 28。
- 订单 `765` 已有审计：期1 `BALANCE_PACKAGE_INITIAL_CREDIT`（id=2385）；无期 2 审计，满足幂等前置检查。
- 该用户存在 `2` 把未删除 API Key。
- 执行前普通余额为 `-0.48650588 USD`（微小欠费），套餐窗口剩余 `0`。

## 执行方式

经 SSH 在 VPS `sub2api-postgres` 内以 `docker exec -i ... psql` + stdin 运行。使用 PostgreSQL `SERIALIZABLE` 事务，锁用户/套餐/订单（`FOR UPDATE`），断言结构性稳定字段（每期额度 `76`、`credited_count=1`、`refresh_count=4`、`status=active`、`next_credit_at=2026-09-10 11:07:20.258688+08`、有效期未过、`remaining ∈ [0, weekly]`、订单 `COMPLETED`/退款 `0`、期 2 幂等审计不存在）。余额与剩余按锁内实时值即时计算，沿用生产 `creditDueBalance` 语义。

锁内实时值：`balance=-0.48650588`、`remaining=0` → `base=-0.48650588`，`new_remaining=75.51349412`，`balance_delta=76`，`new_balance=75.51349412`，`debt_repaid=0.48650588`。

## 执行结果

- 套餐 `191`：到账 `1/4` -> `2/4`，剩余 `0` -> `75.51349412 USD`，状态保持 `active`。
- `next_credit_at`：`2026-09-10 11:07:20.258688 +08:00` -> `2026-09-17 11:07:20.258688 +08:00`。
- 套餐有效期保持 `2026-10-01 11:07:20.258688 +08:00` 不变。
- 用户普通余额：`-0.48650588` -> 提交时 `75.51349412 USD`（本周额度先偿还 `0.48650588` 欠费，剩余进入套餐窗口；此后随实时计费变动）。
- `users.total_recharged`：`464.80000000` -> `540.80000000 USD`（按本期额度 `+76`）。
- 订单 `765` 仍为 `COMPLETED`，退款金额保持 `0`。
- 新增支付审计 `payment_audit_logs.id=2565`：`BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2`，`detail={"credit_usd":76,"credited_count":2}`，操作者 `admin:authorized_manual_settlement`，DB 时间 `2026-09-08 09:23:10+08`。
- 新增欠费还款流水 `balance_debt_ledger.id=79`：`repayment` `0.48650588 USD`，`balance_before=-0.48650588`，`balance_after=75.51349412`，`source_ref=package:191:credit:2`。

## 缓存与核验

- 事务内经 `enqueue_auth_cache_invalidation(key)` 为该用户 `2` 把在用 API Key 写入 `auth_cache_invalidation_outbox`（不接触明文）。worker 两遍失效完成，回读 outbox 待处理数为 `0`。
- Redis `billing:balance:525`：执行前后均不存在（`DEL` 返回 `0`）。
- 两把 API Key 的 L2 鉴权缓存失效后当前不存在（`EXISTS=0`），下次鉴权按新余额重建。
- 套餐进度、剩余、下次刷新、有效期、订单状态、支付审计与欠费流水均从 VPS 生产库回读确认。
- VPS 应用/PostgreSQL/Redis 容器均 `healthy`；容器内 `http://127.0.0.1:8080/health` 返回 `HTTP/1.1 200 OK`。

## 备注

- 该套餐 4 期中已发放 2 期，剩 2 期，`next_credit_at=2026-09-17`。
- 本次未触碰代码、容器或 ZPay 账户；仅对 VPS 生产数据库执行最小必要写入，并复用应用自身的缓存失效机制。
