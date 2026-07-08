# 人民币余额支付与邀请返利重构 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 按 `docs/ai/context/20260707-225836-rmb-balance-payment-affiliate-rebate-design_CN.md` 落地人民币余额充值、余额支付购买套餐/流量包、支付宝邀请返利 8% 规则、订单展示和后台筛选修复。

**Architecture:** 外部支付仍走现有 `POST /api/v1/payment/orders` 和 webhook 履约；余额支付新增专用 `POST /api/v1/payment/orders/balance-pay`，在服务端事务内完成校验、条件扣款、订单创建和权益发放。履约能力从外部支付状态流中抽出为可在事务内复用的订阅/流量包发放函数；邀请返利只在 `payment_type=alipay` 的真实完成订单上触发。

**Tech Stack:** Go + Gin + ent + PostgreSQL migrations；Vue 3 + Pinia + TypeScript + Vitest；后端测试使用现有 `go test -tags=unit` 与 ent 测试客户端。

---

## 输入与非目标

输入设计：`docs/ai/context/20260707-225836-rmb-balance-payment-affiliate-rebate-design_CN.md`。

本计划不删除后台 provider 配置，不删除历史微信/Stripe/Airwallex 订单，不实现混合支付，不新增余额支付自助退款，不改变 API 用量按美元额度计费的逻辑。

## 文件结构

**后端支付核心**

- Modify: `backend/internal/payment/types.go`  
  增加内部 `payment_type=balance` 常量和订单类型白名单 helper。
- Modify: `backend/internal/service/payment_amounts.go`  
  保留历史退款倍率 helper，但新增人民币直付金额口径，余额充值固定 1:1。
- Modify: `backend/internal/service/payment_order.go`  
  在外部订单创建链路中校验 `order_type`，余额充值固定 `fee_rate=0`、`amount=pay_amount`，用户侧外部支付只允许支付宝。
- Create: `backend/internal/service/payment_balance_pay.go`  
  新增余额支付商品事务：校验商品、条件扣款、创建 `payment_type=balance` 完成订单、发放权益、写审计、清理缓存。
- Modify: `backend/internal/service/payment_service.go`  
  增加 `BalancePayOrderRequest` / `BalancePayOrderResponse` 类型和 `billingCacheService` 依赖 setter。
- Modify: `backend/internal/service/payment_fulfillment.go`  
  抽出 `fulfillSubscriptionOrderInTx` / `fulfillTrafficPackOrderInTx`，并让外部支付与余额支付复用。
- Modify: `backend/internal/handler/payment_handler.go`  
  新增 `BalancePayOrder` handler。
- Modify: `backend/internal/server/routes/payment.go`  
  注册 `POST /api/v1/payment/orders/balance-pay`。
- Modify: `backend/cmd/server/wire.go` and `backend/cmd/server/wire_gen.go`  
  注入 `BillingCacheService` 到 `PaymentService`。
- Create: `backend/migrations/160_rmb_balance_payment_affiliate_defaults.sql`  
  Upsert 返利运行态 settings：8%、24 小时、365 天、100 元上限。

**后端测试**

- Modify: `backend/internal/payment/types_test.go`
- Modify: `backend/internal/service/payment_order_result_test.go`
- Modify: `backend/internal/service/payment_fulfillment_test.go`
- Create: `backend/internal/service/payment_balance_pay_test.go`
- Modify: `backend/migrations/auth_identity_payment_migrations_regression_test.go`

**前端购买与订单**

- Modify: `frontend/src/types/payment.ts`
- Modify: `frontend/src/api/payment.ts`
- Modify: `frontend/src/components/payment/paymentFlow.ts`
- Modify: `frontend/src/components/payment/PaymentMethodSelector.vue`
- Modify: `frontend/src/components/payment/providerConfig.ts`
- Modify: `frontend/src/views/user/PaymentView.vue`
- Modify: `frontend/src/views/user/__tests__/PaymentView.spec.ts`
- Modify: `frontend/src/api/__tests__/payment.spec.ts`
- Modify: `frontend/src/components/payment/__tests__/paymentFlow.spec.ts`
- Modify: `frontend/src/components/payment/OrderTable.vue`
- Modify: `frontend/src/components/admin/payment/AdminOrderTable.vue`
- Modify: `frontend/src/components/admin/payment/AdminOrderDetail.vue`
- Modify: `frontend/src/components/admin/payment/AdminRefundDialog.vue`
- Modify: `frontend/src/components/payment/PaymentQRDialog.vue`
- Modify: `frontend/src/components/payment/PaymentStatusPanel.vue`
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/views/admin/affiliates/AdminAffiliateRecordsTable.vue`
- Modify: `frontend/src/views/admin/RedeemView.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**文档与记忆**

- Create: `docs/ai/context/YYYYMMDD-HHMMSS-rmb-balance-payment-affiliate-rebate-result_CN.md`
- Modify: `AGENTS.md`

## Task 1: 订单类型白名单与人民币金额口径

**Files:**

- Modify: `backend/internal/payment/types.go`
- Modify: `backend/internal/payment/types_test.go`
- Modify: `backend/internal/service/payment_amounts.go`
- Modify: `backend/internal/service/payment_order.go`
- Modify: `backend/internal/service/payment_order_result_test.go`

- [ ] **Step 1: 写订单类型白名单失败测试**

在 `backend/internal/payment/types_test.go` 增加：

```go
package payment

import (
	"strings"
	"testing"
)

func TestNormalizeOrderTypeDefaultsEmptyToBalance(t *testing.T) {
	got, ok := NormalizeOrderType("")
	if !ok || got != OrderTypeBalance {
		t.Fatalf("NormalizeOrderType(empty) = (%q,%v), want (%q,true)", got, ok, OrderTypeBalance)
	}
}

func TestNormalizeOrderTypeAllowsKnownValues(t *testing.T) {
	for _, input := range []string{OrderTypeBalance, OrderTypeSubscription, OrderTypeTrafficPack, " subscription "} {
		got, ok := NormalizeOrderType(input)
		if !ok {
			t.Fatalf("NormalizeOrderType(%q) rejected", input)
		}
		if got != strings.TrimSpace(input) {
			t.Fatalf("NormalizeOrderType(%q) = %q", input, got)
		}
	}
}

func TestNormalizeOrderTypeRejectsUnknownNonEmpty(t *testing.T) {
	got, ok := NormalizeOrderType("evil")
	if ok || got != "" {
		t.Fatalf("NormalizeOrderType(evil) = (%q,%v), want empty false", got, ok)
	}
}
```

- [ ] **Step 2: 写金额口径失败测试**

在 `backend/internal/service/payment_order_result_test.go` 替换旧的倍率测试，新增：

```go
func TestCalculateCreateOrderPayAmountForRMBProductAddsOnlyFee(t *testing.T) {
	t.Parallel()

	amountStr, amount, err := calculateCreateOrderPayAmountForOrder(payment.OrderTypeSubscription, 79, 1, 0.14, "CNY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if amountStr != "79.79" || amount != 79.79 {
		t.Fatalf("subscription CNY pay amount = (%q,%v), want (79.79,79.79)", amountStr, amount)
	}
}

func TestCalculateCreateOrderPayAmountForBalanceRechargeIgnoresFeeAndMultiplier(t *testing.T) {
	t.Parallel()

	amountStr, amount, err := calculateCreateOrderPayAmountForOrder(payment.OrderTypeBalance, 10, 1, 0.14, "CNY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if amountStr != "10.00" || amount != 10 {
		t.Fatalf("balance recharge pay amount = (%q,%v), want (10.00,10)", amountStr, amount)
	}

	credited := calculateCreditedBalance(10, 0.14)
	if credited != 10 {
		t.Fatalf("credited balance = %v, want 10", credited)
	}
}
```

- [ ] **Step 3: 运行失败测试**

