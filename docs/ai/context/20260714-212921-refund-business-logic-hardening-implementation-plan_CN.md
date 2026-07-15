# 退款业务逻辑修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复当前套餐退款的自然日比例、手续费基数、幂等重试和资金/权益一致性，不导入旧消费记录。

**Architecture:** 继续使用 `payment_orders` 作为退款聚合根，增加准确订阅关联和资金/权益分阶段状态。同步网关退款先落资金成功事实再撤销权益；余额退款在单个 Ent 事务内完成。

**Tech Stack:** Go、Ent、PostgreSQL、Gin、Vue 3、TypeScript、Vitest。

---

### Task 1: 扩展订单退款事实字段

**Files:**
- Create: `backend/migrations/162_refund_state_machine.sql`
- Modify: `backend/ent/schema/payment_order.go`
- Modify: `backend/internal/repository/migrations_schema_integration_test.go`
- Generated: `backend/ent/**`

- [ ] **Step 1: 写 schema 失败测试**

在 `migrations_schema_integration_test.go` 增加断言：

```go
requireColumn(t, tx, "payment_orders", "subscription_id")
requireColumn(t, tx, "payment_orders", "refund_request_id")
requireColumn(t, tx, "payment_orders", "refund_gateway_status")
requireColumn(t, tx, "payment_orders", "refund_entitlement_status")
requireColumn(t, tx, "payment_orders", "refund_provider_ref")
requireForeignKey(t, tx, "payment_orders", "subscription_id", "user_subscriptions")
```

- [ ] **Step 2: 运行测试确认 RED**

Run:

```bash
cd backend
go test -count=1 ./internal/repository -run TestMigrationsSchema
```

Expected: FAIL，缺少退款状态字段。

- [ ] **Step 3: 新增幂等迁移**

`162_refund_state_machine.sql`：

```sql
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS subscription_id BIGINT;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS refund_request_id VARCHAR(128);
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS refund_gateway_status VARCHAR(20) NOT NULL DEFAULT 'NOT_STARTED';
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS refund_entitlement_status VARCHAR(20) NOT NULL DEFAULT 'NOT_STARTED';
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS refund_provider_ref VARCHAR(128);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'payment_orders_subscription_id_fkey'
    ) THEN
        ALTER TABLE payment_orders
            ADD CONSTRAINT payment_orders_subscription_id_fkey
            FOREIGN KEY (subscription_id) REFERENCES user_subscriptions(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_payment_orders_subscription_id ON payment_orders(subscription_id);

WITH candidates AS (
    SELECT po.id AS order_id, MIN(us.id) AS subscription_id, COUNT(*) AS matched
    FROM payment_orders po
    JOIN user_subscriptions us
      ON us.user_id = po.user_id
     AND us.group_id = po.subscription_group_id
     AND us.notes ~ ('(^|\\n)payment order ' || po.id::text || '($|\\n)')
    WHERE po.order_type = 'subscription'
      AND po.subscription_id IS NULL
    GROUP BY po.id
)
UPDATE payment_orders po
SET subscription_id = candidates.subscription_id
FROM candidates
WHERE po.id = candidates.order_id
  AND candidates.matched = 1;
```

- [ ] **Step 4: 更新 Ent schema 并生成代码**

新增字段：

```go
field.Int64("subscription_id").Optional().Nillable(),
field.String("refund_request_id").Optional().Nillable().MaxLen(128),
field.String("refund_gateway_status").MaxLen(20).Default("NOT_STARTED"),
field.String("refund_entitlement_status").MaxLen(20).Default("NOT_STARTED"),
field.String("refund_provider_ref").Optional().Nillable().MaxLen(128),
```

并执行：

```bash
cd backend
go generate ./ent
```

- [ ] **Step 5: 运行 schema 测试确认 GREEN**

Run:

```bash
go test -count=1 ./internal/repository -run TestMigrationsSchema
```

Expected: PASS。

### Task 2: 按北京时间自然日计算退款金额

**Files:**
- Modify: `backend/internal/service/payment_amounts.go`
- Modify: `backend/internal/service/payment_refund_test.go`

- [ ] **Step 1: 写自然日和手续费失败测试**

新增表驱动测试：

```go
func TestCalculateSubscriptionRefundAmountUsesBeijingCalendarDays(t *testing.T) {
    start := time.Date(2026, 7, 1, 23, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
    tests := []struct {
        name string
        now  time.Time
        want float64
    }{
        {"购买当天", time.Date(2026, 7, 1, 23, 30, 0, 0, start.Location()), 28.0},
        {"第六个自然日", time.Date(2026, 7, 6, 1, 0, 0, 0, start.Location()), 23.2},
        {"第30天", time.Date(2026, 7, 30, 0, 1, 0, 0, start.Location()), 0},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            require.Equal(t, tt.want, calculateSubscriptionRefundAmount(29, 30, start, tt.now))
        })
    }
}

func TestBalanceSubscriptionRefundExcludesFee(t *testing.T) {
    // amount=29, pay_amount=29.29，第6天仍应退23.2，而不是23.4。
}
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
go test -count=1 -tags=unit ./internal/service -run 'TestCalculateSubscriptionRefundAmountUsesBeijingCalendarDays|TestBalanceSubscriptionRefundExcludesFee'
```

