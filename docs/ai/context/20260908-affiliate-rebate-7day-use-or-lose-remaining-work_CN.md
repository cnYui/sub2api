<!-- prune:keep -->
# 邀请返利 7 天 use-or-lose —— 剩余工作 / TODO（后台清除任务为主）

- 时间：2026-09-08
- 分支：`feat/affiliate-rebate-7day-use-or-lose`（WIP 已提交 `0633e05cd`）
- 需求（管理员）：邀请返利冻结期 **7 天**；**7 天内不使用就从余额清零**；**每一笔返利单独计时**；扣费**返利优先**。
- 设计总述见同目录 `20260907-affiliate-rebate-7day-use-or-lose-design_CN.md`。本文件只列**还没做的**。

## 已完成（上半部：消费侧，已 `go build` 通过，生产 inert）

`affiliate_rebate_freeze_hours` 生产当前 = 0，所以下述代码在改成 168 之前**全程零操作、不动任何余额**。这是关键安全阀。

1. 迁移 `214_affiliate_rebate_use_or_lose.sql`：`user_affiliate_ledger` 加 `remaining_amount DECIMAL(20,8) NOT NULL DEFAULT 0` + 部分索引；**不回填存量**（存量恒 0、永不被清零，use-or-lose 只对上线后新返利生效）。
2. `domain_constants.go`：`AffiliateRebateFreezeHoursDefault` 24→168。
3. `affiliate_repo.go` `AccrueQuota`（freezeHours>0 分支）：accrue 行写 `remaining_amount = amount`。
4. `usage_billing_repo.go` 扣费热路径改为 **rebate-first**：`deductUsageBillingBalanceIfSufficientWithLedger` 与 `deductUsageBillingBalanceWithLedger`（两个分支）用 `charged/aff_before/rebate` CTE 先扣返利（`aff_frozen_quota -= LEAST(charge, frozen)`），返回 `rebate_used`；再由 helper `consumeRebateLedgerFIFO` 按 `frozen_until` 升序 FIFO 递减各 accrue 行的 `remaining_amount`。仅 `rebate_used>0` 时才跑 FIFO（热路径守卫）。串行化靠已有的 `users` 行锁。

**不变量**：`aff_frozen_quota` == 所有 accrue 行 `remaining_amount` 之和 == 余额里"仍未用、仍未过期的返利"。

## 还没做（下半部：到期清除 + 收尾）

### 1. 【核心，管理员明确要求】后台任务：清除 7 天未用返利

目前没有任何通用后台任务基础设施（`ThawFrozenQuota` 是 lazy on-read）。需要新建。

- **做什么**：周期性找出 `action='accrue' AND frozen_until <= NOW() AND remaining_amount > 0` 的行，**按用户**在一个事务里：
  1. `SELECT ... FROM users WHERE id=$u FOR UPDATE`（**必须先锁 users 行**，与扣费热路径同一把锁、同一加锁顺序，避免死锁/竞态）；
  2. 求该用户这些过期行的 `SUM(remaining_amount)` = `clawed`；
  3. `UPDATE users SET balance = balance - clawed`（从余额清回）；
  4. `UPDATE user_affiliates SET aff_frozen_quota = GREATEST(aff_frozen_quota - clawed, 0)`；
  5. 把这些行 `remaining_amount = 0`（并可加 `frozen_until = NULL` 标记已处理）；
  6. 写审计：`payment_audit_logs`（operator 建议 `system:affiliate_rebate_expire`，order_id 用占位或空）+ 可选 `user_affiliate_ledger` 一条 `action='expire'` 明细。
- **放哪 / 怎么触发**：
  - 新增 repo 方法 `ExpireUnusedAffiliateRebate(ctx) (usersAffected int, err error)`（一次处理一批用户，比如 100 个，`SELECT DISTINCT user_id ... LIMIT 100` 再逐个上面事务；返回处理数，>0 就继续下一批直到 0）。
  - 新增 service 包装（可挂在 `AffiliateService` 上）。
  - 在 `cmd/server/main.go`（`initializeApplication` 之后、`app.Server.ListenAndServe` 那批 `go func()` 附近，约 150–170 行）起一个 `time.NewTicker`（建议 10–15 分钟）的 goroutine，退出时 `ticker.Stop()`；`app` 需能拿到该 service/repo——检查 `Application` 结构体（`wire_gen.go`/`wire.go`）是否已暴露，没有就补一条装配（DI wiring 是这块最侵入的部分）。
  - **幂等**：`remaining_amount>0` 过滤天然幂等；批处理避免长事务。
  - **频率与"恰好 7 天"**：ticker 周期就是清除延迟上界（如 15 分钟即"最迟超期 15 分钟被清"）。业务上可接受即可。
