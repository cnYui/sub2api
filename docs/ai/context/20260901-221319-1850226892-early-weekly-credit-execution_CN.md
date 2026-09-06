# 用户 1850226892@qq.com 提前刷新周额度记录

## 授权与范围

按管理员要求，提前刷新用户 `1850226892@qq.com`（`users.id=480`）当前余额套餐的下一周额度。

执行前发现该套餐对应订单存在**用户本人发起、因 ZPay 卖家余额不足而失败的退款**，与"给同一套餐加发额度"直接冲突。已就此向管理员确认，管理员选择**放弃该退款、保留套餐后再发放**。

本次仅处理当前有效余额套餐 `user_balance_packages.id=180` 及关联订单 `payment_orders.id=742`。未修改历史套餐（`13/75/84/115`）、订单金额、套餐有效期、历史用量、API Key 配额或流量卡。

## 执行前状态

生产数据库执行前核查：

- 用户普通余额：`-0.05634625 USD`（负，存在微小欠费）。
- `users.total_recharged`：`776.00000000 USD`。
- 当前套餐：`id=180`（订单 742），状态 `active`，已到账 `1/4` 期。
- 每期额度：`154 USD`；当前套餐窗口剩余：`0 USD`。
- 原定下一次刷新：`2026-09-04 16:56:28.902223 +08:00`。
- 套餐有效期：至 `2026-09-25 16:56:28.902223 +08:00`。
- 订单 `742`：**`REFUND_FAILED`**，`refund_amount=44.25`，`refund_requested_by=480`，`refund_requested_at=2026-09-01 21:53:41 +08`，失败原因 `easypay refund failed (HTTP 200): 卖家余额不足`。
- 订单 `742` 尚无 `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2` 审计，满足幂等前置检查。

## 执行方式

执行时间为 `2026-09-01 22:13 +08:00`。使用 PostgreSQL `SERIALIZABLE` 事务，事务内按主键重新锁定用户、当前套餐和关联订单（`FOR UPDATE`），并校验套餐状态、到账进度、每期额度、有效期、订单当前状态与订单套餐快照仍符合预期，同时校验幂等审计不存在。

事务内分两步：

### 第一步：关闭失败退款（放弃退款）

- 订单 `742`：`REFUND_FAILED` -> `COMPLETED`，`refund_amount` `44.25` -> `0`，清空 `refund_requested_at/refund_requested_by/refund_request_reason/refund_reason/refund_at/failed_at/failed_reason`。
- 新增审计 `payment_audit_logs.id=2369`：`REFUND_ABANDONED_MANUAL`，操作者 `admin:authorized_manual_settlement`，明细记录原状态与原退款金额及放弃原因。

### 第二步：提前发放一期（沿用生产 `creditDueBalance` 语义）

- 本期到账：`154 USD`。
- 执行前普通余额为负（`-0.05634625`），本周额度先偿还欠费 `0.05634625 USD`，剩余进入套餐窗口。
- `base = balance - remaining = -0.05634625`；`new_remaining = max(154 + base, 0) = 153.94365375`；`balance_delta = weekly - remaining = 154`。
- 套餐从 `1/4` 变为 `2/4`，保持 `active`。
- 下一次刷新按原计划时间加 `7` 天，从 `2026-09-04 16:56:28.902223 +08:00` 顺延至 `2026-09-11 16:56:28.902223 +08:00`。

## 执行结果

- 用户普通余额：`-0.05634625 USD` -> `153.94365375 USD`。
- `users.total_recharged`：`776.00000000 USD` -> `930.00000000 USD`。
- 套餐 `180`：到账 `1/4` -> `2/4`，剩余 `0` -> `153.94365375 USD`，状态保持 `active`。
- `next_credit_at`：`2026-09-04 16:56:28.902223 +08:00` -> `2026-09-11 16:56:28.902223 +08:00`。
- 套餐有效期保持 `2026-09-25 16:56:28.902223 +08:00` 不变。
- 订单 `742`：`REFUND_FAILED` -> `COMPLETED`，退款金额 `44.25` -> `0`。
- 新增欠费还款流水 `balance_debt_ledger.id=63`：`repayment` `0.05634625 USD`，`balance_before=-0.05634625`，`balance_after=153.94365375`，`source_type=balance_package_weekly_credit`，`source_ref=package:180:credit:2`。
- 新增支付审计 `payment_audit_logs.id=2370`：`BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2`，`detail={"credit_usd":154,"credited_count":2}`，操作者 `admin:authorized_manual_settlement`。

## 缓存与核验

- Redis 余额缓存 `billing:balance:480` 执行前后均不存在（下次请求按新余额重算）。
- 执行时 Redis 中 `apikey:auth:*` 认证缓存总数为 `0`，无旧余额快照残留。
- 仍为该用户 `8` 把未删除 API Key 写入认证缓存失效 outbox（`auth_cache_invalidation_outbox`，`cache_key=hex(sha256(key))`）；worker 已完成首遍失效（删 Redis + 发布 `auth:cache:invalidate` + 清本地 L1），并在安全延迟后完成第二遍复删，回读待处理数为 `0`。
- 用户余额、`total_recharged`、套餐进度、套餐剩余、下次刷新时间、订单状态、欠费流水与支付审计均已从生产数据库回读确认。
- 应用容器、PostgreSQL、Redis 均为 `healthy`。

## 备注

- 本次未触碰代码、容器或 ZPay 账户；仅对生产数据库执行上述最小必要写入。
- 因订单 `742` 的退款已被主动关闭（`44.25` 清零），若用户后续仍要求退款，需重新按当前套餐状态发起并重新核价。
