# 余额套餐续费周期重置实现

## 背景与问题

续费同档余额套餐后，套餐周期没有重置：前端仍显示旧的 `4/4 已完成`、`本周剩余 0`、"刷新已完成"，用户付费后看不到任何新到账。根因在后端 `renewBalancePackage`（`backend/internal/service/balance_package_service.go`）：续费**只延长 `expires_at` 28 天并改写 `payment_order_id`**，既不发放新周期额度，也不重置 `credited_count` / `next_credit_at` / `status`。这是数据/逻辑缺口，不是前端渲染问题——前端直接读这些字段渲染，只要后端正确重置，卡片即自动显示新周期。此前订单 `750` 靠人工事务补发（见 `20260831-092705-859591608-renewal-refresh-execution_CN.md`）。

## 决策

- 续费语义：**重置为新一轮 4 期周额度**（立即发放第 1 期、进度回到 `1/4`、`next_credit_at` 重新计时 +7 天、有效期在原到期基础上 +28 天）。若旧周期尚未走完，把未发放的期数顺延进新周期总期数，保证已付费额度不丢（正常"完成后续费"时顺延为 0，仍是 `1/4`）。不采用"叠加期数"，因为会与"按订单退款"的金额口径冲突。
- 前端：自动重置 + 续费标识（`续费×N` 小标签 + "有效期已延长至 …"一行），让用户确认续费生效。

## 后端改动

- 迁移 `212_balance_package_renewal_count.sql`：`user_balance_packages` 新增 `renewal_count INTEGER NOT NULL DEFAULT 0`（含 `>=0` 约束），记录同档续费次数，仅用于展示与统计，不参与计费或退款金额。
- ent schema `ent/schema/user_balance_package.go` 增加 `renewal_count` 字段并重新生成 ent 代码（只回写 `userbalancepackage*`、`mutation.go`、`migrate/schema.go`、`runtime.go`，未夹带无关的 channelmonitor 注释漂移）。
- `renewBalancePackage` 重写为周期重置：
  - 余额口径与周额度刷新一致——先移除旧窗口 `remaining_usd`，再用本周额度抵扣负余额，剩余进入新窗口（`baseBalance = user.balance - old.remaining_usd`）。
  - `credited_count=1`；`refresh_count = 订单期数 + max(old.refresh_count - old.credited_count, 0)`（顺延未发放期数）；`starts_at=now`；`next_credit_at=now+interval`（若期数为 1 则 `completed` 并清空）；`status=active`；`expires_at=old.expires_at + validity_days`；`renewal_count += 1`。
  - `users.total_recharged` 计入本周额度；负余额被抵扣时写 `balance_debt_ledger` 的 `balance_package_renewal_credit` 还款流水。
  - 审计：`BALANCE_PACKAGE_RENEWAL`（新增 `weekly_credit_usd`/`refresh_count`/`renewal_count`/`carried_periods`/`cycle_reset` 明细）+ 首期 `BALANCE_PACKAGE_INITIAL_CREDIT`。
- 退款报价 `payment_balance_package_refund.go`：账本用量求和限定 `created_at >= starts_at`。续费复用同一套餐行并重置 `starts_at`，账本按套餐 ID 跨周期累计，若不限定会把上一周期（属于已被覆盖的旧订单）的用量算进本次续费订单而少退款。
- `UserBalancePackageView` 增加 `renewal_count` 并在 `ListUserPackages` 回填。

## 前端改动

- `types/payment.ts` 的 `UserBalancePackage` 增加 `renewal_count`。
- `SubscriptionsView.vue`：状态旁新增 `已续费 ×N` 标签，明细区新增"续费已重置周期，有效期延长至 {date}"一行（均在 `renewal_count > 0` 时显示）。
- i18n：中英新增 `renewedBadge`、`renewalExtended`。

## 边界

- 未改动生产数据、余额、订单或部署；本轮仅代码与迁移文件。迁移 `212` 上线后新续费才走重置逻辑；历史已被旧逻辑仅延长有效期的套餐仍需按需人工刷新。
- 未触碰在飞的 `usage_billing_repo.go` 与迁移 `211`；退款文件仅在既有改动上追加周期窗口限定。

## 验证

- ent 在临时 git worktree（AppData 临时目录，规避本机 Defender/Codex 对项目目录写文件的 mmap 锁）中生成，仅回写变更文件。
- `go build ./...` 通过；`gofmt -l`、`go vet ./internal/service/` 无输出。
- `go test ./internal/service ./internal/repository -count=1` 通过。新增/重写 `TestRenewBalancePackageResetsCycleAfterCompletion`（完成后续费重置为 `1/4`、余额与 `total_recharged` 正确、`renewal_count=1`、审计存在、仍单行）与 `TestRenewBalancePackageCarriesUndeliveredPeriodsMidCycle`（`2/4` 中途续费顺延为 `1/6`）。
- 前端 `npm run typecheck` 通过；`SubscriptionsView.spec.ts` 5 项通过（含续费标识展示与未续费不展示两条新用例）。
