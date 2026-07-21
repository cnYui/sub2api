//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestHybridRefund_PersistsSplitAndDoesNotRefundFee(t *testing.T) {
	provider := &refundProviderStub{responses: []*payment.RefundResponse{{RefundID: "refund-hybrid", Status: payment.ProviderStatusSuccess}}}
	scenario := newHybridRefundScenario(t, provider)

	result, err := scenario.svc.ExecuteRefund(scenario.ctx, &RefundPlan{
		OrderID:         scenario.orderID,
		RefundAmount:    63.2,
		GatewayAmount:   63.2,
		Reason:          "混合支付退款",
		DeductionType:   payment.DeductionTypeSubscription,
		SubscriptionID:  scenario.subID,
		SubDaysToDeduct: 30,
	})

	require.NoError(t, err)
	require.True(t, result.Success)
	require.Len(t, provider.requests, 1)
	require.Equal(t, "58.14", provider.requests[0].Amount)
	reloaded, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.Equal(t, 63.2, reloaded.RefundAmount)
	require.Equal(t, 5.06, reloaded.RefundBalanceAmount)
	require.Equal(t, 58.14, reloaded.RefundGatewayAmount)
	require.Equal(t, RefundGatewaySucceeded, reloaded.RefundGatewayStatus)
	require.Equal(t, RefundGatewaySucceeded, reloaded.RefundBalanceStatus)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	reloadedUser, err := scenario.client.User.Query().Where(user.IDEQ(scenario.userID)).Only(scenario.ctx)
	require.NoError(t, err)
	require.Equal(t, 5.06, reloadedUser.Balance)
}

func TestHybridRefund_GatewaySuccessThenLocalRetryDoesNotRefundGatewayTwice(t *testing.T) {
	provider := &refundProviderStub{responses: []*payment.RefundResponse{{RefundID: "should-not-be-used", Status: payment.ProviderStatusSuccess}}}
	scenario := newHybridRefundScenario(t, provider)
	_, err := scenario.client.PaymentOrder.UpdateOneID(scenario.orderID).
		SetStatus(OrderStatusRefundFailed).
		SetRefundAmount(63.2).
		SetRefundRequestID("refund-hybrid-retry").
		SetRefundRequestReason("混合支付退款").
		SetRefundBalanceAmount(5.06).
		SetRefundGatewayAmount(58.14).
		SetRefundBalanceStatus(RefundGatewayNotStarted).
		SetRefundGatewayStatus(RefundGatewaySucceeded).
		SetRefundEntitlementStatus(RefundEntitlementFailed).
		Save(scenario.ctx)
	require.NoError(t, err)

	result, err := scenario.svc.ExecuteRefund(scenario.ctx, &RefundPlan{
		OrderID:         scenario.orderID,
		RefundAmount:    63.2,
		GatewayAmount:   63.2,
		Reason:          "混合支付退款",
		DeductionType:   payment.DeductionTypeSubscription,
		SubscriptionID:  scenario.subID,
		SubDaysToDeduct: 30,
	})

	require.NoError(t, err)
	require.True(t, result.Success)
	require.Empty(t, provider.requests)
	reloaded, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.Equal(t, RefundGatewaySucceeded, reloaded.RefundGatewayStatus)
	require.Equal(t, RefundGatewaySucceeded, reloaded.RefundBalanceStatus)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	reloadedUser, err := scenario.client.User.Query().Where(user.IDEQ(scenario.userID)).Only(scenario.ctx)
	require.NoError(t, err)
	require.Equal(t, 5.06, reloadedUser.Balance)
}

type hybridRefundScenario struct {
	ctx     context.Context
	client  *dbent.Client
	svc     *PaymentService
	orderID int64
	userID  int64
	subID   int64
	subRepo *subscriptionUserSubRepoStub
}

func newHybridRefundScenario(t *testing.T, provider payment.Provider) hybridRefundScenario {
	t.Helper()
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	userEntity, err := client.User.Create().
		SetEmail("hybrid-refund@example.com").
		SetPasswordHash("hash").
		SetUsername("hybrid-refund").
		SetBalance(0).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)
	subID := int64(501)
	startsAt := time.Now().AddDate(0, 0, -5)
	order, err := client.PaymentOrder.Create().
		SetUserID(userEntity.ID).
		SetUserEmail(userEntity.Email).
		SetUserName(userEntity.Username).
		SetAmount(79).
		SetPayAmount(79.79).
		SetFeeRate(1).
		SetFundingMode(paymentFundingModeMixed).
		SetBalanceAmount(6.32).
		SetGatewayAmount(73.47).
		SetRechargeCode("HYBRID-REFUND").
		SetOutTradeNo("sub2_hybrid_refund").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-hybrid-refund").
		SetOrderType(payment.OrderTypeSubscription).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(30).
		SetSubscriptionID(subID).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(startsAt).
		SetCompletedAt(startsAt).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.test").
		Save(ctx)
	require.NoError(t, err)
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        subID,
		UserID:    userEntity.ID,
		GroupID:   7,
		Status:    SubscriptionStatusActive,
		StartsAt:  startsAt,
		ExpiresAt: startsAt.AddDate(0, 0, 30),
	})
	groupRepo := &subscriptionGroupRepoStub{group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription}}
	svc := &PaymentService{
		entClient:       client,
		refundProvider:  provider,
		subscriptionSvc: NewSubscriptionService(groupRepo, subRepo, nil, nil, nil),
	}
	scenario := autoGatewayRefundScenario{ctx: ctx, client: client, userID: userEntity.ID, orderID: order.ID, subID: subID, subRepo: subRepo, svc: svc}
	attachRefundQuoteEntitlement(t, &scenario, 198, 792, 158.4)
	return hybridRefundScenario{ctx: ctx, client: client, svc: svc, orderID: order.ID, userID: userEntity.ID, subID: scenario.subID, subRepo: subRepo}
}
