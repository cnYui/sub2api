# 私下付款历史订单补录实施计划

> **供代理执行：** 必须使用 `superpowers:subagent-driven-development`（推荐）或 `superpowers:executing-plans` 逐项执行。所有步骤使用复选框跟踪。

**目标：** 为 5 笔已人工续期的 29 元订阅补录不可退款的 `offline` 历史订单，并在用户与管理员订单界面显示“私下付款”，同时把历史 `manual_grant` 显示为“赠送金额”。

**架构：** `offline` 是内部订单支付类型，不是 Provider，也不会进入用户结账。受限命令仅包含确认过的 5 条事实，通过 PostgreSQL `SERIALIZABLE` 事务校验并写入已完成订单及审计记录，绝不调用普通付款履约、返利、余额或流量卡链路。

**技术栈：** Go 1.26、PostgreSQL、Ent、Gin、Vue 3、TypeScript、Vitest、Testcontainers PostgreSQL。

---

## 文件结构

- 修改：`backend/internal/payment/types.go`，定义内部 `offline` 支付类型。
- 修改：`backend/internal/service/payment_refund.go`，在用户、管理员和绕过入口处统一拒绝 `offline` 自动退款。
- 新增：`backend/internal/service/offline_payment_backfill.go`，封装固定批次、schema 预检、事务校验、幂等写入与审计。
- 新增：`backend/cmd/offline-payment-backfill/main.go`，提供默认 dry-run、显式确认才写入的单次二进制。
- 修改：`Dockerfile`，在运行镜像中提供受限补录二进制。
- 修改：`frontend/src/i18n/locales/zh.ts`、`frontend/src/i18n/locales/en.ts`，定义 `offline` 与 `manual_grant` 展示文案。
- 修改：`frontend/src/views/user/paymentRefund.ts`、`frontend/src/components/payment/orderUtils.ts`、`frontend/src/views/admin/orders/AdminOrdersView.vue`、`frontend/src/components/admin/payment/AdminOrderTable.vue`、`frontend/src/components/admin/payment/AdminOrderDetail.vue`，隐藏并阻止 `offline` 自动退款。
- 修改：`frontend/src/views/admin/orders/AdminPaymentDashboardView.vue`，给 `offline` 支付方式分布固定中性色。
- 测试：`backend/internal/payment/types_test.go`、`backend/internal/service/payment_order_result_test.go`、`backend/internal/service/payment_refund_test.go`、`backend/internal/service/payment_stats_test.go`、`backend/internal/repository/offline_payment_backfill_integration_test.go`、`backend/cmd/offline-payment-backfill/main_test.go`，以及相关 Vitest 文件。

### 任务 1：建立 `offline` 类型与退款防线

**文件：**
- 修改：`backend/internal/payment/types.go`
- 修改：`backend/internal/payment/types_test.go`
- 修改：`backend/internal/service/payment_order_result_test.go`
- 修改：`backend/internal/service/payment_refund.go`
- 修改：`backend/internal/service/payment_refund_test.go`
- 新增：`backend/internal/service/payment_stats_test.go`

- [ ] **步骤 1：先写支付类型与结账边界的失败测试。**

```go
func TestTypeOfflineKeepsItsInternalIdentifier(t *testing.T) {
	require.Equal(t, "offline", payment.TypeOffline)
	require.Equal(t, payment.TypeOffline, payment.GetBasePaymentType(payment.TypeOffline))
}

func TestValidateUserExternalPaymentTypeRejectsOffline(t *testing.T) {
	require.Equal(t, "PAYMENT_METHOD_NOT_AVAILABLE", infraerrors.Reason(validateUserExternalPaymentType(payment.TypeOffline)))
}
```

第一条放在 `types_test.go`，第二条放在同包的 `payment_order_result_test.go`，避免跨 package 调用未导出的 checkout 校验函数。

- [ ] **步骤 2：运行目标测试并确认尚未定义 `TypeOffline`。**

