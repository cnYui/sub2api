<!-- prune:keep -->
# 管理端「提前发放余额套餐下一周额度」端点（固化手工 SQL）

- 时间：2026-09-08
- 分支：`feat/admin-early-weekly-credit`（基于 `main`，独立于 `feat/affiliate-rebate-7day-use-or-lose`）
- 背景：仓库里同类「周额度提前刷新」执行记录已 15+ 份，每次手写 `SERIALIZABLE` 事务，幂等键 / 锁顺序 / `creditDueBalance` 口径 / 欠费流水正数约束 / 缓存失效缺一不可。runbook 第 8 节明确「值得加一个管理端点固化」。本次实现之。

## 端点

`POST /api/v1/admin/payment/balance-packages/:id/credit-next`

- 鉴权：复用 `adminGroup` 既有中间件（adminAuth + auditLog + AdminComplianceGuard）。
- 幂等：handler 走 `executeAdminIdempotentJSON("admin.balance_packages.credit_next", {package_id}, ...)`（请求级去重）；service 内再查 `payment_audit_logs` 同期审计（DB 级幂等）。
- 入参：路径 `:id` = `user_balance_packages.id`。
- 返回：`EarlyWeeklyCreditResult`（package_id/user_id/order_id/credited_count/refresh_count/credit_usd/remaining_usd/balance_before_usd/balance_after_usd/debt_repaid_usd/status/next_credit_at/expires_at/completed）。

## 语义（与调度器 `creditDueBalance` 完全对齐）

核心方法 `BalancePackageService.CreditNextEarly(ctx, packageID, adminUserID, now)`：

1. 单个 `ent.Tx`；固定加锁顺序 **先用户后套餐**（`lockBalancePackageUser` + 套餐 `ForUpdate()`），与调度器一致，避免与定时到账死锁。
2. 前置校验：套餐 `status=active`、`credited_count < refresh_count`、`expires_at > now`、`next_credit_at != nil`。
3. 幂等：`payment_audit_logs` 不存在 `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_<newCount>` 或 `BALANCE_PACKAGE_WEEKLY_CREDIT_<newCount>`（同一订单）。
4. **金额一律用锁内实时值算**（与坑：活跃用户余额随时变）：
   - `base = balance - remaining`
   - `newRemaining = base>=0 ? weekly : max(weekly+base,0)`
   - `balanceDelta = weekly - remaining`（`AddBalance`）
   - `debtRepaid = min(max(-base,0), weekly)`
5. 与调度器唯一的差别：
   - **不检查 `next_credit_at <= now`**（这正是「提前」）；
   - **每次只发一期**（`newCount = credited+1`，非 `periodsDue`）；
   - `next_credit_at` = 原值 + 一个 `refresh_interval_days`，保持定时节奏；末期（`newCount >= refresh_count`）置 `status=completed` 且清空 `next_credit_at`。
6. 写：套餐（带 `credited_count`/`status`/`expires_at` 乐观 `Where` 守卫，不匹配→`NotFound`→返回 Conflict）、用户（`AddBalance`/`AddTotalRecharged`）、欠费流水（`recordBalanceDebtLedger`，`base<0` 才写、Postgres only）、审计（action `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_<n>`、operator `admin:<id>`、detail 含 before/after/debt）。
7. 提交后缓存失效：`invalidateBalanceCache`（余额缓存）+ `authCacheInvalidator.InvalidateAuthCacheByUserID`（认证快照，清旧负余额快照避免继续拦截刚充值的用户）。与 `admin_user.UpdateUserBalance` 同口径；调度器批量到账不接认证失效是有意的（走 TTL 最终一致）。

## 接线

- `BalancePackageService` 新增 `authCacheInvalidator APIKeyAuthCacheInvalidator` 字段 + `SetAuthCacheInvalidator`。
- `ProvideBalancePackageService(entClient, billingCache, authCacheInvalidator)` 增第三参；wire 由既有 `ProvideAPIKeyAuthCacheInvalidator` 提供，`APIKeyService` 不依赖 BalancePackage/Payment，**无循环依赖**。
- `wire_gen.go` 手工同步该调用（`apiKeyAuthCacheInvalidator` 在该行上文已就绪，与 `wire` 会生成的结果一致）。
- `PaymentService.CreditNextEarlyBalancePackage` 薄封装委托。

## 改动文件

- `backend/internal/service/balance_package_service.go`：常量、字段、setter、`EarlyWeeklyCreditResult`、`CreditNextEarly`。
- `backend/internal/service/payment_service.go`：委托方法。
- `backend/internal/service/wire.go`：`ProvideBalancePackageService` 增参。
- `backend/cmd/server/wire_gen.go`：同步生成调用。
- `backend/internal/handler/admin/payment_handler.go`：`CreditNextEarlyBalancePackage`。
- `backend/internal/server/routes/payment.go`：注册路由。
- `backend/internal/service/balance_package_lifecycle_test.go`：两个定向测试（提前+欠费抵扣、末期完成+完成后拒绝）。

## 验证

- `go build ./...` 通过；`go vet` 改动包通过。
- `go test ./internal/service/ -run 'BalancePackage|CreditDue|CreditNextEarly|CreditInitial'` 通过。
- 注意坑 19：`internal/service` 的 `unit` tag 全量套件不编译，仅跑定向用例。

## 未做 / 后续

- **未出镜像、未部署**：生产 VPS 跑 GHCR 镜像，端点上线需构建新镜像并改 `IMAGE_TAG` 部署后才生效。
- 前端管理页暂无按钮，可后续在余额套餐/订单管理加「提前发放下一周」入口调用本端点；当前可用 admin `x-api-key` 直调。
- 可选：给端点加「一次提前 N 期」参数（当前一次一期，多期需多次调用）。
