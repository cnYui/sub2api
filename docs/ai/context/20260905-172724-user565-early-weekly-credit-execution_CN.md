# users.id=565 第 2 期周额度提前刷新执行记录

- 执行时间：`2026-09-05 16:26:37 +08`（生产数据库事务时间）
- 管理员要求：把 `users.id=565` 下一周的额度提前刷新（邮箱见生产库，本文档不记录）
- 范围：仅 `users.id=565`、套餐 `user_balance_packages.id=121`、订单 `payment_orders.id=763`

---

## 1. 前置：一次判断失误

最初结论是「没有 SSH 访问，此操作做不了」。**这个结论是错的。**

原因：用 `ls ~/.ssh/*.pem ~/.ssh/id_*` 过滤私钥，而实际的部署密钥**按机器命名**
（见 `deploy/ops.env` 的 `OPS_SSH_KEY_FILE`），不匹配 `id_*` 也不是 `.pem`，被漏掉。
默认密钥 `id_ed25519` 确实被服务器拒绝（`Permission denied (publickey)`），
于是误判为「无访问权限」。

`ls ~/.ssh/` 全量看一遍就能发现。**过滤器写窄了，比没查更容易得出确定的错误结论。**

该误判不必要地阻塞了：本次操作、生产价格目录核查（后来绕道模型广场解决）、以及部署。

---

## 2. 为什么必须走数据库

管理接口**没有**「提前刷新周额度」端点。全仓 606 个路由中，`balance-packages`
管理组只有三个：

```
GET  /admin/payment/balance-packages              商品目录（10 个 SKU）
POST /admin/payment/balance-packages/grant        新建套餐
POST /admin/payment/balance-packages/:id/resume-debt-paused
```

前端也无对应入口（搜「提前刷新」/`earlyCredit` 均无命中）。

**`grant` 不能替代**：它创建全新的 `admin_grant` 订单与套餐，会违反「每用户最多一个
有效套餐」，且管理员发放套餐不可退。

**`POST /admin/users/:id/balance` 也不能替代**：它只改余额数字，不碰
`user_balance_packages`。用它会留下 `remaining_usd` 不同步、`credited_count` 不递增、
**`next_credit_at` 不推进导致定时任务到原日期重复发放**，且不写 `payment_audit_logs`
（违反项目硬规则）。比不做更糟。

---

## 3. 执行前状态（生产回读）

| 项 | 值 |
| --- | --- |
| 用户 `565` | `balance=3.07061143`、`total_recharged=2770`、`active` |
| 套餐 `121` | `active`、`1/4`、周额度 `520`、`remaining_usd=3.07061143` |
| | `starts_at=2026-09-02 22:26:42`、`next_credit_at=2026-09-09 22:26:42` |
| | `expires_at=2026-09-30 22:26:42`、`renewal_count=0` |
| 订单 `763` | `COMPLETED`、`refund_amount=0` |
| 幂等 | 订单 `763` 已有 `..._CREDIT_1`（9 月 2 日），**无 `..._CREDIT_2`** |
| API Key | 2 把未删除 |

---

## 4. 执行方式

单个 `SERIALIZABLE` 事务，`FOR UPDATE` 按「用户 → 套餐 → 订单」顺序锁定，
逐项校验后计算并写入；任一前置不符即 `RAISE EXCEPTION` 回滚。

**金额按锁内实时值计算，不用事务外读到的快照**——该用户 16:03 仍在活跃，
余额随时可能变动。沿用生产 `creditDueBalance` 语义：

```
base          = balance - remaining
new_remaining = base >= 0 ? weekly : max(weekly + base, 0)
new_balance   = base + weekly
debt_repaid   = min(max(-base, 0), weekly)
```

锁内实时值：`balance=3.07061143`、`remaining=3.07061143` → `base=0`，
`new_remaining=520`、`new_balance=520`、`debt_repaid=0`。

`next_credit_at` 取 `原值 + refresh_interval_days`（而非 `now() + 7 天`），
保持每周 `22:26:42` 的到账节奏，与 `20260904-094105` 同类执行一致。
`starts_at` 与 `expires_at` **不动**。

---

## 5. 执行结果

| 对象 | 变更 |
| --- | --- |
| 套餐 `121` | `credited_count` `1 -> 2`；`remaining_usd` `3.07061143 -> 520.00000000` |
| | `next_credit_at` `2026-09-09 22:26:42 -> 2026-09-16 22:26:42` |
| | `starts_at`、`expires_at`、`status` **未变** |
| 用户 `565` | `balance` `3.07061143 -> 520.00000000` |
| | `total_recharged` `2770 -> 3290.00000000`（+520） |
| 审计 `payment_audit_logs.id=2447` | `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2`，操作者 `admin:authorized_manual_settlement` |
| | `detail={"credit_usd":520,"credited_count":2,"early":true,"original_next_credit_at":"2026-09-09T22:26:42.153829+08:00"}` |
| 欠费账本 | **未写入** |

**未写账本是正确的**：`base=0` 无欠费可偿还，而 `balance_debt_ledger` 有
`CHECK (amount_usd > 0)` 约束，插零额度行会直接失败。事务里用 `IF v_debt > 0` 跳过。

订单 `763` 状态与退款金额未改动；未触碰其它套餐（`47/70/80`，均已 `refunded`）、
历史用量、API Key 配额或流量卡。

---

## 6. 缓存与核验

- 事务内经 `enqueue_auth_cache_invalidation(k.key)` 为该用户 2 把在用 API Key 写入
  `auth_cache_invalidation_outbox`。**Key 明文经子查询传递，全程未输出、未落盘。**
- 回读 outbox 待处理数为 `0`，worker 已完成处理。
- Redis 余额缓存 `billing:balance:565` 执行前后均不存在（`EXISTS=0`），
  下次请求按新余额重算。
- 套餐进度、剩余、下次刷新、起始、有效期、状态，用户余额与 `total_recharged`，
  订单状态，支付审计，均已从生产数据库回读确认。

---

## 7. 值得固化的两点

**① 这类操作已有 9 份以上执行记录，每次都要人工写 SQL 事务。**
幂等键、锁顺序、`creditDueBalance` 口径、欠费账本的正数约束、缓存失效——
每一环漏掉都会出问题。值得加一个管理端点把它固化进代码。

**② `next_credit_at` 必须推进。** 这是防重复发放的唯一屏障：
定时任务按该字段判定是否到期发放，不推进就会在原定日期再发一次。
任何「只加余额」的变通做法都会踩这个坑。
