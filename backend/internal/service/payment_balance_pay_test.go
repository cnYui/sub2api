//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestBalancePaySubscriptionInsufficientDoesNotCreateOrder(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	planID := createBalancePayTestPlan(t, ctx, client, 7, 79)

	u, err := client.User.Create().
		SetEmail("balance-pay-insufficient@example.com").
		SetUsername("balance-pay-insufficient").
		SetPasswordHash("hash").
		SetBalance(10).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)

	svc := newBalancePayTestService(client, 0)
	_, err = svc.BalancePayOrder(ctx, BalancePayOrderRequest{
		UserID:    u.ID,
		OrderType: payment.OrderTypeSubscription,
		PlanID:    planID,
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

func TestBalancePaySubscriptionDeductsPayAmountAndCompletesOrder(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	planID := createBalancePayTestPlan(t, ctx, client, 7, 79)

	u, err := client.User.Create().
		SetEmail("balance-pay-subscription@example.com").
		SetUsername("balance-pay-subscription").
		SetPasswordHash("hash").
		SetBalance(100).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)

	subRepo := newSubscriptionUserSubRepoStub()
	svc := newBalancePayTestService(client, 1)
	svc.subscriptionSvc = NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, subRepo, nil, nil, nil)

	resp, err := svc.BalancePayOrder(ctx, BalancePayOrderRequest{
		UserID:    u.ID,
		OrderType: payment.OrderTypeSubscription,
		PlanID:    planID,
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

	trafficRepo := &balancePayTrafficPackRepo{pack: &TrafficPack{
		ID:           3,
		Code:         "openai-5",
		Name:         "GPT 流量包",
		Price:        5,
		CreditUSD:    5,
		ValidityDays: 365,
		Platform:     TrafficPackPlatformOpenAI,
		ForSale:      true,
	}}
	svc := newBalancePayTestService(client, 1)
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

func createBalancePayTestPlan(t *testing.T, ctx context.Context, client *dbent.Client, groupID int64, price float64) int64 {
	t.Helper()
	plan, err := client.SubscriptionPlan.Create().
		SetGroupID(groupID).
		SetName("RMB Plan").
		SetPrice(price).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetForSale(true).
		Save(ctx)
	require.NoError(t, err)
	return plan.ID
}

func newBalancePayTestService(client *dbent.Client, feeRate float64) *PaymentService {
	settingSvc := NewSettingService(&paymentFulfillmentSettingRepoStub{values: map[string]string{
		SettingPaymentEnabled:      "true",
		SettingRechargeFeeRate:     strconv.FormatFloat(feeRate, 'f', -1, 64),
		SettingMaxPendingOrders:    "3",
		SettingOrderTimeoutMinutes: "30",
	}}, nil)
	cfgSvc := NewPaymentConfigService(client, settingSvc.settingRepo, nil)
	return &PaymentService{
		entClient:     client,
		configService: cfgSvc,
		groupRepo:     &subscriptionGroupRepoStub{group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription}},
	}
}

type balancePayTrafficPackRepo struct {
	TrafficPackRepository

	pack        *TrafficPack
	creditCalls int
	lastInput   CreditTrafficPackInput
}

func (r *balancePayTrafficPackRepo) GetForSaleByID(_ context.Context, id int64) (*TrafficPack, error) {
	if r.pack == nil || r.pack.ID != id || !r.pack.ForSale {
		return nil, ErrInvalidInput
	}
	cp := *r.pack
	return &cp, nil
}

func (r *balancePayTrafficPackRepo) CreditPurchase(_ context.Context, input CreditTrafficPackInput) error {
	r.creditCalls++
	r.lastInput = input
	return nil
}