Expected: FAIL，当前使用剩余 24 小时块且余额退款包含手续费。

- [ ] **Step 3: 实现最小自然日 helper**

```go
var refundBusinessLocation = time.FixedZone("UTC+8", 8*60*60)

func refundCalendarDayIndex(startsAt, now time.Time) int {
    start := startsAt.In(refundBusinessLocation)
    current := now.In(refundBusinessLocation)
    startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
    currentDate := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, time.UTC)
    return int(currentDate.Sub(startDate)/(24*time.Hour)) + 1
}

func calculateSubscriptionRefundAmount(orderAmount float64, subscriptionDays int, startsAt, now time.Time) float64 {
    usedDays := refundCalendarDayIndex(startsAt, now)
    if usedDays < 1 { usedDays = 1 }
    if usedDays >= subscriptionDays { return 0 }
    remainingDays := subscriptionDays - usedDays
    return decimal.NewFromFloat(orderAmount).
        Mul(decimal.NewFromInt(int64(remainingDays))).
        Div(decimal.NewFromInt(int64(subscriptionDays))).
        Round(1).
        InexactFloat64()
}
```

调用方统一传 `o.Amount` 和准确订阅 `StartsAt`，删除 `includeFee`。

- [ ] **Step 4: 运行测试确认 GREEN**

```bash
go test -count=1 -tags=unit ./internal/service -run 'TestCalculateSubscriptionRefundAmount|TestBalanceSubscriptionRefundExcludesFee'
```

Expected: PASS。

### Task 3: 履约时持久化准确订阅 ID

**Files:**
- Modify: `backend/internal/service/payment_fulfillment.go`
- Modify: `backend/internal/service/payment_fulfillment_test.go`

- [ ] **Step 1: 写失败测试**

扩展订阅履约测试，完成后断言：

```go
reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
require.NoError(t, err)
require.NotNil(t, reloaded.SubscriptionID)
require.Equal(t, assignedSubscription.ID, *reloaded.SubscriptionID)
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
go test -count=1 -tags=unit ./internal/service -run TestExecuteSubscriptionFulfillment
```

Expected: FAIL，订单没有 `subscription_id`。

- [ ] **Step 3: 保存返回的订阅 ID**

把：

```go
_, _, err = s.subscriptionSvc.AssignOrExtendSubscription(...)
```

改为：

```go
sub, _, err := s.subscriptionSvc.AssignOrExtendSubscription(...)
if err != nil { return fmt.Errorf("assign subscription: %w", err) }
if sub == nil { return fmt.Errorf("assign subscription returned nil") }
if _, err := client.PaymentOrder.UpdateOneID(o.ID).SetSubscriptionID(sub.ID).Save(ctx); err != nil {
    return fmt.Errorf("link subscription order: %w", err)
}
```

- [ ] **Step 4: 运行目标与余额购买回归测试**

```bash
go test -count=1 -tags=unit ./internal/service -run 'TestExecuteSubscriptionFulfillment|TestBalancePay'
```

Expected: PASS。

### Task 4: Provider 使用稳定退款请求号并区分明确拒绝

**Files:**
- Modify: `backend/internal/payment/types.go`
- Modify: `backend/internal/payment/provider/easypay.go`
- Modify: `backend/internal/payment/provider/alipay.go`
- Modify: `backend/internal/payment/provider/wxpay.go`
- Modify: `backend/internal/payment/provider/stripe.go`
- Modify: `backend/internal/payment/provider/airwallex.go`
- Modify: provider refund tests

- [ ] **Step 1: 写 Provider 失败测试**

覆盖：

```go
require.Equal(t, "refund-order-7", capturedOutRequestNo)
require.Equal(t, "refund-order-7", capturedOutRefundNo)
require.Equal(t, "refund-order-7", capturedStripeIdempotencyKey)
require.True(t, payment.IsRefundRejected(err))
```

- [ ] **Step 2: 运行 Provider 测试确认 RED**

```bash
go test -count=1 ./internal/payment/provider -run 'Test.*Refund'
```

- [ ] **Step 3: 增加请求 ID 和明确拒绝错误**