运行：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 ./internal/payment ./internal/service -run 'TestTypeOfflineKeepsItsInternalIdentifier|TestValidateUserExternalPaymentTypeRejectsOffline'
```

预期：编译失败，指出 `payment.TypeOffline` 未定义。

- [ ] **步骤 3：在支付常量中增加内部类型，不改 Provider 或用户结账白名单。**

在 `backend/internal/payment/types.go` 的支付类型常量中加入：

```go
TypeOffline PaymentType = "offline"
```

保留 `validateUserExternalPaymentType()` 仅接受 `alipay` 的现有逻辑；不要把 `offline` 放进 registry、factory、`SettingsView`、Provider 支持类型或 `CreateOrder` 路径。

同时在 `payment_stats_test.go` 添加：

```go
func TestBuildMethodDistributionIncludesOfflinePayment(t *testing.T) {
	methods := buildMethodDistribution([]*dbent.PaymentOrder{
		{PaymentType: payment.TypeOffline, PayAmount: 29},
		{PaymentType: payment.TypeOffline, PayAmount: 29},
	})
	require.Equal(t, []PaymentMethodStat{{Type: payment.TypeOffline, Amount: 58, Count: 2}}, methods)
}
```

这条测试证明私下付款进入既有收入/支付方式统计，而不需要为统计系统引入专门分支。

- [ ] **步骤 4：写三条退款拒绝的失败测试。**

在 `payment_refund_test.go` 构造 `payment_type=offline`、`order_type=subscription`、`status=COMPLETED` 的订单，分别断言：

```go
require.Equal(t, "OFFLINE_PAYMENT_MANUAL_REFUND_ONLY", infraerrors.Reason(err))
require.Equal(t, service.OrderStatusCompleted, reloaded.Status)
require.Equal(t, originalExpiry, reloadedSubscription.ExpiresAt)
require.Empty(t, provider.RefundCalls)
```

覆盖 `RequestRefund()`、`PrepareRefund()`、以及直接构造 `RefundPlan` 调用 `ExecuteRefund()` 三个入口。

- [ ] **步骤 5：在三条后端退款入口显式拒绝 `offline`。**

在 `payment_refund.go` 增加统一纯守卫：

```go
func rejectAutomaticOfflineRefund(order *dbent.PaymentOrder) error {
	if order != nil && order.PaymentType == payment.TypeOffline {
		return infraerrors.Forbidden(
			"OFFLINE_PAYMENT_MANUAL_REFUND_ONLY",
			"offline payment refunds require manual financial handling",
		)
	}
	return nil
}
```

在 `validateUserAutoRefundRequest()`、`PrepareRefund()`、`ExecuteRefund()` 取得订单后、触碰订阅和 Provider 前调用它。三个入口返回同一稳定 reason，避免管理员接口依赖“找不到 Provider”才失败。

- [ ] **步骤 6：运行后端目标测试并提交这一独立变更。**

运行：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 ./internal/payment ./internal/service -run 'TestTypeOfflineKeepsItsInternalIdentifier|TestValidateUserExternalPaymentTypeRejectsOffline|Test.*Offline.*Refund'
```

预期：全部通过；用户 checkout 仍拒绝 `offline`，三条退款入口均不修改订单、订阅或余额。

提交：

```bash
git add backend/internal/payment/types.go backend/internal/payment/types_test.go backend/internal/service/payment_order_result_test.go backend/internal/service/payment_refund.go backend/internal/service/payment_refund_test.go backend/internal/service/payment_stats_test.go
git commit -m "feat: add offline payment refund guard"
```

### 任务 2：以 TDD 实现固定批次的补录核心

**文件：**
- 新增：`backend/internal/service/offline_payment_backfill.go`
- 新增：`backend/internal/repository/offline_payment_backfill_integration_test.go`

- [ ] **步骤 1：在集成测试中定义可注入批次并写首次执行的失败用例。**

使用现有 repository Testcontainers harness。生产命令只能调用 `RunOfflinePaymentBackfill()`，它固定调用 `defaultOfflinePaymentBackfillBatch()`；同文件中的 `RunOfflinePaymentBackfillBatch()` 仅作为 Go 内部测试入口接收临时批次，绝不注册 HTTP 路由、CLI 参数或配置项。这样测试可使用隔离 ID，生产仍不能接受任意用户、金额、套餐或日期。测试应断言：

```go
result, err := service.RunOfflinePaymentBackfillBatch(ctx, integrationDB, testBatch, operator, true)
require.NoError(t, err)
require.Equal(t, 5, result.Created)
require.Equal(t, 0, result.Existing)

require.Equal(t, "offline", order.PaymentType)
require.Equal(t, 29.00, order.Amount)
require.Equal(t, 29.00, order.PayAmount)
require.Equal(t, 0.0, order.FeeRate)
require.Equal(t, "offline", order.FundingMode)
require.Nil(t, order.ProviderInstanceID)
require.Nil(t, order.ProviderKey)
require.Equal(t, service.OrderStatusCompleted, order.Status)
require.Equal(t, expectedPaidAt, *order.PaidAt)
require.Equal(t, expectedPaidAt, *order.CompletedAt)
require.Equal(t, expectedPaidAt, order.CreatedAt)
```

