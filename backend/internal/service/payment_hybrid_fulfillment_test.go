//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentbalancehold"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestHybridWebhook_CapturesHoldAndFulfillsExactlyOnce(t *testing.T) {
	ctx := context.Background()
	scenario := newHybridFulfillmentScenario(t, ctx, balanceHoldStatusReserved, OrderStatusPending)

	err := scenario.svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		TradeNo: "trade-hybrid-ok",
		OrderID: scenario.order.OutTradeNo,
		Amount:  73.47,
		Status:  payment.NotificationStatusSuccess,
	}, payment.TypeAlipay)
	require.NoError(t, err)
	err = scenario.svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		TradeNo: "trade-hybrid-ok",
		OrderID: scenario.order.OutTradeNo,
		Amount:  73.47,
		Status:  payment.NotificationStatusSuccess,
	}, payment.TypeAlipay)
	require.NoError(t, err)

	reloaded, err := scenario.client.PaymentOrder.Get(ctx, scenario.order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.Equal(t, 79.79, reloaded.PayAmount)
	require.Equal(t, 73.47, reloaded.GatewayAmount)
	require.Equal(t, paymentResolutionStatusPaid, reloaded.PaymentResolutionStatus)
	hold, err := scenario.client.PaymentBalanceHold.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, balanceHoldStatusCaptured, hold.Status)
	require.NotNil(t, hold.CapturedAt)
	require.Equal(t, 1, scenario.subRepo.createCalls)
}

func TestHybridWebhook_RejectsPayAmountInsteadOfGatewayAmount(t *testing.T) {
	ctx := context.Background()
	scenario := newHybridFulfillmentScenario(t, ctx, balanceHoldStatusReserved, OrderStatusPending)

	err := scenario.svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		TradeNo: "trade-hybrid-wrong-amount",
		OrderID: scenario.order.OutTradeNo,
		Amount:  79.79,
		Status:  payment.NotificationStatusSuccess,
	}, payment.TypeAlipay)

	require.Error(t, err)
	reloaded, err := scenario.client.PaymentOrder.Get(ctx, scenario.order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, reloaded.Status)
	hold, err := scenario.client.PaymentBalanceHold.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, balanceHoldStatusReserved, hold.Status)
	require.Zero(t, scenario.subRepo.createCalls)
}

func TestHybridWebhook_AfterReleasedHoldCreditsGatewayAmountOnce(t *testing.T) {
	ctx := context.Background()
	scenario := newHybridFulfillmentScenario(t, ctx, balanceHoldStatusReleased, OrderStatusExpired)

	err := scenario.svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		TradeNo: "trade-hybrid-late",
		OrderID: scenario.order.OutTradeNo,
		Amount:  73.47,
		Status:  payment.NotificationStatusSuccess,
	}, payment.TypeAlipay)
	require.NoError(t, err)
	err = scenario.svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		TradeNo: "trade-hybrid-late",
		OrderID: scenario.order.OutTradeNo,
		Amount:  73.47,
		Status:  payment.NotificationStatusSuccess,
	}, payment.TypeAlipay)
	require.NoError(t, err)

	reloaded, err := scenario.client.PaymentOrder.Get(ctx, scenario.order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompensated, reloaded.Status)
	require.Equal(t, 73.47, reloaded.CompensationAmount)
	require.NotNil(t, reloaded.CompensatedAt)
	reloadedUser, err := scenario.client.User.Query().Where(user.IDEQ(scenario.userID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 81.79, reloadedUser.Balance)
	require.Zero(t, scenario.subRepo.createCalls)
}

type hybridFulfillmentScenario struct {
	client  *dbent.Client
	svc     *PaymentService
	subRepo *subscriptionUserSubRepoStub
	order   *dbent.PaymentOrder
	userID  int64
}

func newHybridFulfillmentScenario(t *testing.T, ctx context.Context, holdStatus, orderStatus string) hybridFulfillmentScenario {
	t.Helper()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	initialBalance := 2.0
	if holdStatus == balanceHoldStatusReleased {
		initialBalance = 8.32
	}
	u, err := client.User.Create().
		SetEmail("hybrid-fulfillment@example.com").
		SetPasswordHash("hash").
		SetUsername("hybrid-fulfillment").
		SetBalance(initialBalance).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().
		SetUserID(u.ID).
		SetUserEmail(u.Email).
		SetUserName(u.Username).
		SetAmount(79).
		SetPayAmount(79.79).
		SetFeeRate(1).
		SetFundingMode(paymentFundingModeMixed).
		SetBalanceAmount(6.32).
		SetGatewayAmount(73.47).
		SetRechargeCode("HYBRID-FULFILLMENT").
		SetOutTradeNo("sub2_hybrid_fulfillment").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(7).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(30).
		SetStatus(orderStatus).
		SetExpiresAt(time.Now().Add(30 * time.Minute)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)
	holdBuilder := client.PaymentBalanceHold.Create().
		SetOrderID(order.ID).
		SetUserID(u.ID).
		SetAmount(6.32).
		SetStatus(holdStatus).
		SetExpiresAt(time.Now().Add(35 * time.Minute))
	if holdStatus == balanceHoldStatusReleased {
		holdBuilder.SetReleasedAt(time.Now()).SetReleaseReason("test_released")
	}
	_, err = holdBuilder.Save(ctx)
	require.NoError(t, err)
	count, err := client.PaymentBalanceHold.Query().Where(paymentbalancehold.OrderIDEQ(order.ID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	subRepo := newSubscriptionUserSubRepoStub()
	groupRepo := &subscriptionGroupRepoStub{group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription}}
	svc := &PaymentService{
		entClient:       client,
		groupRepo:       groupRepo,
		subscriptionSvc: NewSubscriptionService(groupRepo, subRepo, nil, nil, nil),
	}
	return hybridFulfillmentScenario{client: client, svc: svc, subRepo: subRepo, order: order, userID: u.ID}
}