Run:

```bash
cd backend && go test -count=1 -tags=unit ./internal/payment ./internal/service -run 'TestNormalizeOrderType|TestCalculateCreateOrderPayAmountFor'
```

Expected: `NormalizeOrderType` 未定义，金额测试仍按旧倍率失败。

- [ ] **Step 4: 实现订单类型 helper**

在 `backend/internal/payment/types.go` 增加 `strings` import，并加入：

```go
const (
	TypeBalance PaymentType = "balance"
)

func NormalizeOrderType(raw string) (string, bool) {
	orderType := strings.TrimSpace(raw)
	if orderType == "" {
		return OrderTypeBalance, true
	}
	switch orderType {
	case OrderTypeBalance, OrderTypeSubscription, OrderTypeTrafficPack:
		return orderType, true
	default:
		return "", false
	}
}
```

现有 `import "context"` 改为：

```go
import (
	"context"
	"strings"
)
```

- [ ] **Step 5: 实现金额口径**

在 `backend/internal/service/payment_amounts.go` 修改：

```go
func calculateCreditedBalance(paymentAmount, multiplier float64) float64 {
	return decimal.NewFromFloat(paymentAmount).
		Round(2).
		InexactFloat64()
}
```

在 `backend/internal/service/payment_order.go` 修改：

```go
func calculateCreateOrderPayAmountForOrder(orderType string, limitAmount, feeRate, multiplier float64, currency string) (string, float64, error) {
	effectiveFeeRate := feeRate
	if orderType == payment.OrderTypeBalance {
		effectiveFeeRate = 0
	}
	return calculateCreateOrderPayAmount(limitAmount, effectiveFeeRate, currency)
}

func calculateCreateOrderPaymentAmount(orderType string, limitAmount, multiplier float64, currency string) float64 {
	return limitAmount
}
```

在 `CreateOrder` 开头替换默认逻辑：

```go
orderType, ok := payment.NormalizeOrderType(req.OrderType)
if !ok {
	return nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "invalid order type")
}
req.OrderType = orderType
```

在 `CreateOrder` 计算 `feeRate` 后增加：

```go
if req.OrderType == payment.OrderTypeBalance {
	feeRate = 0
}
```

- [ ] **Step 6: 运行并通过测试**

Run:

```bash
cd backend && go test -count=1 -tags=unit ./internal/payment ./internal/service -run 'TestNormalizeOrderType|TestCalculateCreateOrderPayAmountFor'
```

Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add backend/internal/payment/types.go backend/internal/payment/types_test.go backend/internal/service/payment_amounts.go backend/internal/service/payment_order.go backend/internal/service/payment_order_result_test.go
git commit -m "fix: normalize rmb payment order amounts"
```

## Task 2: 外部支付入口只允许支付宝与安全订单类型

**Files:**

- Modify: `backend/internal/service/payment_order.go`
- Modify: `backend/internal/service/payment_order_result_test.go`
- Modify: `frontend/src/components/payment/paymentFlow.ts`
- Modify: `frontend/src/components/payment/__tests__/paymentFlow.spec.ts`

- [ ] **Step 1: 写外部支付方法限制测试**

在 `backend/internal/service/payment_order_result_test.go` 增加纯函数测试：

```go
func TestValidateUserExternalPaymentTypeAllowsOnlyAlipay(t *testing.T) {
	t.Parallel()

	if err := validateUserExternalPaymentType(payment.TypeAlipay); err != nil {
		t.Fatalf("alipay rejected: %v", err)
	}
	for _, method := range []string{payment.TypeWxpay, payment.TypeStripe, payment.TypeAirwallex, payment.TypeBalance, ""} {
		err := validateUserExternalPaymentType(method)
		if err == nil {
			t.Fatalf("method %q should be rejected", method)
		}
		if appErr := infraerrors.FromError(err); appErr.Reason != "PAYMENT_METHOD_NOT_AVAILABLE" {
			t.Fatalf("reason = %q, want PAYMENT_METHOD_NOT_AVAILABLE", appErr.Reason)
		}
	}
}
```

- [ ] **Step 2: 实现后端限制**

在 `backend/internal/service/payment_order.go` 增加：

```go
func validateUserExternalPaymentType(paymentType string) error {
	if strings.TrimSpace(paymentType) == payment.TypeAlipay {
		return nil
	}
	return infraerrors.BadRequest("PAYMENT_METHOD_NOT_AVAILABLE", "payment method is not available for user checkout")
}
```

在 `CreateOrder` 规范化 `req.PaymentType` 后调用：

```go
if err := validateUserExternalPaymentType(req.PaymentType); err != nil {
	return nil, err
}
```

- [ ] **Step 3: 写前端可见方法过滤测试**

在 `frontend/src/components/payment/__tests__/paymentFlow.spec.ts` 增加：

```ts
it('filters user checkout external methods to alipay only', () => {
  const methods = getUserExternalPaymentMethods({
    alipay: { daily_limit: 0, daily_used: 0, daily_remaining: 0, single_min: 0, single_max: 0, fee_rate: 0, available: true },
    wxpay: { daily_limit: 0, daily_used: 0, daily_remaining: 0, single_min: 0, single_max: 0, fee_rate: 0, available: true },
    stripe: { daily_limit: 0, daily_used: 0, daily_remaining: 0, single_min: 0, single_max: 0, fee_rate: 0, available: true },
    airwallex: { daily_limit: 0, daily_used: 0, daily_remaining: 0, single_min: 0, single_max: 0, fee_rate: 0, available: true },
  })
  expect(Object.keys(methods)).toEqual(['alipay'])
})
```

- [ ] **Step 4: 实现前端过滤 helper**

在 `frontend/src/components/payment/paymentFlow.ts` 增加：

```ts
export function getUserExternalPaymentMethods(methods: Record<string, MethodLimit>): Record<string, MethodLimit> {
  const visible = getVisibleMethods(methods)
  return visible.alipay ? { alipay: visible.alipay } : {}
}
```

- [ ] **Step 5: 运行测试**

Run:

```bash
cd backend && go test -count=1 -tags=unit ./internal/service -run TestValidateUserExternalPaymentType
cd frontend && pnpm vitest run src/components/payment/__tests__/paymentFlow.spec.ts
```

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add backend/internal/service/payment_order.go backend/internal/service/payment_order_result_test.go frontend/src/components/payment/paymentFlow.ts frontend/src/components/payment/__tests__/paymentFlow.spec.ts
git commit -m "fix: restrict user checkout methods to alipay"
```

## Task 3: 余额支付后端事务

**Files:**

- Create: `backend/internal/service/payment_balance_pay.go`
- Modify: `backend/internal/service/payment_service.go`
- Modify: `backend/internal/service/payment_fulfillment.go`
- Modify: `backend/internal/handler/payment_handler.go`
- Modify: `backend/internal/server/routes/payment.go`
- Modify: `backend/cmd/server/wire.go`
- Modify: `backend/cmd/server/wire_gen.go`
- Create: `backend/internal/service/payment_balance_pay_test.go`

- [ ] **Step 1: 写余额不足失败测试**

在 `backend/internal/service/payment_balance_pay_test.go` 增加：

```go
//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestBalancePaySubscriptionInsufficientDoesNotCreateOrder(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	u, err := client.User.Create().
		SetEmail("balance-pay-insufficient@example.com").
		SetUsername("balance-pay-insufficient").
		SetPasswordHash("hash").
		SetBalance(10).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)

	svc := newBalancePayTestService(t, client, 0)
	_, err = svc.BalancePayOrder(ctx, BalancePayOrderRequest{
		UserID:    u.ID,
		OrderType: payment.OrderTypeSubscription,
		PlanID:    7,
		ClientIP:  "127.0.0.1",
		SrcHost:   "api.example.com",
	})
	require.Error(t, err)
	require.Equal(t, "BALANCE_INSUFFICIENT", infraerrors.FromError(err).Reason)

	count, err := client.PaymentOrder.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)

	reloaded, err := client.User.Query().Where(user.IDEQ(u.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 10.0, reloaded.Balance)
}
```

