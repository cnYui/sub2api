//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestRequestRefundRejectsLegacyOrderType(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-legacy@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-legacy-user").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetAllowUserRefund(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode("REFUND-LEGACY-ORDER").
		SetOutTradeNo("sub2_refund_legacy_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-legacy-refund").
		SetOrderType("legacy_balance").
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
	}

	err = svc.RequestRefund(ctx, order.ID, user.ID, "legacy order")
	require.Error(t, err)
	require.Equal(t, "INVALID_ORDER_TYPE", infraerrors.Reason(err))
}

func TestValidateRealPaidBalancePackageOrderRequiresUserPayment(t *testing.T) {
	now := time.Now().UTC()
	base := func() *dbent.PaymentOrder {
		return &dbent.PaymentOrder{
			PaymentType:    payment.TypeAlipay,
			PaymentTradeNo: "trade-real-payment",
			OrderType:      payment.OrderTypeBalanceSubscription,
			PayAmount:      29,
			PaidAt:         &now,
		}
	}

	tests := []struct {
		name       string
		order      func() *dbent.PaymentOrder
		wantReason string
	}{
		{
			name: "administrator grant",
			order: func() *dbent.PaymentOrder {
				o := base()
				o.PaymentType = payment.PaymentTypeAdminGrant
				return o
			},
			wantReason: "ADMIN_GRANTED_ORDER",
		},
		{
			name: "zero payment amount",
			order: func() *dbent.PaymentOrder {
				o := base()
				o.PayAmount = 0
				return o
			},
			wantReason: "REFUND_REQUIRES_REAL_PAYMENT",
		},
		{
			name: "missing paid timestamp",
			order: func() *dbent.PaymentOrder {
				o := base()
				o.PaidAt = nil
				return o
			},
			wantReason: "REFUND_REQUIRES_REAL_PAYMENT",
		},
		{
			name: "missing provider trade number",
			order: func() *dbent.PaymentOrder {
				o := base()
				o.PaymentTradeNo = ""
				return o
			},
			wantReason: "REFUND_REQUIRES_REAL_PAYMENT",
		},
		{
			name: "redeem code payment type",
			order: func() *dbent.PaymentOrder {
				o := base()
				o.PaymentType = "redeem_code"
				return o
			},
			wantReason: "REFUND_REQUIRES_REAL_PAYMENT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRealPaidBalancePackageOrder(tt.order())
			require.Error(t, err)
			require.Equal(t, tt.wantReason, infraerrors.Reason(err))
		})
	}
}

func TestValidateRealPaidBalancePackageOrderAcceptsUserPayment(t *testing.T) {
	paidAt := time.Now().UTC()
	err := validateRealPaidBalancePackageOrder(&dbent.PaymentOrder{
		PaymentType:    payment.TypeAlipay,
		PaymentTradeNo: "trade-real-payment",
		OrderType:      payment.OrderTypeBalanceSubscription,
		PayAmount:      29,
		PaidAt:         &paidAt,
	})
	require.NoError(t, err)
}

func TestPrepareRefundRejectsLegacyGuessedProviderInstance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-legacy-admin@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-legacy-admin-user").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-admin-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetAllowUserRefund(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(188).
		SetPayAmount(188).
		SetFeeRate(0).
		SetRechargeCode("REFUND-LEGACY-ADMIN-ORDER").
		SetOutTradeNo("sub2_refund_legacy_admin_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-legacy-admin-refund").
		SetOrderType("legacy_balance").
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
	}

	plan, err := svc.PrepareRefund(ctx, order.ID, "")
	require.Nil(t, plan)
	require.Error(t, err)
	require.Equal(t, "INVALID_ORDER_TYPE", infraerrors.Reason(err))
}

func TestGwRefundRejectsAlipayMerchantIdentitySnapshotMismatch(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-snapshot-mismatch@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-snapshot-mismatch-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-mismatch-instance").
		SetConfig(encryptWebhookProviderConfig(t, map[string]string{
			"appId":      "runtime-alipay-app",
			"privateKey": "runtime-private-key",
		})).
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(inst.ID, 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode("REFUND-SNAPSHOT-MISMATCH-ORDER").
		SetOutTradeNo("sub2_refund_snapshot_mismatch_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-refund-snapshot-mismatch").
		SetOrderType("legacy_balance").
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		SetProviderSnapshot(map[string]any{
			"schema_version":       2,
			"provider_instance_id": instID,
			"provider_key":         payment.TypeAlipay,
			"merchant_app_id":      "expected-alipay-app",
		}).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient:    client,
		loadBalancer: newWebhookProviderTestLoadBalancer(client),
	}

	_, err = svc.gwRefund(ctx, &RefundPlan{
		OrderID:       order.ID,
		Order:         order,
		RefundAmount:  order.Amount,
		GatewayAmount: order.Amount,
		Reason:        "snapshot mismatch",
	})
	require.ErrorContains(t, err, "alipay app_id mismatch")
}

