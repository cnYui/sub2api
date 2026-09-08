# 用户 kangzhao6618@gmail.com 提前刷新周额度记录（第四期 / 末期，套餐完成，VPS 生产库）

## 授权与范围

按管理员要求，提前刷新用户 `kangzhao6618@gmail.com`（`users.id=484`）当前余额套餐的最后一周额度（第 4 期）。发放后该套餐 4 期全部到账、`status` 置为 `completed`、`next_credit_at` 清空。

本次仅处理当前有效余额套餐 `user_balance_packages.id=190` 及关联订单 `payment_orders.id=757`。未修改历史套餐、订单金额、订单状态、退款金额、套餐有效期、历史用量、API Key 配额或流量卡。

执行目标为 **VPS 生产库**（容器 `sub2api-postgres` / `sub2api-redis`，`root@${OPS_VPS_HOST}`），方法见 `20260905-173123-vps-ssh-db-operations-runbook_CN.md`。

## 执行前状态（VPS 生产库只读核查，VPS 时间 2026-09-07 17:07）

- 当前套餐：`id=190`（订单 757，plan 25），状态 `active`，已到账 `3/4` 期。
- 每期额度：`206 USD`；`refresh_count=4`，`refresh_interval_days=7`。
- 原定下一次刷新：`2026-09-22 10:48:04.150897 +08:00`。
- 套餐有效期：至 `2026-09-29 10:48:04.150897 +08:00`。
- 订单 `757`：`COMPLETED`，退款金额 `0`。
- 订单 `757` 已有到账审计：期1 INITIAL（id=2343）、期2 EARLY（id=2418）、期3 EARLY（id=2469）；无期 4 审计，满足幂等前置检查。
- 该用户存在 `2` 把未删除 API Key。
- 执行前普通余额为 `5.50021832 USD`（正），套餐窗口剩余 `5.50021832`，`base = 0`，无欠费。

## 执行方式

经 SSH 在 VPS `sub2api-postgres` 内以 `docker exec -i ... psql` + stdin 运行。使用 PostgreSQL `SERIALIZABLE` 事务，锁用户/套餐/订单（`FOR UPDATE`），断言结构性稳定字段（每期额度 `206`、`credited_count=3`、`refresh_count=4`、`status=active`、`next_credit_at=2026-09-22 10:48:04.150897+08`、有效期未过、`remaining ∈ [0, weekly]`、订单 `COMPLETED`/退款 `0`、期 4 幂等审计不存在）。余额与剩余按锁内实时值即时计算，沿用生产 `creditDueBalance` 语义。

**末期完成分支**（`newCount = 4 >= refresh_count`）：`status` 置 `completed`、`next_credit_at` 清空（不再 `+7` 天）；本周额度照常入账。

锁内实时值：`balance=5.50021832`、`remaining=5.50021832` → `base=0`，`new_remaining=206`，`balance_delta=200.49978168`，`new_balance=206`，`debt_repaid=0`。

## 执行结果

- 套餐 `190`：到账 `3/4` -> `4/4`，剩余 `5.50021832` -> `206.00000000 USD`，状态 `active` -> `completed`，`next_credit_at` 由 `2026-09-22 10:48:04+08` 清空为 `NULL`。
- 套餐有效期保持 `2026-09-29 10:48:04.150897 +08:00` 不变；剩余 `206 USD` 可用至到期。
- 用户普通余额：`5.50021832` -> 提交时 `206.00000000 USD`。
- `users.total_recharged`：`1104.32000000` -> `1310.32000000 USD`（按本期额度 `+206`）。
- 订单 `757` 仍为 `COMPLETED`，退款金额保持 `0`。
- 新增支付审计 `payment_audit_logs.id=2551`：`BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_4`，`detail={"credit_usd":206,"credited_count":4}`，操作者 `admin:authorized_manual_settlement`，DB 时间 `2026-09-07 17:08:38+08`。
- 本次 `base=0`，欠费抵扣为 `0`，未新增 `balance_debt_ledger` 还款流水。

## 缓存与核验

- 事务内经 `enqueue_auth_cache_invalidation(key)` 为该用户 `2` 把在用 API Key 写入 `auth_cache_invalidation_outbox`（不接触明文）。worker 两遍失效完成，回读 outbox 待处理数为 `0`。
- Redis `billing:balance:484`：执行前后均不存在（`DEL` 返回 `0`）。
- 两把 API Key 的 L2 鉴权缓存失效后当前不存在（`EXISTS=0`），下次鉴权按新余额重建。
- 套餐进度、剩余、状态、`next_credit_at` 已清空、有效期、订单状态、支付审计均从 VPS 生产库回读确认。
- VPS 应用/PostgreSQL/Redis 容器均 `healthy`；容器内 `http://127.0.0.1:8080/health` 返回 `HTTP/1.1 200 OK`。

## 备注

- **该套餐 4 期已全部发放完毕（`completed`）**。本会话内为该用户依次提前发放了期 2/3/4。剩余额度 `206 USD` 在 `2026-09-29` 到期前有效，到期未用部分由定时 `expireBalancePackage` 清除。
- 若之后还要给该用户加额度，**已不能再"提前刷新本套餐"**（无剩余期数），需走续费或新购套餐。
- 本次未触碰代码、容器或 ZPay 账户；仅对 VPS 生产数据库执行最小必要写入，并复用应用自身的缓存失效机制。