```go
type RefundRequest struct {
    TradeNo  string
    OrderID  string
    Amount   string
    Reason   string
    RequestID string
}

type RefundRejectedError struct{ Err error }
func (e *RefundRejectedError) Error() string { return e.Err.Error() }
func (e *RefundRejectedError) Unwrap() error { return e.Err }
func IsRefundRejected(err error) bool {
    var target *RefundRejectedError
    return errors.As(err, &target)
}
```

Provider 使用 `req.RequestID`；EasyPay 仅对解析成功且 `code != 1` 的业务响应包装 `RefundRejectedError`。

- [ ] **Step 4: 运行 Provider 测试确认 GREEN**

```bash
go test -count=1 ./internal/payment/provider -run 'Test.*Refund'
```

Expected: PASS。

### Task 5: 重构用户网关退款状态机

**Files:**
- Create: `backend/internal/service/payment_refund_state.go`
- Modify: `backend/internal/service/payment_refund.go`
- Modify: `backend/internal/service/payment_refund_test.go`

- [ ] **Step 1: 写状态机失败测试**

新增测试：

```go
func TestRequestRefundRetryAfterGatewaySuccessOnlyRevokesEntitlement(t *testing.T) {}
func TestRequestRefundRejectedGatewayIsRetryable(t *testing.T) {}
func TestRequestRefundUnknownGatewayResultIsNotRetryable(t *testing.T) {}
func TestRequestRefundPendingDoesNotRevokeSubscription(t *testing.T) {}
func TestRequestRefundRetryReusesOriginalAmountAndRequestID(t *testing.T) {}
func TestGwRefundUsesOutTradeNoWithoutPaymentTradeNo(t *testing.T) {}
func TestExecuteRefundAfterGatewaySuccessOnlyRetriesEntitlement(t *testing.T) {}
func TestExecuteRefundUnknownGatewayResultRequiresManualReconciliation(t *testing.T) {}
```

断言 Provider 调用次数、订单网关/权益状态、订阅是否存在及退款金额。

- [ ] **Step 2: 运行测试确认 RED**

```bash
go test -count=1 -tags=unit ./internal/service -run 'TestRequestRefund(Retry|Rejected|Unknown|Pending)|TestGwRefundUsesOutTradeNo'
```

- [ ] **Step 3: 增加状态常量与资格判断**

`payment_refund_state.go`：

```go
const (
    RefundGatewayNotStarted = "NOT_STARTED"
    RefundGatewayNotRequired = "NOT_REQUIRED"
    RefundGatewayPending = "PENDING"
    RefundGatewaySucceeded = "SUCCEEDED"
    RefundGatewayFailed = "FAILED"
    RefundGatewayUnknown = "UNKNOWN"
    RefundEntitlementNotStarted = "NOT_STARTED"
    RefundEntitlementSucceeded = "SUCCEEDED"
    RefundEntitlementFailed = "FAILED"
)

func paymentOrderRefundRetryable(o *dbent.PaymentOrder) bool {
    return o != nil && o.Status == OrderStatusRefundFailed &&
        (o.RefundGatewayStatus == RefundGatewayFailed ||
         o.RefundGatewayStatus == RefundGatewaySucceeded && o.RefundEntitlementStatus == RefundEntitlementFailed)
}
```

- [ ] **Step 4: 首次申请与重试复用持久化事实**

- 首次 `COMPLETED`：计算金额、生成 `refund-<orderID>`、写 `REFUNDING`。
- `REFUND_FAILED + gateway FAILED`：复用 `refund_amount` 和 `refund_request_id`，只重试网关。
- `gateway SUCCEEDED + entitlement FAILED`：不调用网关，只重试撤权。
- `UNKNOWN/PENDING`：拒绝自动重试。

管理员 `ExecuteRefund()` 使用同一状态判断：`SUCCEEDED + entitlement FAILED` 只能重试撤权，`UNKNOWN/PENDING` 不得再次调用 Provider。

- [ ] **Step 5: 网关结果先落库再撤权**

成功响应先更新：

```go
SetRefundGatewayStatus(RefundGatewaySucceeded).
SetNillableRefundProviderRef(psNilIfEmpty(resp.RefundID))
```

随后按 `subscription_id` 撤销；失败写权益 `FAILED`，成功写 `SUCCEEDED` 和最终状态。

- [ ] **Step 6: 修复缺交易号行为**

`gwRefund()` 只在 `OutTradeNo` 和 `PaymentTradeNo` 同时为空时返回错误；不再写 `REFUND_NO_TRADE_NO` 后假成功。

- [ ] **Step 7: 运行目标测试确认 GREEN**

```bash
go test -count=1 -tags=unit ./internal/service -run 'Test.*Refund'
```

Expected: PASS。

### Task 6: 余额退款改为单事务

**Files:**
- Modify: `backend/internal/service/payment_refund.go`
- Modify: `backend/internal/service/payment_refund_test.go`

- [ ] **Step 1: 写事务失败测试**

模拟订阅更新失败，断言：