func TestCalculateGatewayRefundAmountUsesCurrencyPrecision(t *testing.T) {
	require.InDelta(t, 6.173, calculateGatewayRefundAmount(100, 12.345, 50, "KWD"), 1e-12)
	require.InDelta(t, 12.345, calculateGatewayRefundAmount(100, 12.345, 100, "KWD"), 1e-12)
	require.InDelta(t, 52, calculateGatewayRefundAmount(100, 103, 50, "JPY"), 1e-12)
}

func TestResolveGatewayRefundAmountUsesQuoteForZPay(t *testing.T) {
	order := &dbent.PaymentOrder{Amount: 49, PayAmount: 49.49}
	plan := &RefundPlan{Order: order, RefundAmount: 1.99, GatewayAmount: 2.01}

	require.Equal(t, 1.99, resolveGatewayRefundAmount(plan, zpayRefundProviderTestDouble{}))
	require.Equal(t, 2.01, resolveGatewayRefundAmount(plan, refundProviderTestDouble{}))
}

func TestFormatGatewayRefundAmountUsesOrderCurrency(t *testing.T) {
	order := &dbent.PaymentOrder{
		ProviderSnapshot: map[string]any{
			"currency": "KWD",
		},
	}

	require.Equal(t, "12.345", formatGatewayRefundAmount(12.345, order))
}

func TestValidateRefundProviderResponseAcceptsPending(t *testing.T) {
	require.NoError(t, validateRefundProviderResponse(&payment.RefundResponse{Status: payment.ProviderStatusPending}))
	require.NoError(t, validateRefundProviderResponse(&payment.RefundResponse{Status: payment.ProviderStatusSuccess}))
	require.Error(t, validateRefundProviderResponse(&payment.RefundResponse{Status: payment.ProviderStatusFailed}))
	require.Error(t, validateRefundProviderResponse(nil))
}

func TestMarkRefundOKRevokesBalancePackageRemainingCredit(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("balance-package-refund@example.com").
		SetPasswordHash("hash").
		SetUsername("balance-package-refund-user").
		Save(ctx)
	require.NoError(t, err)

	now := time.Now().UTC()
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(49).
		SetPayAmount(49.49).
		SetFeeRate(0.01).
		SetRechargeCode("BALANCE-PACKAGE-REFUND").
		SetOutTradeNo("sub2_balance_package_refund").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-balance-package-refund").
		SetOrderType(payment.OrderTypeBalanceSubscription).
		SetStatus(OrderStatusRefunding).
		SetExpiresAt(now.Add(time.Hour)).
		SetPaidAt(now).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	packageRecord, err := client.UserBalancePackage.Create().
		SetUserID(user.ID).
		SetPlanID(1).
		SetPaymentOrderID(order.ID).
		SetWeeklyCreditUsd(128).
		SetRemainingUsd(6.87332064).
		SetCreditedCount(4).
		SetRefreshCount(4).
		SetRefreshIntervalDays(7).
		SetStartsAt(now.Add(-21 * 24 * time.Hour)).
		SetNextCreditAt(now.Add(7 * 24 * time.Hour)).
		SetExpiresAt(now.Add(7 * 24 * time.Hour)).
		SetStatus("completed").
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{entClient: client}
	result, err := svc.markRefundOk(ctx, &RefundPlan{
		OrderID: order.ID, Order: order, RefundAmount: 1.99, GatewayAmount: 1.99, Reason: "partial refund",
	})
	require.NoError(t, err)
	require.True(t, result.Success)

	updatedPackage, err := client.UserBalancePackage.Get(ctx, packageRecord.ID)
	require.NoError(t, err)
	require.Equal(t, "refunded", updatedPackage.Status)
	require.Zero(t, updatedPackage.RemainingUsd)
	require.Nil(t, updatedPackage.NextCreditAt)
}