同一测试还应断言每条订单有一条 `OFFLINE_PAYMENT_RECORDED` 审计、总实付为 `145.00`、订阅到期时间与用户余额未变、没有 `payment_balance_holds`、流量卡账本或 affiliate 账本新增行。

- [ ] **步骤 2：运行集成测试并确认核心函数尚不存在。**

运行：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run '^TestOfflinePaymentBackfill'
```

预期：编译失败，指出 `service.RunOfflinePaymentBackfill` 未定义。

- [ ] **步骤 3：实现固定事实、schema 预检与 dry-run。**

在 `offline_payment_backfill.go` 定义 `defaultOfflinePaymentBackfillBatch()`，其中批次事实不可由命令参数覆盖：

```go
const offlinePaymentBackfillSource = "offline_paid_backfill_20260716"

var offlinePaymentBackfillEntries = []offlinePaymentBackfillEntry{
	{SubscriptionID: 2, UserID: 3, PaidAt: mustShanghaiTime("2026-07-16T12:08:33.371+08:00"), ExpectedExpiry: mustShanghaiTime("2026-08-16T00:00:00+08:00")},
	{SubscriptionID: 4, UserID: 6, PaidAt: mustShanghaiTime("2026-07-16T12:06:25.442+08:00"), ExpectedExpiry: mustShanghaiTime("2026-08-16T00:00:00+08:00")},
	{SubscriptionID: 7, UserID: 12, PaidAt: mustShanghaiTime("2026-07-16T12:05:16.893+08:00"), ExpectedExpiry: mustShanghaiTime("2026-08-16T00:00:00+08:00")},
	{SubscriptionID: 9, UserID: 15, PaidAt: mustShanghaiTime("2026-07-16T11:49:52.625+08:00"), ExpectedExpiry: mustShanghaiTime("2026-10-15T00:00:00+08:00")},
	{SubscriptionID: 13, UserID: 21, PaidAt: mustShanghaiTime("2026-07-16T13:30:29.288+08:00"), ExpectedExpiry: mustShanghaiTime("2026-10-15T00:00:00+08:00")},
}
```

五个确定性订单号依次为 `offline_paid_backfill_20260716_s2`、`offline_paid_backfill_20260716_s4`、`offline_paid_backfill_20260716_s7`、`offline_paid_backfill_20260716_s9`、`offline_paid_backfill_20260716_s13`。先读取 `schema_migrations` 和 `information_schema.columns`，要求 `162_refund_state_machine.sql`、`163_alipay_balance_hybrid_payment.sql` 已应用，且 `payment_orders` 包含 `subscription_id`、`funding_mode`、`balance_amount`、`gateway_amount` 和退款状态列；不满足时返回 `OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY`，不启动写事务。dry-run 必须执行全部只读前置检查并返回计划创建的 5 条订单，但总是回滚。

- [ ] **步骤 4：实现单事务校验、幂等和审计写入。**

以 `sql.LevelSerializable` 开始事务，先执行：

```sql
SELECT pg_advisory_xact_lock(hashtext('offline_paid_backfill_20260716'));
LOCK TABLE payment_orders IN SHARE ROW EXCLUSIVE MODE;
```

随后对五条订阅及其用户使用 `FOR UPDATE`，校验 `(subscription_id,user_id)`、`group_id=2`、`status='active'`、设计中精确到期时间、计划 `id=1/group_id=2/price=29.00/validity_days=30`，以及不存在其他 `subscription` 订单。所有确定性订单号不存在时才插入；五条均已存在且所有不可变字段和审计均匹配时返回幂等 no-op；出现部分存在、错误支付类型、金额、用户、订阅或审计时整批报错回滚。

每条 INSERT 必须明确写入：

```text
payment_type=offline; order_type=subscription; plan_id=1; subscription_group_id=2;
subscription_days=30; subscription_id=当前批次条目的固定订阅 ID; amount=pay_amount=29.00; fee_rate=0;
funding_mode=offline; balance_amount=gateway_amount=0; status=COMPLETED;
payment_trade_no=''; provider_instance_id=NULL; provider_key=NULL;
paid_at=completed_at=created_at=updated_at=当前批次条目的固定历史续期时间; expires_at=同一历史续期时间。
```

`created_at` 使用历史付款时间，因为用户“我的订单”表格显示该字段；实际补录执行时间只写入审计日志。每条订单同事务插入 `OFFLINE_PAYMENT_RECORDED`，detail 为 JSON，固定包含 `source`、`subscription_id`、`user_id`、`paid_at`、`amount=29.00`、`currency=CNY`、`refund_policy=manual_only`，且 `operator` 必须非空。

- [ ] **步骤 5：补齐失败关闭与幂等测试，运行并提交。**

覆盖重跑 no-op、部分已有订单、订阅/用户/分组/状态/到期时间/套餐不符、缺迁移或列、空 operator 五种场景；每个失败场景断言零订单和零审计写入。运行：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run '^TestOfflinePaymentBackfill'
```