- [ ] **Step 2: 写余额足够发放套餐测试**

继续在 `backend/internal/service/payment_balance_pay_test.go` 增加：

```go
func TestBalancePaySubscriptionDeductsPayAmountAndCompletesOrder(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	u, err := client.User.Create().
		SetEmail("balance-pay-subscription@example.com").
		SetUsername("balance-pay-subscription").
		SetPasswordHash("hash").
		SetBalance(100).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)

	subRepo := newSubscriptionUserSubRepoStub()
	svc := newBalancePayTestService(t, client, 1)
	svc.subscriptionSvc = NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, subRepo, nil, nil, nil)

	resp, err := svc.BalancePayOrder(ctx, BalancePayOrderRequest{
		UserID:    u.ID,
		OrderType: payment.OrderTypeSubscription,
		PlanID:    7,
		ClientIP:  "127.0.0.1",
		SrcHost:   "api.example.com",
	})
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, resp.Status)
	require.Equal(t, payment.TypeBalance, resp.PaymentType)
	require.Equal(t, 79.0, resp.Amount)
	require.Equal(t, 79.79, resp.PayAmount)

	reloaded, err := client.User.Query().Where(user.IDEQ(u.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 20.21, reloaded.Balance)
	require.Equal(t, 1, subRepo.createCalls)
}
```

`newBalancePayTestService` fixture 要显式创建 plan/group stub，不隐藏商品价格：

```go
func newBalancePayTestService(t *testing.T, client *dbent.Client, feeRate float64) *PaymentService {
	t.Helper()
	settingSvc := NewSettingService(&paymentFulfillmentSettingRepoStub{values: map[string]string{
		SettingPaymentEnabled:     "true",
		SettingRechargeFeeRate:    strconv.FormatFloat(feeRate, 'f', -1, 64),
		SettingMaxPendingOrders:   "3",
		SettingOrderTimeoutMinutes:"30",
	}}, nil)
	cfgSvc := NewPaymentConfigService(client, settingSvc.settingRepo, nil)
	return &PaymentService{
		entClient:     client,
		configService: cfgSvc,
		userRepo:      repository.NewUserRepository(client),
		groupRepo:     &subscriptionGroupRepoStub{group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription}},
	}
}
```

- [ ] **Step 3: 写余额足够发放流量包测试**

在同文件增加：

```go
func TestBalancePayTrafficPackDeductsPayAmountAndCreditsPack(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	u, err := client.User.Create().
		SetEmail("balance-pay-traffic@example.com").
		SetUsername("balance-pay-traffic").
		SetPasswordHash("hash").
		SetBalance(20).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)

	trafficRepo := &paymentFulfillmentTrafficPackRepo{}
	svc := newBalancePayTestService(t, client, 1)
	svc.trafficPackService = NewTrafficPackService(trafficRepo)

	resp, err := svc.BalancePayOrder(ctx, BalancePayOrderRequest{
		UserID:        u.ID,
		OrderType:     payment.OrderTypeTrafficPack,
		TrafficPackID: 3,
		ClientIP:      "127.0.0.1",
		SrcHost:       "api.example.com",
	})
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, resp.Status)
	require.Equal(t, payment.TypeBalance, resp.PaymentType)
	require.Equal(t, payment.OrderTypeTrafficPack, resp.OrderType)
	require.Equal(t, 5.05, resp.PayAmount)
	require.Equal(t, 1, trafficRepo.creditCalls)
}
```

- [ ] **Step 4: 运行失败测试**

Run:

```bash
cd backend && go test -count=1 -tags=unit ./internal/service -run 'TestBalancePay'
```

Expected: `BalancePayOrder` 和测试 fixture 中需要的 repo wiring 尚未实现，测试失败。

- [ ] **Step 5: 增加请求与响应类型**

在 `backend/internal/service/payment_service.go` 增加：

```go
type BalancePayOrderRequest struct {
	UserID        int64
	OrderType     string
	PlanID        int64
	TrafficPackID int64
	ClientIP      string
	SrcHost       string
	SrcURL        string
	Locale        string
}

type BalancePayOrderResponse struct {
	OrderID     int64   `json:"order_id"`
	Amount      float64 `json:"amount"`
	PayAmount   float64 `json:"pay_amount"`
	FeeRate     float64 `json:"fee_rate"`
	Status      string  `json:"status"`
	PaymentType string  `json:"payment_type"`
	OrderType   string  `json:"order_type"`
	OutTradeNo  string  `json:"out_trade_no"`
	Currency    string  `json:"currency"`
}

func (s *PaymentService) SetBillingCacheService(cache *BillingCacheService) {
	s.billingCacheService = cache
}
```

在 `PaymentService` struct 增加：

```go
billingCacheService *BillingCacheService
```

- [ ] **Step 6: 实现余额支付事务**

创建 `backend/internal/service/payment_balance_pay.go`：

```go
package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *PaymentService) BalancePayOrder(ctx context.Context, req BalancePayOrderRequest) (*BalancePayOrderResponse, error) {
	orderType, ok := payment.NormalizeOrderType(req.OrderType)
	if !ok || orderType == payment.OrderTypeBalance {
		return nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "balance payment only supports product orders")
	}
	req.OrderType = orderType

	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get payment config: %w", err)
	}
	if !cfg.Enabled {
		return nil, infraerrors.Forbidden("PAYMENT_DISABLED", "payment system is disabled")
	}

	userEntity, err := s.entClient.User.Get(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if userEntity.Status != payment.EntityStatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}

	orderAmount, payAmount, feeRate, plan, trafficPack, err := s.resolveBalancePayProduct(ctx, req, cfg)
	if err != nil {
		return nil, err
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin balance pay tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.deductBalanceForPurchaseInTx(txCtx, tx, req.UserID, payAmount); err != nil {
		return nil, err
	}

	now := time.Now()
	outTradeNo, err := s.allocateOutTradeNo(txCtx, tx)
	if err != nil {
		return nil, err
	}
	orderBuilder := tx.PaymentOrder.Create().
		SetUserID(req.UserID).
		SetUserEmail(userEntity.Email).
		SetUserName(userEntity.Username).
		SetAmount(orderAmount).
		SetPayAmount(payAmount).
		SetFeeRate(feeRate).
		SetRechargeCode(fmt.Sprintf("PAY-BALANCE-%d", now.UnixNano())).
		SetOutTradeNo(outTradeNo).
		SetPaymentType(payment.TypeBalance).
		SetPaymentTradeNo("balance").
		SetOrderType(req.OrderType).
		SetStatus(OrderStatusRecharging).
		SetExpiresAt(now).
		SetPaidAt(now).
		SetClientIP(req.ClientIP).
		SetSrcHost(req.SrcHost)
	if req.SrcURL != "" {
		orderBuilder.SetSrcURL(req.SrcURL)
	}
	if plan != nil {
		orderBuilder.SetPlanID(plan.ID).SetSubscriptionGroupID(plan.GroupID).SetSubscriptionDays(psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit))
	}
	if trafficPack != nil {
		orderBuilder.SetProviderSnapshot(buildPaymentOrderProviderSnapshot(nil, CreateOrderRequest{}, trafficPack))
	}

	order, err := orderBuilder.Save(txCtx)
	if err != nil {
		return nil, fmt.Errorf("create balance payment order: %w", err)
	}

	switch req.OrderType {
	case payment.OrderTypeSubscription:
		err = s.fulfillSubscriptionOrderInTx(txCtx, tx.Client(), order, false)
	case payment.OrderTypeTrafficPack:
		err = s.fulfillTrafficPackOrderInTx(txCtx, tx.Client(), order)
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit balance pay tx: %w", err)
	}
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateUserBalance(context.Background(), req.UserID)
	}
	return &BalancePayOrderResponse{
		OrderID: order.ID, Amount: orderAmount, PayAmount: payAmount, FeeRate: feeRate,
		Status: OrderStatusCompleted, PaymentType: payment.TypeBalance, OrderType: req.OrderType,
		OutTradeNo: outTradeNo, Currency: payment.DefaultPaymentCurrency,
	}, nil
}

func (s *PaymentService) deductBalanceForPurchaseInTx(ctx context.Context, tx *dbent.Tx, userID int64, amount float64) error {
	n, err := tx.User.Update().Where(user.IDEQ(userID), user.BalanceGTE(amount)).AddBalance(-amount).Save(ctx)
	if err != nil {
		return fmt.Errorf("deduct balance: %w", err)
	}
	if n == 0 {
		return infraerrors.BadRequest("BALANCE_INSUFFICIENT", "balance is insufficient")
	}
	return nil
}
```

