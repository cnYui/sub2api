//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestCalculateSubscriptionRefundAmountFloorsRemainingDaysAndRoundsToOneDecimal(t *testing.T) {
	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Duration(24.2 * float64(24*time.Hour)))

	got := calculateSubscriptionRefundAmount(29, 29.29, 30, expiresAt, now, false)

	require.Equal(t, 23.2, got)
}

func TestRequestRefundAutomaticallyRefundsAlipaySubscriptionWithoutFeeAndRevokesSubscription(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	var refundForm url.Values
	refundServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api.php", r.URL.Path)
		require.Equal(t, "refund", r.URL.Query().Get("act"))
		require.NoError(t, r.ParseForm())
		refundForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"ok"}`))
	}))
	defer refundServer.Close()

	user, err := client.User.Create().
		SetEmail("auto-alipay-refund@example.com").
		SetPasswordHash("hash").
		SetUsername("auto-alipay-refund-user").
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeEasyPay).
		SetName("zpay-refund-instance").
		SetConfig(encryptWebhookProviderConfig(t, map[string]string{
			"pid":       "pid-1",
			"pkey":      "pkey-1",
			"apiBase":   refundServer.URL,
			"notifyUrl": "https://api.example.com/notify",
			"returnUrl": "https://api.example.com/return",
		})).
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		SetAllowUserRefund(true).
		SetPaymentMode("popup").
		Save(ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(inst.ID, 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(29).
		SetPayAmount(29.29).
		SetFeeRate(1).
		SetRechargeCode("AUTO-ALIPAY-REFUND").
		SetOutTradeNo("sub2_auto_alipay_refund").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("zpay-trade-1").
		SetOrderType(payment.OrderTypeSubscription).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(30).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now().Add(-5 * 24 * time.Hour)).
		SetCompletedAt(time.Now().Add(-5 * 24 * time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeEasyPay).
		SetProviderSnapshot(map[string]any{
			"schema_version":       2,
			"provider_instance_id": instID,
			"provider_key":         payment.TypeEasyPay,
			"merchant_id":          "pid-1",
		}).
		Save(ctx)
	require.NoError(t, err)

	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        99,
		UserID:    user.ID,
		GroupID:   7,
		Status:    SubscriptionStatusActive,
		StartsAt:  time.Now().Add(-5 * 24 * time.Hour),
		ExpiresAt: time.Now().Add(25*24*time.Hour + time.Hour),
	})
	svc := &PaymentService{
		entClient:    client,
		loadBalancer: newWebhookProviderTestLoadBalancer(client),
		subscriptionSvc: NewSubscriptionService(&subscriptionGroupRepoStub{
			group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
		}, subRepo, nil, nil, nil),
	}

	err = svc.RequestRefund(ctx, order.ID, user.ID, "用户自动退款")
	require.NoError(t, err)
	require.Equal(t, "24.20", refundForm.Get("money"))
	require.Equal(t, "sub2_auto_alipay_refund", refundForm.Get("out_trade_no"))

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	require.Equal(t, 24.2, reloaded.RefundAmount)

	_, err = subRepo.GetByID(ctx, 99)
	require.ErrorIs(t, err, ErrSubscriptionNotFound)
}

func TestRequestRefundAutomaticallyRefundsBalanceSubscriptionIncludingFee(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	user, err := client.User.Create().
		SetEmail("auto-balance-refund@example.com").
		SetPasswordHash("hash").
		SetUsername("auto-balance-refund-user").
		SetStatus(payment.EntityStatusActive).
		SetBalance(0).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(29).
		SetPayAmount(29.29).
		SetFeeRate(1).
		SetRechargeCode("AUTO-BALANCE-REFUND").
		SetOutTradeNo("sub2_auto_balance_refund").
		SetPaymentType(payment.TypeBalance).
		SetPaymentTradeNo("balance").
		SetOrderType(payment.OrderTypeSubscription).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(30).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now().Add(-5 * 24 * time.Hour)).
		SetCompletedAt(time.Now().Add(-5 * 24 * time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        100,
		UserID:    user.ID,
		GroupID:   7,
		Status:    SubscriptionStatusActive,
		StartsAt:  time.Now().Add(-5 * 24 * time.Hour),
		ExpiresAt: time.Now().Add(25*24*time.Hour + time.Hour),
	})
	svc := &PaymentService{
		entClient: client,
		subscriptionSvc: NewSubscriptionService(&subscriptionGroupRepoStub{
			group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
		}, subRepo, nil, nil, nil),
	}

	err = svc.RequestRefund(ctx, order.ID, user.ID, "余额自动退款")
	require.NoError(t, err)

	reloadedUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 24.4, reloadedUser.Balance)

	reloadedOrder, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloadedOrder.Status)
	require.Equal(t, 24.4, reloadedOrder.RefundAmount)

	_, err = subRepo.GetByID(ctx, 100)
	require.ErrorIs(t, err, ErrSubscriptionNotFound)
}

func TestRequestRefundRejectsTrafficPackAutoRefund(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("traffic-refund@example.com").
		SetPasswordHash("hash").
		SetUsername("traffic-refund-user").
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(5).
		SetPayAmount(5.05).
		SetFeeRate(1).
		SetRechargeCode("TRAFFIC-REFUND").
		SetOutTradeNo("sub2_traffic_refund").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("zpay-trade-traffic").
		SetOrderType(payment.OrderTypeTrafficPack).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetCompletedAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{entClient: client}
	err = svc.RequestRefund(ctx, order.ID, user.ID, "流量卡退款")
	require.Error(t, err)
	require.Equal(t, "INVALID_ORDER_TYPE", infraerrors.Reason(err))
}

func TestValidateRefundRequestRejectsLegacyGuessedProviderInstance(t *testing.T) {
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
		SetOrderType(payment.OrderTypeBalance).
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

	_, err = svc.validateRefundRequest(ctx, order.ID, user.ID)
	require.Error(t, err)
	require.Equal(t, "USER_REFUND_DISABLED", infraerrors.Reason(err))
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
		SetOrderType(payment.OrderTypeBalance).
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

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 0, "", false, false)
	require.Nil(t, plan)
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "REFUND_DISABLED", infraerrors.Reason(err))
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
		SetOrderType(payment.OrderTypeBalance).
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

	err = svc.gwRefund(ctx, &RefundPlan{
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