预期：首次 5 条创建、重跑 no-op、任一前置条件错误均全量回滚。

提交：

```bash
git add backend/internal/service/offline_payment_backfill.go backend/internal/repository/offline_payment_backfill_integration_test.go
git commit -m "feat: add offline payment backfill service"
```

### 任务 3：提供默认 dry-run 的一次性二进制并纳入镜像

**文件：**
- 新增：`backend/cmd/offline-payment-backfill/main.go`
- 新增：`backend/cmd/offline-payment-backfill/main_test.go`
- 修改：`Dockerfile`

- [ ] **步骤 1：为命令参数写失败测试。**

将 flag 解析提取为纯函数，覆盖：

```go
require.ErrorContains(t, parseBackfillArgs(nil), "operator is required")
require.ErrorContains(t, parseBackfillArgs([]string{"--execute", "--operator=admin:13"}), "confirmation token")
require.NoError(t, parseBackfillArgs([]string{"--dry-run", "--operator=admin:13"}))
require.NoError(t, parseBackfillArgs([]string{
	"--execute", "--confirm=offline-paid-backfill-20260716", "--operator=admin:13",
}))
```

- [ ] **步骤 2：运行命令测试并确认入口尚不存在。**

运行：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/offline-payment-backfill
```

预期：目录或测试入口不存在，测试失败。

- [ ] **步骤 3：实现不自动迁移的命令。**

命令读取 `config.LoadForBootstrap()`，使用 `database/sql` 加 `cfg.Database.DSNWithTimezone(cfg.Timezone)` 直连 PostgreSQL；不得调用 `repository.InitEnt()`，因为它会自动应用所有嵌入 migration。命令语义固定为：

```text
--dry-run --operator=admin:13                         # 默认，校验后回滚
--execute --confirm=offline-paid-backfill-20260716 --operator=admin:13  # 唯一写入路径
```

命令输出每个订阅的预检结果、计划创建或已存在状态、总金额和最终模式；空 operator、未知 flag、错误确认 token、schema 未就绪或任一业务前置条件不符都以非零退出。

- [ ] **步骤 4：将二进制加入根 Dockerfile。**

在 backend builder stage 除 `/app/sub2api` 外构建：

```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -o /app/offline-payment-backfill \
    ./cmd/offline-payment-backfill
```

在 final stage 用与主二进制相同的 `sub2api:sub2api` 所有权复制到 `/app/offline-payment-backfill`。不要修改入口命令，也不要把该功能暴露为 HTTP 路由。

- [ ] **步骤 5：运行命令与镜像构建验证并提交。**

运行：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/offline-payment-backfill
go build ./cmd/offline-payment-backfill
cd ..
docker build --target backend-builder -t sub2api-offline-backfill-build-check .
```

预期：命令仅在显式 execute + 正确确认 token 时允许写路径，镜像 builder 能构建二进制。

提交：

```bash
git add backend/cmd/offline-payment-backfill/main.go backend/cmd/offline-payment-backfill/main_test.go Dockerfile
git commit -m "feat: add offline payment backfill command"
```

### 任务 4：以 TDD 完成订单展示、筛选、图表和退款按钮收口