- **并发正确性**：清除任务与扣费都改同一用户的 `balance`/`aff_frozen_quota`/ledger，两边都**先锁 `users` 行**即可串行、无死锁。
- **可选**：同时保留一个 lazy 清除（读 affiliate 汇总时对该用户跑一次），让活跃用户即时看到；但**不可只靠 lazy**（不活跃用户不会被清）。

### 2. 处理旧的 "mature-and-keep" thaw（否则与清除任务冲突）

`affiliate_repo.go` 的 `thawFrozenQuotaTx` / `ThawFrozenQuota` 现在是"到期成熟保留"（清 `frozen_until`、减 `aff_frozen_quota`、**钱留着**）。use-or-lose 下这是错的：它会把行 `frozen_until` 置 NULL，让清除任务再也找不到 → 永不清零。**必须处理**：
- 方案 A（推荐）：把 `ThawFrozenQuota` 语义改成"到期清除"（即上面的清除逻辑），lazy 调用点（`affiliate_service.go:246`）与后台任务共用它；名字建议改成 `ExpireUnusedRebate` 更贴切（改接口 + 1 处真实调用 + 2 处 test stub：`auth_email_oauth_test.go:402`、`payment_fulfillment_test.go:98`，都是 panic stub，改名即可）。
- 方案 B：删除 thaw 及其 lazy 调用，只靠后台任务。
- **别两者并存**（一个保留一个清零，会打架）。

### 3. affiliate 汇总展示语义

`affiliate_repo.go` `affiliateUserOverviewSQL`（约 44–49 行）里 `matured_frozen_quota = SUM(amount) WHERE frozen_until <= NOW()` 原意是"已成熟可用返利"。use-or-lose 下 `frozen_until<=now` 的行是**要被清零的**，不是"可用"。需改：展示"可用返利"应基于 `remaining_amount`（未过期、未用尽），过期的不再计入 `aff_quota`。核对 `/affiliate` 页面字段含义别误导用户。

### 4. 测试

- 单测（sqlmock，`usage_billing_repo_unit_test.go`）：rebate-first 消费 SQL 断言已随上半部改动**需要同步更新**（现有 `conditionalBalanceDeductSQL`/`overdraftBalanceDeductSQL` 正则是 own-first 的旧 CTE，新 CTE 变成 `charged/aff_before/rebate`，且多返回一列 `rebate_used`——**这三个 Apply 测试现在肯定挂**，务必先本地 `go test -tags=unit ./internal/repository/` 修正）。补 `consumeRebateLedgerFIFO` 的断言。
- 集成测试（真 PG，需 Docker）：发放(freeze>0)→部分消费(验证返利优先、FIFO、remaining 递减)→到期清除(验证 balance/aff_frozen_quota 正确扣回、审计写入)。**本机没 Docker，只能靠 CI 跑**（`internal/repository` 集成测试无 Docker 会静默跳过、`os.Exit(0)`，见 [[sub2api-fork-ci-cd-jobs-state]]）。

### 5. 激活（上线后单独一步，别和代码一起）

代码合并后仍 inert。确认 CI 集成测试绿、人工复核无误后，再把生产 `settings.affiliate_rebate_freeze_hours` 由 `0` 改 `168`（走 admin API / DB，写审计）。**这是唯一让它真正开始动用户余额的开关**——先上代码、验证、再开。

## 坑清单

- 迁移一旦应用不可改（当前最大 214）。
- 动的是每请求计费热路径 + 会从用户余额扣钱：事务 + `users` 行锁 + 并发安全；扣费金额本身不变，只改"返利这本账"。
- sqlmock 精确断言 SQL：改扣费 SQL 必须本地重跑 `-tags=unit`（CI 任务上次在此栽过）。
- 集成测试要 Docker，本机跑不了，靠 CI（上次就是靠 CI 才逮到 bug）。
- 保持不变量 `aff_frozen_quota == Σ remaining_amount`：消费、清除、发放三处都要同步两者。
