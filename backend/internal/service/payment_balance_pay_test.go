//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestValidateSubOrderAllowsRenewingSameActiveSubscription(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	planID := createBalancePayTestPlan(t, ctx, client, 7, 59)

	u, err := client.User.Create().
		SetEmail("renew-same-subscription@example.com").
		SetUsername("renew-same-subscription").
		SetPasswordHash("hash").
		SetBalance(100).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)

	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        42,
		UserID:    u.ID,
		GroupID:   7,
		Status:    SubscriptionStatusActive,
		StartsAt:  time.Now().Add(-24 * time.Hour),
		ExpiresAt: time.Now().Add(29 * 24 * time.Hour),
	})
	svc := newBalancePayTestService(client, 0)
	svc.subscriptionSvc = NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, subRepo, nil, nil, nil)

	_, err = svc.validateSubOrder(ctx, CreateOrderRequest{
		UserID:    u.ID,
		OrderType: payment.OrderTypeSubscription,
		PlanID:    planID,
	})
	require.NoError(t, err)
}

func TestValidateSubOrderRejectsActiveSubscriptionInDifferentGroup(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	planID := createBalancePayTestPlan(t, ctx, client, 7, 59)

	u, err := client.User.Create().
		SetEmail("existing-subscription@example.com").
		SetUsername("existing-subscription").
		SetPasswordHash("hash").
		SetBalance(100).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)

	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        42,
		UserID:    u.ID,
		GroupID:   2,
		Status:    SubscriptionStatusActive,
		StartsAt:  time.Now().Add(-24 * time.Hour),
		ExpiresAt: time.Now().Add(29 * 24 * time.Hour),
	})
	svc := newBalancePayTestService(client, 0)
	svc.subscriptionSvc = NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, subRepo, nil, nil, nil)

	_, err = svc.validateSubOrder(ctx, CreateOrderRequest{
		UserID:    u.ID,
		OrderType: payment.OrderTypeSubscription,
		PlanID:    planID,
	})
	require.Error(t, err)
	require.Equal(t, "ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND", infraerrors.FromError(err).Reason)
	require.Equal(t, "当前套餐仍在有效期内，如需更换套餐，请先退款后再购买", infraerrors.FromError(err).Message)
}

func TestBalancePaySubscriptionRenewsSameActiveSubscription(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	planID := createBalancePayTestPlan(t, ctx, client, 7, 59)

	u, err := client.User.Create().
		SetEmail("balance-pay-renew-same-subscription@example.com").
		SetUsername("balance-pay-renew-same-subscription").
		SetPasswordHash("hash").
		SetBalance(100).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)

	expiresAt := time.Now().Add(29 * 24 * time.Hour)
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        43,
		UserID:    u.ID,
		GroupID:   7,
		Status:    SubscriptionStatusActive,
		StartsAt:  time.Now().Add(-24 * time.Hour),
		ExpiresAt: expiresAt,
		Notes:     "initial subscription",
	})
	svc := newBalancePayTestService(client, 0)
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	svc.subscriptionSvc = NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, subRepo, nil, nil, nil)
	svc.subscriptionSvc.entitlementPeriodRepo = entitlementRepo

	resp, err := svc.BalancePayOrder(ctx, BalancePayOrderRequest{
		UserID:    u.ID,
		OrderType: payment.OrderTypeSubscription,
		PlanID:    planID,
		ClientIP:  "127.0.0.1",
		SrcHost:   "api.example.com",
	})
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, resp.Status)

	order, err := client.PaymentOrder.Query().Only(ctx)
	require.NoError(t, err)
	require.NotNil(t, order.SubscriptionID)
	require.Equal(t, int64(43), *order.SubscriptionID)

	renewed, err := subRepo.GetByID(ctx, 43)
	require.NoError(t, err)
	require.Equal(t, expiresAt.AddDate(0, 0, 30), renewed.ExpiresAt)
	require.Contains(t, renewed.Notes, "initial subscription")
	require.Contains(t, renewed.Notes, "payment order")
	require.Zero(t, subRepo.createCalls)

	period, err := entitlementRepo.GetBySource(ctx, paymentOrderSubscriptionEntitlementSource(order.ID))
	require.NoError(t, err)
	require.Equal(t, int64(43), period.SubscriptionID)
	require.Equal(t, expiresAt, period.StartsAt)
	require.Equal(t, expiresAt.AddDate(0, 0, 30), period.ExpiresAt)
}