**文件：**
- 修改：`frontend/src/i18n/locales/zh.ts`
- 修改：`frontend/src/i18n/locales/en.ts`
- 修改：`frontend/src/views/user/paymentRefund.ts`
- 修改：`frontend/src/components/payment/orderUtils.ts`
- 修改：`frontend/src/views/admin/orders/AdminOrdersView.vue`
- 修改：`frontend/src/components/admin/payment/AdminOrderTable.vue`
- 修改：`frontend/src/components/admin/payment/AdminOrderDetail.vue`
- 修改：`frontend/src/views/admin/orders/AdminPaymentDashboardView.vue`
- 修改：`frontend/src/components/payment/__tests__/paymentFlow.spec.ts`
- 修改：`frontend/src/views/user/__tests__/paymentRefund.spec.ts`
- 修改：`frontend/src/components/payment/__tests__/orderUtils.spec.ts`
- 修改：`frontend/src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts`
- 新增：`frontend/src/views/admin/orders/__tests__/AdminOrdersView.spec.ts`
- 新增：`frontend/src/views/admin/orders/__tests__/AdminPaymentDashboardView.spec.ts`

- [ ] **步骤 1：先写前端失败测试。**

测试必须覆盖：

```ts
// 这条为既有结账白名单的保护性断言，改动前应已通过。
expect(normalizeVisibleMethod('offline')).toBe('')
expect(canRequestOrderRefund(order({ payment_type: 'offline', status: 'COMPLETED' }), new Set())).toBe(false)
expect(canRequestOrderRefund(order({ payment_type: 'offline', status: 'REFUND_FAILED', refund_retryable: true }), new Set())).toBe(false)
expect(isAutomaticRefundAllowed('offline')).toBe(false)
```

以 `offline` 和 `manual_grant` 订单挂载共享 `OrderTable`，分别断言页面文本为“私下付款”和“赠送金额”。挂载 `AdminOrdersView` 并返回 `COMPLETED/offline` 订单，断言筛选有“私下付款”但没有退款按钮。挂载 dashboard 并返回 `{ type: 'offline', amount: 29, count: 1 }`，断言显示“私下付款”、`¥29.00`、`(1)` 和固定颜色类。

- [ ] **步骤 2：运行目标 Vitest 并确认新断言失败。**

运行：

```bash
cd frontend
pnpm test:run -- src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/orderUtils.spec.ts src/views/user/__tests__/paymentRefund.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts src/views/admin/orders/__tests__/AdminOrdersView.spec.ts src/views/admin/orders/__tests__/AdminPaymentDashboardView.spec.ts
```

预期：`normalizeVisibleMethod('offline')` 保护性断言保持通过；文案、`REFUND_FAILED` 退款保护、管理员退款按钮、筛选或图表颜色相关的新断言失败。

- [ ] **步骤 3：实现文案与纯退款资格守卫。**

在两个 locale 的 `payment.methods` 中加入：

```ts
offline: '私下付款',          // 英文：'Offline payment'
manual_grant: '赠送金额',     // 英文：'Gift amount'
```

在 `orderUtils.ts` 添加：

```ts
export function isAutomaticRefundAllowed(paymentType: string): boolean {
  return paymentType !== 'offline'
}
```

在用户退款判断最前面调用该函数；在两个管理员表格/详情组件与活跃 `AdminOrdersView` 的退款按钮条件中同时叠加该函数。不得改变支付宝、余额和既有退款状态的行为。

- [ ] **步骤 4：实现筛选和图表显示，不让 offline 进入结账。**

在 `AdminOrdersView` 和兼容 `AdminOrderTable` 的 payment type options 中仅新增：

```ts
{ value: 'offline', label: t('payment.methods.offline') }
```

在 `AdminPaymentDashboardView.methodColor()` 中为 `offline` 添加中性色，例如 `bg-slate-500`。不要修改 `frontend/src/types/payment.ts` 的用户配置 `PaymentType`、`paymentFlow.ts` 可见方式白名单、Provider 配置或 `SettingsView` 支付方式列表；这些改动会错误把 `offline` 暴露为用户结账能力。

- [ ] **步骤 5：运行前端目标测试、类型检查和构建并提交。**

运行：

```bash
cd frontend
pnpm test:run -- src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/orderUtils.spec.ts src/views/user/__tests__/paymentRefund.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts src/views/admin/orders/__tests__/AdminOrdersView.spec.ts src/views/admin/orders/__tests__/AdminPaymentDashboardView.spec.ts
pnpm typecheck
pnpm build
```

预期：所有目标测试通过，`offline` 只作为历史订单显示，不在购买页出现。

提交：