func TestWriteAuditLogUpdatesDuplicateAction(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}

	svc.writeAuditLog(ctx, 42, "REFUND_FAILED", "user", map[string]any{"detail": "first"})
	svc.writeAuditLog(ctx, 42, "REFUND_FAILED", "user", map[string]any{"detail": "last"})

	logs, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ("42"), paymentauditlog.ActionEQ("REFUND_FAILED")).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Contains(t, logs[0].Detail, "last")
}

func TestQueryAndFinalizeRefundFinalizesProviderStatuses(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     string
		wantStatus string
		wantDeduct float64
	}{
		{name: "success", status: payment.ProviderStatusSuccess, wantStatus: OrderStatusRefunded, wantDeduct: 100},
		{name: "failed", status: payment.ProviderStatusFailed, wantStatus: OrderStatusRefundFailed},
		{name: "pending", status: payment.ProviderStatusPending, wantStatus: OrderStatusRefundPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			order := createPendingRefundOrderForTest(t, ctx, client, "query-finalize-"+tc.name)

			var deducted float64
			svc := &PaymentService{
				entClient:    client,
				loadBalancer: &captureLoadBalancer{},
				userRepo: &mockUserRepo{deductBalanceFn: func(ctx context.Context, id int64, amount float64) error {
					deducted += amount
					return nil
				}},
			}
			restore := replacePaymentProviderFactoryForTest(t, &refundQueryProviderTestDouble{
				refundResponse: &payment.RefundResponse{RefundID: "rf_test", Status: tc.status},
			})
			defer restore()

			result, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tc.status == payment.ProviderStatusSuccess, result.Success)
			require.Equal(t, tc.wantDeduct, deducted)

			reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, reloaded.Status)
		})
	}
}

func TestQueryAndFinalizeRefundUnsupportedProviderReturnsClearError(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderForTest(t, ctx, client, "query-finalize-unsupported")
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}}
	restore := replacePaymentProviderFactoryForTest(t, refundProviderTestDouble{})
	defer restore()

	result, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "REFUND_QUERY_UNSUPPORTED", infraerrors.Reason(err))
}

func createPendingRefundOrderForTest(t *testing.T, ctx context.Context, client *dbent.Client, suffix string) *dbent.PaymentOrder {
	t.Helper()

	user, err := client.User.Create().
		SetEmail(suffix + "@example.com").
		SetPasswordHash("hash").
		SetUsername(suffix).
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeStripe).
		SetName(suffix + "-provider").
		SetConfig("{}").
		SetSupportedTypes("stripe").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-" + suffix).
		SetOutTradeNo("sub2_" + suffix).
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("pi_" + suffix).
		SetOrderType("legacy_balance").
		SetStatus(OrderStatusRefundPending).
		SetRefundAmount(100).
		SetRefundReason("pending refund").
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(order.ID, 10)).
		SetAction("REFUND_PENDING").
		SetOperator("admin").
		SetDetail(`{"refundID":"rf_test","deductionRollbackOK":true}`).
		Save(ctx)
	require.NoError(t, err)
	return order
}

func replacePaymentProviderFactoryForTest(t *testing.T, prov payment.Provider) func() {
	t.Helper()
	original := createPaymentProviderFromInstance
	createPaymentProviderFromInstance = func(providerKey, instanceID string, config map[string]string) (payment.Provider, error) {
		return prov, nil
	}
	return func() { createPaymentProviderFromInstance = original }
}

type refundProviderTestDouble struct{}

func (refundProviderTestDouble) Name() string { return "refund-test" }
func (refundProviderTestDouble) ProviderKey() string {
	return payment.TypeStripe
}
func (refundProviderTestDouble) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeStripe}
}
func (refundProviderTestDouble) CreatePayment(context.Context, payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	return nil, nil
}
func (refundProviderTestDouble) QueryOrder(context.Context, string) (*payment.QueryOrderResponse, error) {
	return nil, nil
}
func (refundProviderTestDouble) VerifyNotification(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
	return nil, nil
}
func (refundProviderTestDouble) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, nil
}

type zpayRefundProviderTestDouble struct {
	refundProviderTestDouble
}

func (zpayRefundProviderTestDouble) UsesRefundQuoteAmount() bool { return true }

type refundQueryProviderTestDouble struct {
	refundProviderTestDouble
	refundResponse *payment.RefundResponse
}

func (p *refundQueryProviderTestDouble) QueryRefund(context.Context, payment.RefundQueryRequest) (*payment.RefundResponse, error) {
	return p.refundResponse, nil
}