func TestBalancePaySubscriptionRejectsDifferentActiveSubscription(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	planID := createBalancePayTestPlan(t, ctx, client, 7, 59)

	u, err := client.User.Create().
		SetEmail("balance-pay-existing-subscription@example.com").
		SetUsername("balance-pay-existing-subscription").
		SetPasswordHash("hash").
		SetBalance(100).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)

	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        43,
		UserID:    u.ID,
		GroupID:   2,
		Status:    SubscriptionStatusActive,
		StartsAt:  time.Now().Add(-24 * time.Hour),
		ExpiresAt: time.Now().Add(29 * 24 * time.Hour),
	})
	svc := newBalancePayTestService(client, 0)
	svc.subscriptionSvc = NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, subRepo, nil, nil, nil)

	_, err = svc.BalancePayOrder(ctx, BalancePayOrderRequest{
		UserID:    u.ID,
		OrderType: payment.OrderTypeSubscription,
		PlanID:    planID,
		ClientIP:  "127.0.0.1",
		SrcHost:   "api.example.com",
	})
	require.Error(t, err)
	require.Equal(t, "ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND", infraerrors.FromError(err).Reason)

	orderCount, err := client.PaymentOrder.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, orderCount)

	reloaded, err := client.User.Query().Where(user.IDEQ(u.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 100.0, reloaded.Balance)
	require.Zero(t, subRepo.createCalls)
}

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
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	svc.subscriptionSvc = NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, subRepo, nil, nil, nil)
	svc.subscriptionSvc.entitlementPeriodRepo = entitlementRepo

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

	order, err := client.PaymentOrder.Query().Only(ctx)
	require.NoError(t, err)
	period, err := entitlementRepo.GetBySource(ctx, paymentOrderSubscriptionEntitlementSource(order.ID))
	require.NoError(t, err)
	require.Equal(t, u.ID, period.UserID)
	require.Equal(t, int64(7), period.GroupID)
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
	seedPaymentTestSubscriptionGroup(t, ctx, client, groupID)
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

func seedPaymentTestSubscriptionGroup(t *testing.T, ctx context.Context, client *dbent.Client, groupID int64) {
	t.Helper()
	_, err := client.ExecContext(ctx, `
INSERT INTO groups (
	id, name, status, platform, subscription_type,
	rate_multiplier, is_exclusive, default_validity_days,
	allow_image_generation, claude_code_only,
	model_routing, model_routing_enabled, mcp_xml_inject,
	supported_model_scopes, sort_order,
	allow_messages_dispatch, require_oauth_only, require_privacy_set,
	default_mapped_model, messages_dispatch_model_config, models_list_config,
	rpm_limit, created_at, updated_at
)
VALUES (
	?, ?, ?, ?, ?,
	1.0, false, 30,
	false, false,
	'{}', false, true,
	'["claude","gemini_text","gemini_image"]', 0,
	false, false, false,
	'', '{}', '{}',
	0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
)
ON CONFLICT(id) DO UPDATE SET
	status = excluded.status,
	platform = excluded.platform,
	subscription_type = excluded.subscription_type,
	updated_at = CURRENT_TIMESTAMP
`, groupID, "test-subscription-group-"+strconv.FormatInt(groupID, 10), payment.EntityStatusActive, PlatformOpenAI, SubscriptionTypeSubscription)
	require.NoError(t, err)
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