`resolveBalancePayProduct` 使用现有 `validateSubOrder` / `validateTrafficPackOrder`，并用 `calculateCreateOrderPayAmountForOrder(req.OrderType, price, cfg.RechargeFeeRate, cfg.BalanceRechargeMultiplier, payment.DefaultPaymentCurrency)` 计算 `payAmount`。

- [ ] **Step 7: 抽出事务内履约函数**

在 `backend/internal/service/payment_fulfillment.go` 调整：

```go
func (s *PaymentService) fulfillSubscriptionOrderInTx(ctx context.Context, client *dbent.Client, o *dbent.PaymentOrder, applyRebate bool) error {
	gid := *o.SubscriptionGroupID
	days := *o.SubscriptionDays
	g, err := s.groupRepo.GetByID(ctx, gid)
	if err != nil || g.Status != payment.EntityStatusActive {
		return fmt.Errorf("group %d no longer exists or inactive", gid)
	}
	if !s.hasAuditLogWithClient(ctx, client, o.ID, "SUBSCRIPTION_ASSIGNED") {
		orderNote := fmt.Sprintf("payment order %d", o.ID)
		_, _, err = s.subscriptionSvc.AssignOrExtendSubscription(ctx, &AssignSubscriptionInput{UserID: o.UserID, GroupID: gid, ValidityDays: days, AssignedBy: 0, Notes: orderNote})
		if err != nil {
			return fmt.Errorf("assign subscription: %w", err)
		}
		s.writeAuditLogWithClient(ctx, client, o.ID, "SUBSCRIPTION_ASSIGNED", "system", map[string]any{"groupID": gid, "validityDays": days})
	}
	if applyRebate {
		if err := s.applyAffiliateRebateForOrder(ctx, o); err != nil {
			return err
		}
	}
	return s.markCompletedWithClient(ctx, client, o, "SUBSCRIPTION_SUCCESS")
}

func (s *PaymentService) fulfillTrafficPackOrderInTx(ctx context.Context, client *dbent.Client, o *dbent.PaymentOrder) error {
	input, err := trafficPackCreditInputFromOrder(o)
	if err != nil {
		return err
	}
	if err := s.trafficPackService.CreditPurchase(ctx, input); err != nil {
		return err
	}
	return s.markCompletedWithClient(ctx, client, o, "TRAFFIC_PACK_SUCCESS")
}
```

把现有 `doSub` 改为调用 `fulfillSubscriptionOrderInTx(ctx, s.entClient, o, true)`；把 `ExecuteTrafficPackFulfillment` 中发放逻辑改为调用 `fulfillTrafficPackOrderInTx(ctx, s.entClient, o)`。

- [ ] **Step 8: 增加 handler 与 route**

在 `backend/internal/handler/payment_handler.go` 增加：

```go
type BalancePayOrderRequest struct {
	OrderType     string `json:"order_type" binding:"required"`
	PlanID        int64  `json:"plan_id"`
	TrafficPackID int64  `json:"traffic_pack_id"`
}

func (h *PaymentHandler) BalancePayOrder(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	var req BalancePayOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.paymentService.BalancePayOrder(c.Request.Context(), service.BalancePayOrderRequest{
		UserID: subject.UserID, OrderType: req.OrderType, PlanID: req.PlanID, TrafficPackID: req.TrafficPackID,
		ClientIP: c.ClientIP(), SrcHost: c.Request.Host, SrcURL: c.Request.Referer(), Locale: c.GetHeader("Accept-Language"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
```

在 `backend/internal/server/routes/payment.go` 中：

```go
orders.POST("/balance-pay", paymentHandler.BalancePayOrder)
```

- [ ] **Step 9: 注入余额缓存依赖**

在 `backend/cmd/server/wire.go` / `backend/cmd/server/wire_gen.go` 中，构造 `PaymentService` 后调用：

```go
paymentService.SetBillingCacheService(billingCacheService)
```

- [ ] **Step 10: 运行测试**

Run:

```bash
cd backend && go test -count=1 -tags=unit ./internal/service -run 'TestBalancePay|TestExecuteSubscriptionFulfillment|TestExecuteTrafficPackFulfillment'
```

Expected: PASS。

- [ ] **Step 11: 提交**

```bash
git add backend/internal/service/payment_balance_pay.go backend/internal/service/payment_service.go backend/internal/service/payment_fulfillment.go backend/internal/handler/payment_handler.go backend/internal/server/routes/payment.go backend/cmd/server/wire.go backend/cmd/server/wire_gen.go backend/internal/service/payment_balance_pay_test.go
git commit -m "feat: add rmb balance product payment"
```

## Task 4: 邀请返利规则与运行态默认值

**Files:**

- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/payment_fulfillment.go`
- Modify: `backend/internal/service/payment_fulfillment_test.go`
- Create: `backend/migrations/160_rmb_balance_payment_affiliate_defaults.sql`
- Modify: `backend/migrations/auth_identity_payment_migrations_regression_test.go`

- [ ] **Step 1: 写返利基数与支付方式测试**

在 `backend/internal/service/payment_fulfillment_test.go` 增加：

```go
func TestAffiliateRebateBaseAmountRequiresAlipayAndIncludesTrafficPack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		order dbent.PaymentOrder
		want float64
	}{
		{name: "alipay balance recharge", order: dbent.PaymentOrder{PaymentType: payment.TypeAlipay, OrderType: payment.OrderTypeBalance, Amount: 100, PayAmount: 100}, want: 100},
		{name: "alipay subscription uses amount not pay amount", order: dbent.PaymentOrder{PaymentType: payment.TypeAlipay, OrderType: payment.OrderTypeSubscription, Amount: 79, PayAmount: 79.79}, want: 79},
		{name: "alipay traffic pack", order: dbent.PaymentOrder{PaymentType: payment.TypeAlipay, OrderType: payment.OrderTypeTrafficPack, Amount: 5, PayAmount: 5.05}, want: 5},
		{name: "balance payment subscription skipped", order: dbent.PaymentOrder{PaymentType: payment.TypeBalance, OrderType: payment.OrderTypeSubscription, Amount: 79, PayAmount: 79.79}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := affiliateRebateBaseAmount(&tt.order); got != tt.want {
				t.Fatalf("affiliateRebateBaseAmount = %v, want %v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: 修改返利 base helper**

在 `backend/internal/service/payment_fulfillment.go` 修改：

```go
func affiliateRebateBaseAmount(o *dbent.PaymentOrder) float64 {
	if o == nil || o.PaymentType != payment.TypeAlipay {
		return 0
	}
	switch o.OrderType {
	case payment.OrderTypeBalance, payment.OrderTypeSubscription, payment.OrderTypeTrafficPack:
		return o.Amount
	default:
		return 0
	}
}
```

- [ ] **Step 3: 修改全局默认常量**

在 `backend/internal/service/domain_constants.go` 修改：

```go
AffiliateRebateRateDefault          = 8.0
AffiliateRebateFreezeHoursDefault   = 24
AffiliateRebateDurationDaysDefault  = 365
AffiliateRebatePerInviteeCapDefault = 100.0
```

- [ ] **Step 4: 新增运行态 settings 迁移**

创建 `backend/migrations/160_rmb_balance_payment_affiliate_defaults.sql`：

```sql
INSERT INTO settings (key, value, updated_at)
VALUES
  ('affiliate_rebate_rate', '8', NOW()),
  ('affiliate_rebate_freeze_hours', '24', NOW()),
  ('affiliate_rebate_duration_days', '365', NOW()),
  ('affiliate_rebate_per_invitee_cap', '100', NOW())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    updated_at = NOW()
WHERE settings.value IS DISTINCT FROM EXCLUDED.value;
```

- [ ] **Step 5: 写 migration 回归测试**

在 `backend/migrations/auth_identity_payment_migrations_regression_test.go` 增加：

```go
func TestMigration160SetsAffiliateRebateDefaults(t *testing.T) {
	sql := mustReadMigration(t, "160_rmb_balance_payment_affiliate_defaults.sql")
	require.Contains(t, sql, "affiliate_rebate_rate")
	require.Contains(t, sql, "'8'")
	require.Contains(t, sql, "affiliate_rebate_freeze_hours")
	require.Contains(t, sql, "'24'")
	require.Contains(t, sql, "affiliate_rebate_duration_days")
	require.Contains(t, sql, "'365'")
	require.Contains(t, sql, "affiliate_rebate_per_invitee_cap")
	require.Contains(t, sql, "'100'")
}
```

- [ ] **Step 6: 运行测试**

Run:

```bash
cd backend && go test -count=1 -tags=unit ./internal/service -run 'TestAffiliateRebateBaseAmount|TestExecuteSubscriptionFulfillmentAppliesAffiliateRebate'
cd backend && go test -count=1 ./migrations -run TestMigration160SetsAffiliateRebateDefaults
```

Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add backend/internal/service/domain_constants.go backend/internal/service/payment_fulfillment.go backend/internal/service/payment_fulfillment_test.go backend/migrations/160_rmb_balance_payment_affiliate_defaults.sql backend/migrations/auth_identity_payment_migrations_regression_test.go
git commit -m "fix: apply alipay affiliate rebate policy"
```

## Task 5: 前端 API 类型与余额支付调用

**Files:**

- Modify: `frontend/src/types/payment.ts`
- Modify: `frontend/src/api/payment.ts`
- Modify: `frontend/src/api/__tests__/payment.spec.ts`

- [ ] **Step 1: 写 API 测试**

在 `frontend/src/api/__tests__/payment.spec.ts` 增加：

```ts
it('posts balance pay orders to the dedicated endpoint', async () => {
  mockPost.mockResolvedValueOnce({ data: { order_id: 9, status: 'COMPLETED', payment_type: 'balance' } })

  await paymentAPI.balancePayOrder({ order_type: 'subscription', plan_id: 7 })

  expect(mockPost).toHaveBeenCalledWith('/payment/orders/balance-pay', {
    order_type: 'subscription',
    plan_id: 7,
  })
})
```

- [ ] **Step 2: 修改类型**

在 `frontend/src/types/payment.ts` 修改：

```ts
export type PaymentType = 'alipay' | 'wxpay' | 'alipay_direct' | 'wxpay_direct' | 'stripe' | 'easypay' | 'airwallex' | 'balance'

export interface BalancePayOrderRequest {
  order_type: 'subscription' | 'traffic_pack'
  plan_id?: number
  traffic_pack_id?: number
}
```

- [ ] **Step 3: 增加 API 方法**

在 `frontend/src/api/payment.ts` 增加：

```ts
balancePayOrder(data: BalancePayOrderRequest) {
  return apiClient.post<CreateOrderResult>('/payment/orders/balance-pay', data)
},
```

并在 import 中加入 `BalancePayOrderRequest`。

- [ ] **Step 4: 运行测试**

Run:

```bash
cd frontend && pnpm vitest run src/api/__tests__/payment.spec.ts
```

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add frontend/src/types/payment.ts frontend/src/api/payment.ts frontend/src/api/__tests__/payment.spec.ts
git commit -m "feat: add balance pay frontend api"
```

## Task 6: `/purchase` 余额充值卡与充值确认

**Files:**

- Modify: `frontend/src/views/user/PaymentView.vue`
- Modify: `frontend/src/components/payment/PaymentMethodSelector.vue`
- Modify: `frontend/src/components/payment/providerConfig.ts`
- Modify: `frontend/src/views/user/__tests__/PaymentView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

- [ ] **Step 1: 写 UI 失败测试**

在 `frontend/src/views/user/__tests__/PaymentView.spec.ts` 增加：

```ts
it('shows balance recharge as the first purchase product', async () => {
  getCheckoutInfo.mockResolvedValueOnce(checkoutInfoWithFiveZPayPlansFixture())

  const wrapper = mount(PaymentView, { global: paymentViewStubs() })
  await flushPromises()

  const cards = wrapper.findAll('[data-testid="purchase-product-card"]')
  expect(cards[0].text()).toContain('余额充值')
  expect(cards[0].text()).toContain('¥1 起')
})

it('opens recharge confirm with default amount 1 and alipay only', async () => {
  getCheckoutInfo.mockResolvedValueOnce(checkoutInfoWithFiveZPayPlansFixture())

  const wrapper = mount(PaymentView, { global: paymentViewStubs() })
  await flushPromises()
  await wrapper.findAll('[data-testid="purchase-product-card"]')[0].trigger('click')

  expect(wrapper.text()).toContain('余额充值')
  expect((wrapper.get('[data-testid="balance-recharge-amount"]').element as HTMLInputElement).value).toBe('1')
  expect(wrapper.text()).toContain('payment.methods.alipay')
  expect(wrapper.text()).not.toContain('payment.methods.wxpay')
  expect(wrapper.text()).not.toContain('payment.methods.stripe')
})

it.each(['0', '1.5', '101', ''])('rejects invalid recharge amount %s', async (value) => {
  getCheckoutInfo.mockResolvedValueOnce(checkoutInfoWithFiveZPayPlansFixture())

  const wrapper = mount(PaymentView, { global: paymentViewStubs() })
  await flushPromises()
  await wrapper.findAll('[data-testid="purchase-product-card"]')[0].trigger('click')
  await wrapper.get('[data-testid="balance-recharge-amount"]').setValue(value)

  expect(wrapper.get('[data-testid="balance-recharge-submit"]').attributes('disabled')).toBeDefined()
  expect(wrapper.text()).toContain('充值金额必须是 1-100 的整数')
})
```

- [ ] **Step 2: 增加前端状态**

在 `PaymentView.vue` 中把 `paymentPhase` 改为：

```ts
const paymentPhase = ref<'select' | 'recharge' | 'paying'>('select')
const rechargeAmount = ref('1')
const rechargeError = computed(() => {
  if (!/^\d+$/.test(rechargeAmount.value.trim())) return t('payment.recharge.invalidAmount')
  const value = Number(rechargeAmount.value)
  if (value < 1 || value > 100) return t('payment.recharge.invalidAmount')
  return ''
})
const validRechargeAmount = computed(() => rechargeError.value ? 0 : Number(rechargeAmount.value))
```

把 `PurchaseProduct` 扩展为：

```ts
type BalanceRechargePurchaseProduct = { id: 'balance-recharge'; type: 'balance_recharge'; product: PurchaseProductCardModel }
type PurchaseProduct = BalanceRechargePurchaseProduct | SubscriptionPurchaseProduct | TrafficPackPurchaseProduct
```

增加：

```ts
function buildBalanceRechargeProduct(): BalanceRechargePurchaseProduct {
  return {
    id: 'balance-recharge',
    type: 'balance_recharge',
    product: {
      testId: 'purchase-product-card',
      title: t('payment.recharge.title'),
      priceText: '¥1 起',
      buttonText: t('payment.recharge.button'),
      detailRows: [
        { label: t('payment.recharge.usage'), value: t('payment.recharge.usageValue') },
        { label: t('payment.recharge.arrival'), value: t('payment.recharge.arrivalValue') },
        { label: t('payment.recharge.fee'), value: t('payment.recharge.noFee') },
      ],
    },
  }
}
```

`purchaseProducts` 改为充值卡在第一张：

```ts
const purchaseProducts = computed<PurchaseProduct[]>(() => [
  buildBalanceRechargeProduct(),
  ...checkout.value.plans.map((plan, index) => buildSubscriptionProduct(plan, index)),
  ...checkout.value.traffic_packs.map(pack => buildTrafficPackProduct(pack)),
])
```

- [ ] **Step 3: 增加充值确认模板**

在商品确认分支前增加 `paymentPhase === 'recharge'`：

```vue
<template v-if="paymentPhase === 'recharge'">
  <div class="card p-5">
    <h3 class="text-lg font-bold text-gray-900 dark:text-white">{{ t('payment.recharge.title') }}</h3>
    <label class="mt-4 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('payment.recharge.amount') }}</label>
    <input
      v-model="rechargeAmount"
      data-testid="balance-recharge-amount"
      class="input mt-2"
      inputmode="numeric"
      autocomplete="off"
    />
    <p v-if="rechargeError" class="mt-2 text-sm text-red-600 dark:text-red-400">{{ rechargeError }}</p>
  </div>
  <div class="card p-6">
    <PaymentMethodSelector :methods="rechargeMethodOptions" selected="alipay" @select="selectedMethod = 'alipay'" />
  </div>
  <div class="card p-6">
    <div class="flex justify-between text-sm">
      <span class="text-gray-500 dark:text-gray-400">{{ t('payment.recharge.amount') }}</span>
      <span class="text-gray-900 dark:text-white">¥{{ validRechargeAmount.toFixed(2) }}</span>
    </div>
    <div class="mt-2 flex justify-between border-t border-gray-200 pt-2 text-sm dark:border-dark-600">
      <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.actualPay') }}</span>
      <span class="text-lg font-bold text-gray-900 dark:text-gray-100">¥{{ validRechargeAmount.toFixed(2) }}</span>
    </div>
  </div>
  <button data-testid="balance-recharge-submit" class="btn btn-alipay w-full py-3 text-base font-medium" :disabled="!!rechargeError || submitting" @click="confirmRecharge">
    {{ t('payment.recharge.confirm', { amount: validRechargeAmount.toFixed(0) }) }}
  </button>
  <button class="btn btn-secondary w-full" @click="backToSubscriptionList">{{ t('common.back') }}</button>
</template>
```

- [ ] **Step 4: 实现充值下单**

在 `PaymentView.vue` 增加：

```ts
const rechargeMethodOptions = computed<PaymentMethodOption[]>(() => {
  const alipay = visibleExternalMethods.value.alipay
  return alipay ? [{ type: 'alipay', fee_rate: 0, available: alipay.available !== false }] : []
})

function openRechargeConfirm(defaultAmount = 1) {
  selectedPlan.value = null
  selectedTrafficPack.value = null
  paymentPhase.value = 'recharge'
  selectedMethod.value = 'alipay'
  rechargeAmount.value = String(Math.min(100, Math.max(1, Math.ceil(defaultAmount))))
}

async function confirmRecharge() {
  if (rechargeError.value || submitting.value) return
  await createOrder(validRechargeAmount.value, 'balance', undefined, { paymentType: 'alipay' })
}
```

`selectPurchaseProduct` 增加：

```ts
if (item.type === 'balance_recharge') {
  openRechargeConfirm(1)
  return
}
```

`backToSubscriptionList` 设置：

```ts
paymentPhase.value = 'select'
```

- [ ] **Step 5: 增加 i18n**

在 `frontend/src/i18n/locales/zh.ts` 的 `payment` 下增加：

```ts
recharge: {
  title: '余额充值',
  button: '立即充值',
  amount: '充值金额',
  usage: '用途',
  usageValue: '购买套餐/流量包',
  arrival: '到账',
  arrivalValue: '实时到账',
  fee: '手续费',
  noFee: '无',
  invalidAmount: '充值金额必须是 1-100 的整数',
  confirm: '确认支付 ¥{amount}',
},
```

英文文件用等价英文文本，不影响中文主流程。

- [ ] **Step 6: 运行测试**

Run:

```bash
cd frontend && pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts
```

Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add frontend/src/views/user/PaymentView.vue frontend/src/views/user/__tests__/PaymentView.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: restore rmb balance recharge checkout"
```

## Task 7: 产品确认页余额支付与余额不足引导

**Files:**

- Modify: `frontend/src/views/user/PaymentView.vue`
- Modify: `frontend/src/views/user/__tests__/PaymentView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

- [ ] **Step 1: 写余额支付 UI 测试**

在 `PaymentView.spec.ts` 增加：

```ts
it('shows alipay and balance only for product checkout', async () => {
  getCheckoutInfo.mockResolvedValueOnce(checkoutInfoWithFiveZPayPlansFixture())

  const wrapper = mount(PaymentView, { global: paymentViewStubs() })
  await flushPromises()
  await wrapper.findAll('[data-testid="purchase-product-card"]')[1].trigger('click')

  expect(wrapper.text()).toContain('payment.methods.alipay')
  expect(wrapper.text()).toContain('payment.methods.balance')
  expect(wrapper.text()).not.toContain('payment.methods.wxpay')
  expect(wrapper.text()).not.toContain('payment.methods.stripe')
  expect(wrapper.text()).not.toContain('payment.methods.airwallex')
})

it('opens recharge confirm with rounded shortage when balance is insufficient', async () => {
  getCheckoutInfo.mockResolvedValueOnce(checkoutInfoWithFiveZPayPlansFixture())
  const wrapper = mount(PaymentView, { global: paymentViewStubs({ userBalance: 10 }) })
  await flushPromises()
  await wrapper.findAll('[data-testid="purchase-product-card"]')[1].trigger('click')
  await wrapper.find('[data-testid="payment-method-balance"]').trigger('click')
  await wrapper.find('[data-testid="subscription-submit"]').trigger('click')

  expect(wrapper.text()).toContain('余额充值')
  expect((wrapper.get('[data-testid="balance-recharge-amount"]').element as HTMLInputElement).value).toBe('20')
})
```

- [ ] **Step 2: 添加余额 method option**

先让 `PaymentMethodSelector.vue` 能稳定展示和测试余额方式。给按钮增加 test id：

```vue
:data-testid="`payment-method-${method.type}`"
```

把图标区域改为余额使用人民币符号，其它方式继续使用现有图片：

```vue
<span
  v-if="method.type === 'balance'"
  class="flex h-7 w-7 items-center justify-center rounded-full bg-gray-900 text-sm font-bold text-white dark:bg-gray-100 dark:text-gray-900"
>
  ¥
</span>
<img v-else :src="methodIcon(method.type)" :alt="t(`payment.methods.${method.type}`)" class="h-7 w-7 object-contain" />
```

在 `providerConfig.ts` 把余额排在支付宝后面：

```ts
export const METHOD_ORDER = ['alipay', 'balance', 'alipay_direct', 'wxpay', 'wxpay_direct', 'stripe', 'airwallex'] as const
```

在 `PaymentView.vue` 增加：

```ts
const visibleExternalMethods = computed(() => getUserExternalPaymentMethods(checkout.value.methods))
const productMethodOptions = computed<PaymentMethodOption[]>(() => {
  const methods: PaymentMethodOption[] = []
  const alipay = visibleExternalMethods.value.alipay
  if (alipay) methods.push({ type: 'alipay', fee_rate: alipay.fee_rate ?? 0, available: alipay.available !== false })
  methods.push({ type: 'balance', fee_rate: feeRate.value, available: true })
  return methods
})

function userBalanceAmount(): number {
  return Number(authStore.user?.balance || 0)
}

function ensureBalanceEnough(totalAmount: number): boolean {
  if (selectedMethod.value !== 'balance') return true
  const shortage = Math.max(0, totalAmount - userBalanceAmount())
  if (shortage <= 0) return true
  openRechargeConfirm(shortage)
  if (shortage > 100) {
    appStore.showWarning(t('payment.recharge.maxOnce'))
  }
  return false
}
```

`subMethodOptions` 和 `trafficPackMethodOptions` 改为基于 `productMethodOptions`，并让余额不受外部 provider 单笔限额限制。

- [ ] **Step 3: 调用余额支付 API**

在 `confirmSubscribe` 改为：

```ts
async function confirmSubscribe() {
  if (!selectedPlan.value || submitting.value) return
  const total = subTotalAmount.value
  if (!ensureBalanceEnough(total)) return
  if (selectedMethod.value === 'balance') {
    await balancePayProduct({ order_type: 'subscription', plan_id: selectedPlan.value.id })
    return
  }
  await createOrder(selectedPlan.value.price, 'subscription', selectedPlan.value.id)
}
```

在 `confirmTrafficPack` 改为：

```ts
async function confirmTrafficPack() {
  if (!selectedTrafficPack.value || submitting.value) return
  const total = trafficPackTotalAmount.value
  if (!ensureBalanceEnough(total)) return
  if (selectedMethod.value === 'balance') {
    await balancePayProduct({ order_type: 'traffic_pack', traffic_pack_id: selectedTrafficPack.value.id })
    return
  }
  await createOrder(selectedTrafficPack.value.price, 'traffic_pack', undefined, { trafficPackId: selectedTrafficPack.value.id })
}
```

新增：

```ts
async function balancePayProduct(payload: BalancePayOrderRequest) {
  submitting.value = true
  try {
    const result = await paymentAPI.balancePayOrder(payload)
    appStore.showSuccess(t('payment.balancePay.success'))
    await authStore.refreshUser()
    if (payload.order_type === 'subscription') await subscriptionStore.fetchActiveSubscriptions(true)
    if (payload.order_type === 'traffic_pack') await reloadCheckoutInfo()
    backToSubscriptionList()
    paymentState.value = {
      ...emptyPaymentState(),
      orderId: result.data.order_id,
      amount: result.data.amount,
      payAmount: result.data.pay_amount,
      paymentType: 'balance',
      orderType: payload.order_type,
      outTradeNo: result.data.out_trade_no || '',
      currency: 'CNY',
    }
  } catch (err: unknown) {
    const reason = typeof err === 'object' && err && 'reason' in err ? String(err.reason) : ''
    if (reason === 'BALANCE_INSUFFICIENT') {
      openRechargeConfirm(1)
      return
    }
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('payment.result.failed')))
  } finally {
    submitting.value = false
  }
}
```

- [ ] **Step 4: 增加余额支付文案**

在 `zh.ts`：

```ts
methods: {
  balance: '余额',
},
balancePay: {
  success: '余额支付成功',
},
recharge: {
  maxOnce: '单次最多充值 100 元，请分多次充值',
}
```

保留原有 `methods` 其他键。

- [ ] **Step 5: 运行测试**

Run:

```bash
cd frontend && pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts
```

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add frontend/src/views/user/PaymentView.vue frontend/src/components/payment/PaymentMethodSelector.vue frontend/src/components/payment/providerConfig.ts frontend/src/views/user/__tests__/PaymentView.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: add balance checkout for products"
```

## Task 8: 订单表、后台筛选与人民币展示

**Files:**

- Modify: `frontend/src/components/payment/OrderTable.vue`
- Modify: `frontend/src/components/admin/payment/AdminOrderTable.vue`
- Modify: `frontend/src/components/admin/payment/AdminOrderDetail.vue`
- Modify: `frontend/src/components/admin/payment/AdminRefundDialog.vue`
- Modify: `frontend/src/components/payment/PaymentQRDialog.vue`
- Modify: `frontend/src/components/payment/PaymentStatusPanel.vue`
- Modify: `frontend/src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

- [ ] **Step 1: 写订单显示测试**

在 `frontend/src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts` 增加：

```ts
it('shows balance payment and traffic pack order type with CNY amounts', () => {
  const order = {
    id: 1,
    user_id: 9,
    amount: 79,
    pay_amount: 79.79,
    currency: 'CNY',
    fee_rate: 1,
    payment_type: 'balance',
    out_trade_no: 'sub2_balance',
    status: 'COMPLETED',
    order_type: 'traffic_pack',
    created_at: '2026-07-07T00:00:00Z',
    expires_at: '2026-07-07T00:00:00Z',
    refund_amount: 0,
  } as PaymentOrder

  const wrapper = mount(AdminOrderTable, {
    props: { orders: [order], loading: false, page: 1, pageSize: 20, total: 1 },
    global: adminPaymentTableStubs(),
  })

  expect(wrapper.text()).toContain('¥79.79')
  expect(wrapper.text()).toContain('余额')
  expect(wrapper.text()).toContain('流量包')
})
```

- [ ] **Step 2: 修正已入账金额符号**

在 `OrderTable.vue` / `AdminOrderTable.vue` / `AdminOrderDetail.vue` / `AdminRefundDialog.vue` / `PaymentQRDialog.vue` / `PaymentStatusPanel.vue` 中把：

```ts
const creditedAmountSymbol = currencySymbol('USD')
```

替换为：

```ts
function creditedAmountSymbol(order?: PaymentOrder | null): string {
  if (!order || order.order_type === 'traffic_pack') return currencySymbol('USD')
  return currencySymbol(order.currency || 'CNY')
}
```

模板中将 `{{ creditedAmountSymbol }}` 改为 `{{ creditedAmountSymbol(row) }}` 或 `{{ creditedAmountSymbol(order) }}`。

- [ ] **Step 3: 补后台筛选**

在 `AdminOrderTable.vue` 的 `paymentTypeFilterOptions` 增加：

```ts
{ value: 'balance', label: t('payment.methods.balance') },
```

在 `orderTypeFilterOptions` 增加：

```ts
{ value: 'traffic_pack', label: t('payment.admin.trafficPackOrder') },
```

- [ ] **Step 4: 补文案**

在 `zh.ts`：

```ts
payment: {
  methods: {
    balance: '余额',
  },
  admin: {
    traffic_packOrder: '流量包',
  },
  orders: {
    balanceRechargeAmount: '充值金额',
    creditedAmount: '入账金额',
  }
}
```

在 `en.ts` 增加对应英文。

- [ ] **Step 5: 运行测试**

Run:

```bash
cd frontend && pnpm vitest run src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts
```

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add frontend/src/components/payment/OrderTable.vue frontend/src/components/admin/payment/AdminOrderTable.vue frontend/src/components/admin/payment/AdminOrderDetail.vue frontend/src/components/admin/payment/AdminRefundDialog.vue frontend/src/components/payment/PaymentQRDialog.vue frontend/src/components/payment/PaymentStatusPanel.vue frontend/src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "fix: display balance orders in rmb"
```

## Task 9: 余额相关文案统一为人民币

**Files:**

- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/views/admin/affiliates/AdminAffiliateRecordsTable.vue`
- Modify: `frontend/src/views/admin/RedeemView.vue`
- Modify: `frontend/src/components/user/profile/ProfileBalanceNotifyCard.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`
- Modify: `frontend/src/components/user/profile/__tests__/ProfileInfoCard.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts`

- [ ] **Step 1: 写核心人民币文案测试**

在 `frontend/src/components/user/profile/__tests__/ProfileInfoCard.spec.ts` 修改余额断言：

```ts
expect(wrapper.get('[data-testid="profile-overview-metric-balance"]').text()).toContain('¥10.00')
```

在 `frontend/src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts` 增加：

```ts
expect(wrapper.text()).toContain('余额金额（人民币）')
```

- [ ] **Step 2: 改 Header 余额符号**

在 `AppHeader.vue` 中把两处：

```vue
${{ user.balance?.toFixed(2) || '0.00' }}
```

改为：

```vue
¥{{ user.balance?.toFixed(2) || '0.00' }}
```

- [ ] **Step 3: 改返利页面金额符号**

在 `AdminAffiliateRecordsTable.vue` 中把：

```vue
:value="'$' + formatAmount(selectedOverview.available_quota)"
:value="'$' + formatAmount(selectedOverview.history_quota)"
```

改为：

```vue
:value="'¥' + formatAmount(selectedOverview.available_quota)"
:value="'¥' + formatAmount(selectedOverview.history_quota)"
```

- [ ] **Step 4: 改兑换码余额提示**

在 `zh.ts` 中把：

```ts
balanceHint: '余额金额（美元）'
```

改为：

```ts
balanceHint: '余额金额（人民币）'
```

并清理与 `balance_recharge_multiplier` 相关的用户侧文案，固定表达为“人民币余额按 1:1 入账”。

- [ ] **Step 5: 运行针对性测试**

Run:

```bash
cd frontend && pnpm vitest run src/components/user/profile/__tests__/ProfileInfoCard.spec.ts src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts
```

Expected: PASS。

- [ ] **Step 6: 全局搜索复核**

Run:

```bash
rg -n "\\$\\{\\{ user\\.balance|余额金额（美元）|availableQuota.*'\\$'|historyQuota.*'\\$'|balance_recharge_multiplier" frontend/src
```

Expected: 不再出现用户余额人民币口径冲突；`balance_recharge_multiplier` 只允许保留在后端兼容设置、类型字段或后台配置字段中。

- [ ] **Step 7: 提交**

```bash
git add frontend/src/components/layout/AppHeader.vue frontend/src/views/admin/affiliates/AdminAffiliateRecordsTable.vue frontend/src/views/admin/RedeemView.vue frontend/src/components/user/profile/ProfileBalanceNotifyCard.vue frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts frontend/src/components/user/profile/__tests__/ProfileInfoCard.spec.ts frontend/src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts
git commit -m "fix: label user balance as rmb"
```

## Task 10: 全量验证、上下文归档与 AGENTS 更新

**Files:**

- Create: `docs/ai/context/YYYYMMDD-HHMMSS-rmb-balance-payment-affiliate-rebate-result_CN.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: 后端完整验证**

Run:

```bash
cd backend && go test -count=1 -tags=unit ./internal/payment ./internal/service ./internal/handler ./internal/server
cd backend && go test -count=1 ./migrations
```

Expected: PASS。

- [ ] **Step 2: 前端完整验证**

Run:

```bash
cd frontend && pnpm typecheck
cd frontend && pnpm vitest run src/api/__tests__/payment.spec.ts src/components/payment/__tests__/paymentFlow.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts
```

Expected: PASS。

- [ ] **Step 3: 构建验证**

Run:

```bash
cd frontend && pnpm build
cd backend && go test -count=1 ./cmd/server
```

Expected: PASS。

- [ ] **Step 4: 本地手工验收**

启动本地应用后验收：

```bash
curl -sS http://127.0.0.1:8080/health
```

Expected: 返回健康状态。

浏览器验收：

- `/purchase` 第一张为余额充值卡。
- 余额充值默认 1 元，0/1.5/101/空值均禁用按钮。
- 产品确认页只显示支付宝与余额。
- 余额不足进入充值确认页。
- 余额支付成功后 `/orders` 显示 `payment_type=balance`、状态 `COMPLETED`、金额 `¥`。

- [ ] **Step 5: 写结果上下文**

创建 `docs/ai/context/YYYYMMDD-HHMMSS-rmb-balance-payment-affiliate-rebate-result_CN.md`，内容包含：

```markdown
# 人民币余额支付与邀请返利重构结果

## 已完成

- 余额充值：支付宝-only，1-100 元整数，默认 1 元，`amount=pay_amount`，`fee_rate=0`。
- 余额支付：`payment_type=balance`，只购买套餐/流量包，扣 `pay_amount`，不产生邀请返利。
- 邀请返利：支付宝完成订单按 `amount` 返利，默认 8%、冻结 24 小时、有效期 365 天、单被邀请人上限 ¥100。
- 订单展示：用户与后台显示余额支付、流量包订单和人民币金额。

## 验证

- `cd backend && go test -count=1 -tags=unit ./internal/payment ./internal/service ./internal/handler ./internal/server`
- `cd backend && go test -count=1 ./migrations`
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm vitest run src/api/__tests__/payment.spec.ts src/components/payment/__tests__/paymentFlow.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts`

## 运行态提醒

- 发布公网前先备份 `sub2api-candidate-postgres` 与 `sub2api-candidate-redis`。
- 迁移 `160_rmb_balance_payment_affiliate_defaults.sql` 会显式覆盖返利 settings。
```

- [ ] **Step 6: 更新 AGENTS.md**

在 `AGENTS.md` 的“最高优先级定论”顶部新增一条：

```markdown
- 2026-07-07 已写人民币余额支付与邀请返利重构实施计划：计划按 TDD 拆为订单类型/金额口径、外部支付支付宝-only、余额支付事务、邀请返利默认值、前端充值/余额支付、订单展示、人民币文案和全量验收；执行时必须先修正 `users.balance` 透支扣减风险，余额购买只能条件扣款成功后发货。计划见 `docs/ai/context/20260707-230444-rmb-balance-payment-affiliate-rebate-implementation-plan_CN.md`。
```

- [ ] **Step 7: 检查上下文文件与敏感信息**

Run:

```bash
git diff --check
git ls-files --others --exclude-standard docs/ai/context
rg -n "sk-|AKIA|smtp_password|HMAC|secret|token" docs/ai/context/20260707-230444-rmb-balance-payment-affiliate-rebate-implementation-plan_CN.md docs/ai/context/*rmb-balance-payment-affiliate-rebate-result_CN.md
```

Expected: `git diff --check` 无输出；敏感信息搜索无真实密钥。

- [ ] **Step 8: 提交**

```bash
git add docs/ai/context/20260707-230444-rmb-balance-payment-affiliate-rebate-implementation-plan_CN.md docs/ai/context/*rmb-balance-payment-affiliate-rebate-result_CN.md AGENTS.md
git commit -m "docs: plan rmb balance payment rebuild"
```

## 自检

- 设计覆盖：余额人民币口径、余额充值、余额支付套餐/流量包、支付宝直购、返利 8%/24h/365d/¥100、返利基数 `amount`、余额支付不返利、`order_type` 白名单、后台 `traffic_pack` 筛选均有对应任务。
- 占位扫描：本文不包含未决占位、空泛实现步骤或要求执行者自行补全的段落。
- 类型一致：后端订单类型使用 `payment.OrderTypeBalance/subscription/traffic_pack`；内部余额支付方式使用 `payment.TypeBalance`；前端 `PaymentType` 增加 `'balance'`，外部下单仍只发 `'alipay'`。
- 风险隔离：余额购买不复用允许透支的 `UserRepository.DeductBalance`，必须在 `PaymentService` 事务内使用 `WHERE balance >= pay_amount` 条件扣款。