```bash
git add frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts frontend/src/views/user/paymentRefund.ts frontend/src/components/payment/orderUtils.ts frontend/src/views/admin/orders/AdminOrdersView.vue frontend/src/components/admin/payment/AdminOrderTable.vue frontend/src/components/admin/payment/AdminOrderDetail.vue frontend/src/views/admin/orders/AdminPaymentDashboardView.vue frontend/src/components/payment/__tests__/paymentFlow.spec.ts frontend/src/components/payment/__tests__/orderUtils.spec.ts frontend/src/views/user/__tests__/paymentRefund.spec.ts frontend/src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts frontend/src/views/admin/orders/__tests__/AdminOrdersView.spec.ts frontend/src/views/admin/orders/__tests__/AdminPaymentDashboardView.spec.ts
git commit -m "feat: display offline payment orders"
```

### 任务 5：全量验证与独立运维执行手册

**文件：**
- 新增：`docs/ai/context/$(TZ=Asia/Shanghai date +%Y%m%d-%H%M%S)-offline-payment-backfill-result_CN.md`，仅在代码验证或获得运行态授权后记录实际结果。

- [ ] **步骤 1：运行代码级全量验证。**

运行：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 ./internal/payment
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/offline-payment-backfill
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run 'TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate|TestOfflinePaymentBackfill'
cd ../frontend
pnpm typecheck
pnpm build
cd ..
git diff --check
```

预期：测试、编译、构建和 diff 检查均通过。若完整前端回归仍有既有失败，结果文档必须列出未触及的失败文件与本次目标测试结果。

- [ ] **步骤 2：完成代码审查和提交前上下文检查。**

运行：

```bash
git diff --check
git ls-files --others --exclude-standard docs/ai/context
git status --short
```

预期：不暂存、不回退用户已有的 `AGENTS.md` 与其他任务文件；仅暂存本计划实际创建的代码、测试和新的结果文档。

- [ ] **步骤 3：将运行态补录作为单独、需再次授权的操作。**

代码合并或部署本身不等于允许写入候选/生产数据。获得明确运行态授权后，按以下顺序执行：

```bash
# 先确认目标库已显式应用 162 和 163，而不是由补录命令隐式迁移。
docker exec sub2api-candidate-postgres psql -U sub2api -d sub2api -Atc \
  "SELECT filename FROM schema_migrations WHERE filename IN ('162_refund_state_machine.sql','163_alipay_balance_hybrid_payment.sql') ORDER BY filename;"

# 部署导致迁移前、补录命令紧邻执行前，各创建一份 custom-format 备份。
backup="deploy/backups/$(TZ=Asia/Shanghai date +%Y%m%d-%H%M%S)-sub2api-candidate-before-offline-payment-backfill.dump"
umask 077
docker exec sub2api-candidate-postgres pg_dump -U sub2api -d sub2api --format=custom --no-owner --no-privileges > "$backup"
chmod 600 "$backup"
docker exec -i sub2api-candidate-postgres pg_restore -l < "$backup" > /dev/null

# 仅在已部署含新二进制的目标容器中先 dry-run。
docker exec --user sub2api sub2api-candidate /app/offline-payment-backfill \
  --dry-run --operator='admin:13'

# dry-run 的五条前置条件全部通过后才允许 execute。
docker exec --user sub2api sub2api-candidate /app/offline-payment-backfill \
  --execute --confirm=offline-paid-backfill-20260716 --operator='admin:13'
```

实际执行必须以真实执行管理员替换 `admin:13`。补录后以只读 SQL 核对：5 条 `offline/COMPLETED/subscription`、`SUM(pay_amount)=145.00`、5 条 `OFFLINE_PAYMENT_RECORDED`、五个订阅到期时间不变、无 `payment_balance_holds`、流量卡或 affiliate 账本新行。

- [ ] **步骤 4：归档结果并提交。**

新增带实际时间戳的结果文档，记录代码验证、是否部署、是否已执行 dry-run/execute、备份路径和只读核对结果。完成后只暂存本次新结果文档与未在前述任务提交的文件：

```bash
result_doc="docs/ai/context/$(TZ=Asia/Shanghai date +%Y%m%d-%H%M%S)-offline-payment-backfill-result_CN.md"
# 使用同一个 result_doc 变量写入结果内容后再暂存，不能重新取时间戳。
git add "$result_doc"
git commit -m "docs: record offline payment backfill result"
```

不要在未获运行态授权时执行上述容器、备份或数据库写入命令。