```go
require.Equal(t, 0.0, reloadedUser.Balance)
require.Equal(t, OrderStatusCompleted, reloadedOrder.Status)
require.Equal(t, SubscriptionStatusActive, reloadedSubscription.Status)
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
go test -count=1 -tags=unit ./internal/service -run TestBalanceRefundRollsBackAllState
```

- [ ] **Step 3: 在 Ent 事务内完成余额退款**

事务内：

```go
tx.User.UpdateOneID(userID).AddBalance(refundAmount)
tx.UserSubscription.UpdateOneID(subscriptionID).
    SetStatus(SubscriptionStatusExpired).
    SetDeletedAt(now)
tx.PaymentOrder.UpdateOneID(orderID).
    SetStatus(finalStatus).
    SetRefundGatewayStatus(RefundGatewayNotRequired).
    SetRefundEntitlementStatus(RefundEntitlementSucceeded)
```

审计日志使用 `writeAuditLogWithClient()` 写入同一事务，提交后失效余额和订阅缓存。

- [ ] **Step 4: 运行余额退款测试确认 GREEN**

```bash
go test -count=1 -tags=unit ./internal/service -run 'Test.*Balance.*Refund'
```

Expected: PASS。

### Task 7: API 返回服务端退款资格并支持安全重试

**Files:**
- Modify: `backend/internal/handler/payment_handler.go`
- Modify: `backend/internal/handler/admin/payment_handler.go`
- Modify: `frontend/src/types/payment.ts`
- Modify: `frontend/src/views/user/paymentRefund.ts`
- Modify: `frontend/src/views/user/__tests__/paymentRefund.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

- [ ] **Step 1: 写前端失败测试**

```ts
it('退款明确失败且服务端允许时显示重试', () => {
  expect(canRequestOrderRefund({ ...order, status: 'REFUND_FAILED', refund_retryable: true }, providers)).toBe(true)
})

it('未知或处理中退款不显示重试', () => {
  expect(canRequestOrderRefund({ ...order, status: 'REFUND_FAILED', refund_retryable: false }, providers)).toBe(false)
  expect(canRequestOrderRefund({ ...order, status: 'REFUNDING', refund_retryable: true }, providers)).toBe(false)
})
```

- [ ] **Step 2: 运行前端测试确认 RED**

```bash
pnpm vitest run src/views/user/__tests__/paymentRefund.spec.ts
```

- [ ] **Step 3: 后端响应增加字段**

用户和管理员订单结果增加：

```go
RefundRetryable bool `json:"refund_retryable"`
```

值由 `paymentOrderRefundRetryable(order)` 计算，前端类型增加 `refund_retryable?: boolean`。

- [ ] **Step 4: 前端使用服务端资格**

```ts
if (order.status === 'REFUND_FAILED') return order.refund_retryable === true
if (order.status !== 'COMPLETED') return false
```

按钮文案在失败重试时显示“重试退款”。

- [ ] **Step 5: 运行前端测试确认 GREEN**

```bash
pnpm vitest run src/views/user/__tests__/paymentRefund.spec.ts
pnpm typecheck
```

Expected: PASS。

### Task 8: 完整回归与结果文档

**Files:**
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-refund-business-logic-hardening-result_CN.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: 运行后端测试**

```bash
cd backend
go test -count=1 -tags=unit ./internal/service
go test -count=1 ./internal/payment/provider ./internal/repository
go test -count=1 ./cmd/server
```

- [ ] **Step 2: 运行前端测试与构建**

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/paymentRefund.spec.ts src/views/user/__tests__/paymentUx.spec.ts
pnpm typecheck
pnpm build
```

- [ ] **Step 3: 运行格式和生成代码检查**

```bash
gofmt -w \
  backend/ent/schema/payment_order.go \
  backend/internal/payment/types.go \
  backend/internal/payment/provider/easypay.go \
  backend/internal/payment/provider/alipay.go \
  backend/internal/payment/provider/wxpay.go \
  backend/internal/payment/provider/stripe.go \
  backend/internal/payment/provider/airwallex.go \
  backend/internal/service/payment_amounts.go \
  backend/internal/service/payment_fulfillment.go \
  backend/internal/service/payment_refund.go \
  backend/internal/service/payment_refund_state.go \
  backend/internal/handler/payment_handler.go \
  backend/internal/handler/admin/payment_handler.go \
  backend/internal/service/payment_refund_test.go \
  backend/internal/service/payment_fulfillment_test.go
git diff --check
git status --short
```

- [ ] **Step 4: 记录验证证据**

结果文档必须记录：

- RED/GREEN 测试名称和结果。
- 迁移字段与当前订单关联范围。
- 自然日、手续费和重试口径。
- 未导入旧消费记录、未部署、未修改运行态。
- 完整验证命令和输出结论。
