# 用户 kangzhao6618@gmail.com 提前刷新周额度记录（第三期，VPS 生产库）

## 授权与范围

按管理员要求，再次提前刷新用户 `kangzhao6618@gmail.com`（`users.id=484`）当前余额套餐的下一周额度（第 3 期）。管理员在追问确认后明确「是，再发第 3 期」，并指明**当前生产在远程 VPS 上、用 SSH 连**。

本次仅处理当前有效余额套餐 `user_balance_packages.id=190` 及关联订单 `payment_orders.id=757`。未修改历史套餐、订单金额、订单状态、退款金额、套餐有效期、历史用量、API Key 配额或流量卡。

## 环境澄清（重要）

- 本次执行目标是 **VPS 生产库**（容器 `sub2api` / `sub2api-postgres` / `sub2api-redis`，`root@${OPS_VPS_HOST}`，密钥 `${OPS_SSH_KEY_FILE}`），连接与操作方法见 `20260905-173123-vps-ssh-db-operations-runbook_CN.md`。
- 本地 `sub2api-official-18082-*` 容器在本次执行时 **Docker Desktop 后端未运行、已停**；管理员要求不要拉起本地 Docker，改走 VPS。
- **本地 18082 与 VPS 生产为同一套数据**：VPS 上已存在上一期审计 `payment_audit_logs.id=2418`（`BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2`，`2026-09-04 09:41:05+08`），与本会话上一轮在本地写入的行 **id、action、detail、时间戳完全一致**。且原定第 2 期到期时间为 `2026-09-08`，本次执行时定时任务尚不可能发放第 2 期，故 VPS 上的 `2/4` 只能来自上一轮手动 EARLY 发放——据此判定两处为同一数据。
- **时钟提示**：VPS 数据库 `now()` 记录的第 3 期审计时间为 `2026-09-06 10:26:31+08`，比本机会话日期（2026-09-04）靠前。审计以数据库时钟为准；此差异仅为观察项，不影响本次原子操作的正确性，但说明两处环境存在时钟/同步差，后续若有 本地↔VPS 的 restore/同步需警惕互相覆盖。

## 执行前状态（VPS 生产库只读核查）

- 当前套餐：`id=190`（订单 757，plan 25），状态 `active`，已到账 `2/4` 期。
- 每期额度：`206 USD`；`refresh_count=4`，`refresh_interval_days=7`。
- 原定下一次刷新：`2026-09-15 10:48:04.150897 +08:00`（即上一期推后的结果）。
- 套餐有效期：至 `2026-09-29 10:48:04.150897 +08:00`。
- 订单 `757`：`COMPLETED`，退款金额 `0`，快照 weekly 206 / rc 4 / ivl 7。
- 订单 `757` 已有审计：`BALANCE_PACKAGE_INITIAL_CREDIT`（期1）、`BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2`（期2，id=2418）；无期 3 审计，满足幂等前置检查。
- 该用户存在 `2` 把未删除 API Key。
- 执行前普通余额为 `-0.10666215 USD`（微小欠费），套餐窗口剩余 `0`。

## 执行方式

执行经 SSH 在 VPS `sub2api-postgres` 内以 `docker exec -i ... psql` + stdin 运行。使用 PostgreSQL `SERIALIZABLE` 事务，锁用户/套餐/订单（`FOR UPDATE`），断言结构性稳定字段（每期额度 `206`、`credited_count=2`、`refresh_count=4`、`status=active`、`next_credit_at=2026-09-15 10:48:04.150897+08`、有效期未过、`remaining ∈ [0, weekly]`、订单 `COMPLETED`/退款 `0`、期 3 幂等审计不存在）。余额与剩余按锁内实时值即时计算，沿用生产 `creditDueBalance` 语义。

锁内实时值：`balance=-0.10666215`、`remaining=0` → `base=-0.10666215`，`new_remaining=205.89333785`，`balance_delta=206`，`new_balance=205.89333785`，`debt_repaid=0.10666215`。

## 执行结果

- 套餐 `190`：到账 `2/4` -> `3/4`，剩余 `0` -> `205.89333785 USD`，状态保持 `active`。
- `next_credit_at`：`2026-09-15 10:48:04.150897 +08:00` -> `2026-09-22 10:48:04.150897 +08:00`。
- 套餐有效期保持 `2026-09-29 10:48:04.150897 +08:00` 不变。
- 用户普通余额：`-0.10666215` -> 提交时 `205.89333785 USD`（本周额度先偿还 `0.10666215` 欠费，剩余进入套餐窗口；此后随实时计费变动）。
- `users.total_recharged`：`898.32000000` -> `1104.32000000 USD`（按本期额度 `+206`）。
- 订单 `757` 仍为 `COMPLETED`，退款金额保持 `0`。
- 新增支付审计 `payment_audit_logs.id=2469`：`BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_3`，`detail={"credit_usd":206,"credited_count":3}`，操作者 `admin:authorized_manual_settlement`，DB 时间 `2026-09-06 10:26:31+08`。
- 新增欠费还款流水 `balance_debt_ledger.id=71`：`repayment` `0.10666215 USD`，`balance_before=-0.10666215`，`balance_after=205.89333785`，`source_ref=package:190:credit:3`。

## 缓存与核验

- 事务内经 `enqueue_auth_cache_invalidation(key)` 为该用户 `2` 把在用 API Key 写入 `auth_cache_invalidation_outbox`（`cache_key=encode(sha256(key),'hex')`，不接触明文）。worker 两遍失效完成，回读 outbox 待处理数为 `0`。
- Redis `billing:balance:484`：执行前后均不存在（`DEL` 返回 `0`）。
- 两把 API Key 的 L2 鉴权缓存失效后当前不存在（`EXISTS=0`），下次鉴权按新余额重建。
- 套餐进度、剩余、下次刷新、有效期、订单状态、支付审计与欠费流水均从 VPS 生产库回读确认。
- VPS 应用/PostgreSQL/Redis 容器均 `healthy`；容器内 `http://127.0.0.1:8080/health` 返回 `HTTP/1.1 200 OK`。

## 备注

- 该用户本会话已连续发放期 2（`20260904-094105-*`）与期 3（本记录）。4 期中已发放 3 期，剩最后 1 期，`next_credit_at=2026-09-22`。
- 本次未触碰代码、容器或 ZPay 账户；仅对 VPS 生产数据库执行最小必要写入，并复用应用自身的缓存失效机制。
