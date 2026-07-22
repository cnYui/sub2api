//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"entgo.io/ent"
	entsql "entgo.io/ent/dialect/sql"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionentitlementperiod"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type refundProviderStub struct {
	responses []*payment.RefundResponse
	errors    []error
	requests  []payment.RefundRequest
}

func (p *refundProviderStub) Name() string        { return "refund-stub" }
func (p *refundProviderStub) ProviderKey() string { return payment.TypeEasyPay }
func (p *refundProviderStub) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeAlipay}
}
func (p *refundProviderStub) CreatePayment(context.Context, payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	panic("unexpected CreatePayment call")
}
func (p *refundProviderStub) QueryOrder(context.Context, string) (*payment.QueryOrderResponse, error) {
	panic("unexpected QueryOrder call")
}
func (p *refundProviderStub) VerifyNotification(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
	panic("unexpected VerifyNotification call")
}
func (p *refundProviderStub) Refund(_ context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	call := len(p.requests)
	p.requests = append(p.requests, req)
	var resp *payment.RefundResponse
	if call < len(p.responses) {
		resp = p.responses[call]
	}
	var err error
	if call < len(p.errors) {
		err = p.errors[call]
	}
	return resp, err
}

type refundDeleteOnceRepo struct {
	*subscriptionUserSubRepoStub
	err error
}

type refundRevokeOnceEntitlementRepo struct {
	*refundQuoteEntitlementRepo
	err error
}

type refundEntSubscriptionRepo struct {
	userSubRepoNoop
	client     *dbent.Client
	failDelete bool
}

func (r *refundEntSubscriptionRepo) clientFor(ctx context.Context) *dbent.Client {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return r.client
}

func (r *refundEntSubscriptionRepo) GetByID(ctx context.Context, id int64) (*UserSubscription, error) {
	entity, err := r.clientFor(ctx).UserSubscription.Query().Where(usersubscription.IDEQ(id)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}
	return &UserSubscription{
		ID:        entity.ID,
		UserID:    entity.UserID,
		GroupID:   entity.GroupID,
		StartsAt:  entity.StartsAt,
		ExpiresAt: entity.ExpiresAt,
		Status:    entity.Status,
	}, nil
}

func (r *refundEntSubscriptionRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := r.clientFor(ctx).UserSubscription.UpdateOneID(id).SetStatus(status).Save(ctx)
	return err
}

func (r *refundEntSubscriptionRepo) Delete(ctx context.Context, id int64) error {
	if r.failDelete {
		return errors.New("delete subscription failed")
	}
	_, err := r.clientFor(ctx).UserSubscription.UpdateOneID(id).SetDeletedAt(time.Now()).Save(ctx)
	return err
}

func (r *refundDeleteOnceRepo) Delete(ctx context.Context, id int64) error {
	if r.err != nil {
		err := r.err
		r.err = nil
		return err
	}
	return r.subscriptionUserSubRepoStub.Delete(ctx, id)
}

func (r *refundRevokeOnceEntitlementRepo) RevokeBySource(ctx context.Context, source SubscriptionEntitlementSource, now time.Time, reason string) error {
	if r.err != nil {
		err := r.err
		r.err = nil
		return err
	}
	return r.refundQuoteEntitlementRepo.RevokeBySource(ctx, source, now, reason)
}

type autoGatewayRefundScenario struct {
	ctx     context.Context
	client  *dbent.Client
	userID  int64
	orderID int64
	subID   int64
	subRepo *subscriptionUserSubRepoStub
	svc     *PaymentService
}

type refundQuoteEntitlementFixture struct {
	groupID        int64
	subscriptionID int64
	entitlementID  int64
	startsAt       time.Time
	expiresAt      time.Time
}

type refundQuoteEntitlementRepo struct {
	client *dbent.Client
}

func (r *refundQuoteEntitlementRepo) clientFor(ctx context.Context) *dbent.Client {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return r.client
}

func (r *refundQuoteEntitlementRepo) GetBySource(ctx context.Context, source SubscriptionEntitlementSource) (*SubscriptionEntitlementPeriod, error) {
	period, err := r.clientFor(ctx).SubscriptionEntitlementPeriod.Query().
		Where(
			subscriptionentitlementperiod.SourceTypeEQ(source.Type),
			subscriptionentitlementperiod.SourceIDEQ(source.ID),
		).
		Only(ctx)
	if err != nil {
		return nil, ErrSubscriptionEntitlementPeriodNotFound
	}
	return &SubscriptionEntitlementPeriod{
		ID:                  period.ID,
		UserID:              period.UserID,
		SubscriptionID:      period.SubscriptionID,
		GroupID:             period.GroupID,
		Source:              SubscriptionEntitlementSource{Type: period.SourceType, ID: period.SourceID},
		StartsAt:            period.StartsAt,
		ExpiresAt:           period.ExpiresAt,
		PeriodDays:          period.PeriodDays,
		DailyLimitUSD:       cloneOptionalFloat64(period.DailyLimitUsd),
		WeeklyLimitUSD:      cloneOptionalFloat64(period.WeeklyLimitUsd),
		PeriodTotalQuotaUSD: cloneOptionalFloat64(period.PeriodTotalQuotaUsd),
		QuotaWindowUnit:     period.QuotaWindowUnit,
		QuotaWindowDays:     period.QuotaWindowDays,
		Status:              period.Status,
		RevokedAt:           period.RevokedAt,
		RevokedReason:       period.RevokedReason,
	}, nil
}

func (r *refundQuoteEntitlementRepo) Create(ctx context.Context, period *SubscriptionEntitlementPeriod) error {
	if period == nil {
		return ErrSubscriptionEntitlementPeriodNilInput
	}
	builder := r.clientFor(ctx).SubscriptionEntitlementPeriod.Create().
		SetUserID(period.UserID).
		SetSubscriptionID(period.SubscriptionID).
		SetGroupID(period.GroupID).
		SetSourceType(period.Source.Type).
		SetSourceID(period.Source.ID).
		SetStartsAt(period.StartsAt).
		SetExpiresAt(period.ExpiresAt).
		SetPeriodDays(period.PeriodDays).
		SetQuotaWindowUnit(period.QuotaWindowUnit).
		SetQuotaWindowDays(period.QuotaWindowDays).
		SetStatus(period.Status)
	if period.DailyLimitUSD != nil {
		builder.SetDailyLimitUsd(*period.DailyLimitUSD)
	}
	if period.WeeklyLimitUSD != nil {
		builder.SetWeeklyLimitUsd(*period.WeeklyLimitUSD)
	}
	if period.PeriodTotalQuotaUSD != nil {
		builder.SetPeriodTotalQuotaUsd(*period.PeriodTotalQuotaUSD)
	}
	_, err := builder.Save(ctx)
	return err
}

func (r *refundQuoteEntitlementRepo) RevokeUnexpiredBySubscription(ctx context.Context, subscriptionID int64, now time.Time, reason string) error {
	_, err := r.clientFor(ctx).SubscriptionEntitlementPeriod.Update().
		Where(
			subscriptionentitlementperiod.SubscriptionIDEQ(subscriptionID),
			subscriptionentitlementperiod.StatusEQ(SubscriptionEntitlementPeriodStatusActive),
			subscriptionentitlementperiod.ExpiresAtGT(now),
		).
		SetStatus(SubscriptionEntitlementPeriodStatusRevoked).
		SetRevokedAt(now).
		SetRevokedReason(reason).
		Save(ctx)
	return err
}

func (r *refundQuoteEntitlementRepo) RevokeBySource(ctx context.Context, source SubscriptionEntitlementSource, now time.Time, reason string) error {
	updated, err := r.clientFor(ctx).SubscriptionEntitlementPeriod.Update().
		Where(
			subscriptionentitlementperiod.SourceTypeEQ(source.Type),
			subscriptionentitlementperiod.SourceIDEQ(source.ID),
			subscriptionentitlementperiod.StatusEQ(SubscriptionEntitlementPeriodStatusActive),
		).
		SetStatus(SubscriptionEntitlementPeriodStatusRevoked).
		SetRevokedAt(now).
		SetRevokedReason(reason).
		Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrSubscriptionEntitlementPeriodNotFound
	}
	return nil
}

func newAutoGatewayRefundScenario(t *testing.T, provider payment.Provider, repo *subscriptionUserSubRepoStub) autoGatewayRefundScenario {
	t.Helper()
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	userEntity, err := client.User.Create().
		SetEmail("refund-state@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-state-user").
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeEasyPay).
		SetName("refund-state-instance").
		SetConfig(encryptWebhookProviderConfig(t, map[string]string{
			"pid":       "pid-1",
			"pkey":      "pkey-1",
			"apiBase":   "https://pay.example.test",
			"notifyUrl": "https://api.example.test/notify",
			"returnUrl": "https://api.example.test/return",
		})).
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		SetAllowUserRefund(true).
		Save(ctx)
	require.NoError(t, err)

	subID := int64(201)
	startsAt := time.Now().AddDate(0, 0, -5)
	order, err := client.PaymentOrder.Create().
		SetUserID(userEntity.ID).
		SetUserEmail(userEntity.Email).
		SetUserName(userEntity.Username).
		SetAmount(29).
		SetPayAmount(29.29).
		SetFeeRate(1).
		SetRechargeCode("REFUND-STATE").
		SetOutTradeNo("sub2_refund_state").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-refund-state").
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
		SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).
		SetProviderKey(payment.TypeEasyPay).
		Save(ctx)
	require.NoError(t, err)

	if repo == nil {
		repo = newSubscriptionUserSubRepoStub()
	}
	repo.seed(&UserSubscription{
		ID:        subID,
		UserID:    userEntity.ID,
		GroupID:   7,
		Status:    SubscriptionStatusActive,
		StartsAt:  startsAt,
		ExpiresAt: startsAt.AddDate(0, 0, 30),
	})
	svc := &PaymentService{
		entClient:       client,
		loadBalancer:    newWebhookProviderTestLoadBalancer(client),
		refundProvider:  provider,
		subscriptionSvc: NewSubscriptionService(&subscriptionGroupRepoStub{group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription}}, repo, nil, nil, nil),
	}
	return autoGatewayRefundScenario{ctx: ctx, client: client, userID: userEntity.ID, orderID: order.ID, subID: subID, subRepo: repo, svc: svc}
}

func attachRefundQuoteEntitlement(t *testing.T, scenario *autoGatewayRefundScenario, weeklyLimit, periodTotalQuota, usedQuota float64) refundQuoteEntitlementFixture {
	t.Helper()
	require.NotNil(t, scenario)
	ensureRefundQuoteUsageFactsTable(t, scenario.ctx, scenario.client)

	groupEntity, err := scenario.client.Group.Create().
		SetName("refund-quote-codex-" + strconv.FormatInt(scenario.orderID, 10)).
		SetPlatform("openai").
		SetSubscriptionType(SubscriptionTypeSubscription).
		SetWeeklyLimitUsd(weeklyLimit).
		SetDefaultValidityDays(28).
		Save(scenario.ctx)
	require.NoError(t, err)

	startsAt := time.Now().AddDate(0, 0, -3)
	expiresAt := startsAt.AddDate(0, 0, 28)
	subscriptionEntity, err := scenario.client.UserSubscription.Create().
		SetUserID(scenario.userID).
		SetGroupID(groupEntity.ID).
		SetStartsAt(startsAt).
		SetExpiresAt(expiresAt).
		SetWeeklyAnchorAt(startsAt).
		SetWeeklyWindowStart(startsAt).
		SetStatus(SubscriptionStatusActive).
		Save(scenario.ctx)
	require.NoError(t, err)

	scenario.subID = subscriptionEntity.ID
	scenario.subRepo.seed(&UserSubscription{
		ID:                subscriptionEntity.ID,
		UserID:            scenario.userID,
		GroupID:           groupEntity.ID,
		Status:            SubscriptionStatusActive,
		StartsAt:          startsAt,
		ExpiresAt:         expiresAt,
		WeeklyAnchorAt:    &startsAt,
		WeeklyWindowStart: &startsAt,
	})
	scenario.svc.subscriptionSvc.groupRepo = &subscriptionGroupRepoStub{
		group: &Group{
			ID:               groupEntity.ID,
			Status:           payment.EntityStatusActive,
			Platform:         "openai",
			SubscriptionType: SubscriptionTypeSubscription,
			WeeklyLimitUSD:   &weeklyLimit,
		},
	}
	scenario.svc.subscriptionSvc.entitlementPeriodRepo = &refundQuoteEntitlementRepo{client: scenario.client}

	_, err = scenario.client.PaymentOrder.UpdateOneID(scenario.orderID).
		SetSubscriptionGroupID(groupEntity.ID).
		SetSubscriptionDays(28).
		SetSubscriptionID(subscriptionEntity.ID).
		SetSubscriptionSnapshot(map[string]any{
			"version":                1,
			"plan_name":              "29 元订阅池",
			"group_id":               groupEntity.ID,
			"validity_days":          28,
			"weekly_limit_usd":       weeklyLimit,
			"period_total_quota_usd": periodTotalQuota,
			"quota_window_unit":      "week",
			"quota_window_days":      7,
		}).
		Save(scenario.ctx)
	require.NoError(t, err)

	period, err := scenario.client.SubscriptionEntitlementPeriod.Create().
		SetUserID(scenario.userID).
		SetSubscriptionID(subscriptionEntity.ID).
		SetGroupID(groupEntity.ID).
		SetSourceType(subscriptionEntitlementSourceTypePaymentOrder).
		SetSourceID(strconv.FormatInt(scenario.orderID, 10)).
		SetStartsAt(startsAt).
		SetExpiresAt(expiresAt).
		SetPeriodDays(28).
		SetWeeklyLimitUsd(weeklyLimit).
		SetPeriodTotalQuotaUsd(periodTotalQuota).
		SetQuotaWindowUnit("week").
		SetQuotaWindowDays(7).
		SetStatus(SubscriptionEntitlementPeriodStatusActive).
		Save(scenario.ctx)
	require.NoError(t, err)

	if usedQuota > 0 {
		insertRefundQuoteUsageFact(t, scenario.ctx, scenario.client, period.ID, scenario.userID, usedQuota)
	}
	return refundQuoteEntitlementFixture{
		groupID:        groupEntity.ID,
		subscriptionID: subscriptionEntity.ID,
		entitlementID:  period.ID,
		startsAt:       startsAt,
		expiresAt:      expiresAt,
	}
}

func ensureRefundQuoteUsageFactsTable(t *testing.T, ctx context.Context, client *dbent.Client) {
	t.Helper()
	var result entsql.Result
	err := client.Driver().Exec(ctx, `
CREATE TABLE IF NOT EXISTS usage_facts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	request_id TEXT NOT NULL,
	api_key_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	account_id INTEGER NOT NULL,
	request_fingerprint TEXT NOT NULL,
	payload_version INTEGER NOT NULL DEFAULT 1,
	payload TEXT NOT NULL,
	billing_status TEXT NOT NULL DEFAULT 'pending',
	attempt_count INTEGER NOT NULL DEFAULT 0,
	next_attempt_at DATETIME NOT NULL,
	last_error TEXT NOT NULL DEFAULT '',
	completed_at DATETIME NOT NULL,
	settled_at DATETIME,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	entitlement_period_id INTEGER
)`, []any{}, &result)
	require.NoError(t, err)
}

func insertRefundQuoteUsageFact(t *testing.T, ctx context.Context, client *dbent.Client, periodID, userID int64, cost float64) {
	t.Helper()
	now := time.Now()
	var result entsql.Result
	err := client.Driver().Exec(ctx, `
INSERT INTO usage_facts (
	request_id, api_key_id, user_id, account_id, request_fingerprint,
	payload_version, payload, billing_status, attempt_count, next_attempt_at,
	last_error, completed_at, settled_at, created_at, updated_at, entitlement_period_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		[]any{
			"refund-quote-" + strconv.FormatInt(periodID, 10),
			int64(11),
			userID,
			int64(22),
			"refundquoterequestfingerprint000000000000000000000000000000",
			1,
			`{"billing_command":{"subscription_cost":` + strconv.FormatFloat(cost, 'f', -1, 64) + `}}`,
			"settled",
			0,
			now,
			"",
			now,
			now,
			now,
			now,
			periodID,
		},
		&result,
	)
	require.NoError(t, err)
}

func insertRefundQuoteUnallocatedUsageFact(t *testing.T, ctx context.Context, client *dbent.Client, subscriptionID, userID int64, completedAt time.Time, cost float64) {
	t.Helper()
	now := time.Now()
	var result entsql.Result
	err := client.Driver().Exec(ctx, `
INSERT INTO usage_facts (
	request_id, api_key_id, user_id, account_id, request_fingerprint,
	payload_version, payload, billing_status, attempt_count, next_attempt_at,
	last_error, completed_at, settled_at, created_at, updated_at, entitlement_period_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		[]any{
			"refund-quote-unallocated-" + strconv.FormatInt(subscriptionID, 10),
			int64(11),
			userID,
			int64(22),
			"refundquoteunallocatedfingerprint0000000000000000000000000",
			1,
			`{"billing_command":{"SubscriptionID":` + strconv.FormatInt(subscriptionID, 10) + `,"subscription_cost":` + strconv.FormatFloat(cost, 'f', -1, 64) + `}}`,
			"settled",
			0,
			now,
			"",
			completedAt,
			now,
			now,
			now,
			nil,
		},
		&result,
	)
	require.NoError(t, err)
}

func newOfflinePaymentRefundScenario(t *testing.T) (autoGatewayRefundScenario, *refundProviderStub, time.Time, float64) {
	t.Helper()
	provider := &refundProviderStub{responses: []*payment.RefundResponse{{RefundID: "unexpected", Status: payment.ProviderStatusSuccess}}}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	originalBalance := 17.5
	_, err := scenario.client.User.UpdateOneID(scenario.userID).SetBalance(originalBalance).Save(scenario.ctx)
	require.NoError(t, err)
	_, err = scenario.client.PaymentOrder.UpdateOneID(scenario.orderID).
		SetPaymentType(payment.TypeOffline).
		Save(scenario.ctx)
	require.NoError(t, err)
	subscription, err := scenario.subRepo.GetByID(scenario.ctx, scenario.subID)
	require.NoError(t, err)
	return scenario, provider, subscription.ExpiresAt, originalBalance
}

func requireOfflinePaymentRefundRemainsUnchanged(t *testing.T, scenario autoGatewayRefundScenario, provider *refundProviderStub, originalExpiry time.Time, originalBalance float64, err error) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, "OFFLINE_PAYMENT_MANUAL_REFUND_ONLY", infraerrors.Reason(err))
	reloadedOrder, loadErr := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, loadErr)
	require.Equal(t, OrderStatusCompleted, reloadedOrder.Status)
	reloadedSubscription, loadErr := scenario.subRepo.GetByID(scenario.ctx, scenario.subID)
	require.NoError(t, loadErr)
	require.Equal(t, originalExpiry, reloadedSubscription.ExpiresAt)
	reloadedUser, loadErr := scenario.client.User.Get(scenario.ctx, scenario.userID)
	require.NoError(t, loadErr)
	require.Equal(t, originalBalance, reloadedUser.Balance)
	require.Empty(t, provider.requests)
}

func TestOfflinePaymentRequestRefundRequiresManualHandling(t *testing.T) {
	scenario, provider, originalExpiry, originalBalance := newOfflinePaymentRefundScenario(t)

	err := scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "offline refund")

	requireOfflinePaymentRefundRemainsUnchanged(t, scenario, provider, originalExpiry, originalBalance, err)
}

func TestOfflinePaymentPrepareRefundRequiresManualHandling(t *testing.T) {
	scenario, provider, originalExpiry, originalBalance := newOfflinePaymentRefundScenario(t)

	plan, result, err := scenario.svc.PrepareRefund(scenario.ctx, scenario.orderID, 29, "offline refund", false, true)

	require.Nil(t, plan)
	require.Nil(t, result)
	requireOfflinePaymentRefundRemainsUnchanged(t, scenario, provider, originalExpiry, originalBalance, err)
}

func TestOfflinePaymentExecuteRefundRequiresManualHandling(t *testing.T) {
	scenario, provider, originalExpiry, originalBalance := newOfflinePaymentRefundScenario(t)

	result, err := scenario.svc.ExecuteRefund(scenario.ctx, &RefundPlan{
		OrderID:        scenario.orderID,
		RefundAmount:   29,
		GatewayAmount:  29,
		Reason:         "offline refund",
		DeductionType:  payment.DeductionTypeSubscription,
		SubscriptionID: scenario.subID,
	})

	require.Nil(t, result)
	requireOfflinePaymentRefundRemainsUnchanged(t, scenario, provider, originalExpiry, originalBalance, err)
}

func TestCalculateSubscriptionRefundAmountUsesBeijingCalendarDays(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	startsAt := time.Date(2026, 7, 1, 23, 0, 0, 0, location)
	tests := []struct {
		name string
		now  time.Time
		want float64
	}{
		{name: "购买当天", now: time.Date(2026, 7, 1, 23, 30, 0, 0, location), want: 28.0},
		{name: "第六个自然日", now: time.Date(2026, 7, 6, 1, 0, 0, 0, location), want: 23.2},
		{name: "第30天", now: time.Date(2026, 7, 30, 0, 1, 0, 0, location), want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, calculateSubscriptionRefundAmount(29, 30, startsAt, tt.now))
		})
	}
}

func TestPaymentOrderRefundRetryableUsesPersistedStage(t *testing.T) {
	tests := []struct {
		name  string
		order *dbent.PaymentOrder
		want  bool
	}{
		{
			name:  "网关明确失败",
			order: &dbent.PaymentOrder{Status: OrderStatusRefundFailed, RefundGatewayStatus: RefundGatewayFailed},
			want:  true,
		},
		{
			name:  "网关成功但撤权失败",
			order: &dbent.PaymentOrder{Status: OrderStatusRefundFailed, RefundGatewayStatus: RefundGatewaySucceeded, RefundEntitlementStatus: RefundEntitlementFailed},
			want:  true,
		},
		{
			name:  "网关结果未知",
			order: &dbent.PaymentOrder{Status: OrderStatusRefundFailed, RefundGatewayStatus: RefundGatewayUnknown},
			want:  false,
		},
		{
			name:  "网关处理中",
			order: &dbent.PaymentOrder{Status: OrderStatusRefunding, RefundGatewayStatus: RefundGatewayPending},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, paymentOrderRefundRetryable(tt.order))
		})
	}
}

func TestRequestRefundRejectedGatewayIsRetryableAndReusesFacts(t *testing.T) {
	provider := &refundProviderStub{
		responses: []*payment.RefundResponse{nil, {RefundID: "refund-provider-1", Status: payment.ProviderStatusSuccess}},
		errors:    []error{&payment.RefundRejectedError{Err: errors.New("卖家余额不足")}, nil},
	}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	attachRefundQuoteEntitlement(t, &scenario, 58, 232, 46.4)

	err := scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "首次退款")
	require.Error(t, err)
	failedOrder, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.Equal(t, RefundGatewayFailed, failedOrder.RefundGatewayStatus)
	require.True(t, paymentOrderRefundRetryable(failedOrder))
	require.NotNil(t, failedOrder.RefundRequestID)
	requestID := *failedOrder.RefundRequestID
	refundAmount := failedOrder.RefundAmount

	retryStartsAt := time.Now().AddDate(0, 0, -10)
	scenario.subRepo.byID[scenario.subID].StartsAt = retryStartsAt
	scenario.subRepo.byID[scenario.subID].ExpiresAt = retryStartsAt.AddDate(0, 0, 30)
	err = scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "第二次退款")
	require.NoError(t, err)
	require.Len(t, provider.requests, 2)
	require.Equal(t, requestID, provider.requests[0].RequestID)
	require.Equal(t, requestID, provider.requests[1].RequestID)
	require.Equal(t, provider.requests[0].Amount, provider.requests[1].Amount)

	reloaded, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.Equal(t, refundAmount, reloaded.RefundAmount)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
}

func TestRequestRefundUnknownGatewayResultIsNotRetryable(t *testing.T) {
	provider := &refundProviderStub{errors: []error{errors.New("connection reset")}}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	attachRefundQuoteEntitlement(t, &scenario, 58, 232, 0)

	err := scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "未知结果")
	require.Error(t, err)
	failedOrder, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.Equal(t, RefundGatewayUnknown, failedOrder.RefundGatewayStatus)
	require.False(t, paymentOrderRefundRetryable(failedOrder))

	err = scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "再次尝试")
	require.Error(t, err)
	require.Len(t, provider.requests, 1)
}

func TestRequestRefundPendingDoesNotRevokeSubscription(t *testing.T) {
	provider := &refundProviderStub{responses: []*payment.RefundResponse{{RefundID: "refund-pending", Status: payment.ProviderStatusPending}}}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	attachRefundQuoteEntitlement(t, &scenario, 58, 232, 0)

	err := scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "处理中")
	require.NoError(t, err)
	reloaded, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunding, reloaded.Status)
	require.Equal(t, RefundGatewayPending, reloaded.RefundGatewayStatus)
	require.Equal(t, RefundEntitlementNotStarted, reloaded.RefundEntitlementStatus)
	_, err = scenario.subRepo.GetByID(scenario.ctx, scenario.subID)
	require.NoError(t, err)
}

func TestRequestRefundRevokesPaymentOrderEntitlementSource(t *testing.T) {
	provider := &refundProviderStub{responses: []*payment.RefundResponse{{RefundID: "refund-success", Status: payment.ProviderStatusSuccess}}}
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	attachRefundQuoteEntitlement(t, &scenario, 58, 232, 0)
	scenario.svc.subscriptionSvc.entitlementPeriodRepo = entitlementRepo

	err := scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "撤销权益周期")

	require.NoError(t, err)
	require.Equal(t, []SubscriptionEntitlementSource{{
		Type: "payment_order",
		ID:   strconv.FormatInt(scenario.orderID, 10),
	}}, entitlementRepo.revokeSourceCalls)
	require.Equal(t, []string{"payment_refund"}, entitlementRepo.revokeReasons)
}

func TestRequestRefundRetryAfterGatewaySuccessOnlyRevokesEntitlement(t *testing.T) {
	provider := &refundProviderStub{responses: []*payment.RefundResponse{{RefundID: "refund-success", Status: payment.ProviderStatusSuccess}}}
	baseRepo := newSubscriptionUserSubRepoStub()
	failingRepo := &refundDeleteOnceRepo{subscriptionUserSubRepoStub: baseRepo}
	scenario := newAutoGatewayRefundScenario(t, provider, baseRepo)
	scenario.svc.subscriptionSvc = NewSubscriptionService(&subscriptionGroupRepoStub{group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription}}, failingRepo, nil, nil, nil)
	attachRefundQuoteEntitlement(t, &scenario, 58, 232, 0)
	scenario.svc.subscriptionSvc.entitlementPeriodRepo = &refundRevokeOnceEntitlementRepo{
		refundQuoteEntitlementRepo: &refundQuoteEntitlementRepo{client: scenario.client},
		err:                        errors.New("revoke entitlement failed"),
	}

	err := scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "撤权失败")
	require.Error(t, err)
	failedOrder, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.Equal(t, RefundGatewaySucceeded, failedOrder.RefundGatewayStatus)
	require.Equal(t, RefundEntitlementFailed, failedOrder.RefundEntitlementStatus)
	require.True(t, paymentOrderRefundRetryable(failedOrder))
	require.Len(t, provider.requests, 1)

	err = scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "只重试撤权")
	require.NoError(t, err)
	require.Len(t, provider.requests, 1)
	subscription, err := scenario.subRepo.GetByID(scenario.ctx, scenario.subID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusExpired, subscription.Status)
}

func TestRequestRefundContinuesGatewaySucceededRefundingState(t *testing.T) {
	provider := &refundProviderStub{}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	attachRefundQuoteEntitlement(t, &scenario, 58, 232, 0)
	requestID := "refund-continuation"
	_, err := scenario.client.PaymentOrder.UpdateOneID(scenario.orderID).
		SetStatus(OrderStatusRefunding).
		SetRefundAmount(23.2).
		SetRefundRequestID(requestID).
		SetRefundRequestReason("继续收尾").
		SetRefundGatewayStatus(RefundGatewaySucceeded).
		SetRefundEntitlementStatus(RefundEntitlementNotStarted).
		Save(scenario.ctx)
	require.NoError(t, err)
	require.NoError(t, scenario.svc.createAuditLogWithClient(scenario.ctx, scenario.client, scenario.orderID, "REFUND_SUCCESS", "system", map[string]any{
		"refundAmount": 23.2, "reason": "上次提交结果未知",
	}))

	err = scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "再次提交")
	require.NoError(t, err)
	require.Empty(t, provider.requests)
	subscription, err := scenario.subRepo.GetByID(scenario.ctx, scenario.subID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusExpired, subscription.Status)
	reloaded, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	require.Equal(t, RefundEntitlementSucceeded, reloaded.RefundEntitlementStatus)
	auditCount, err := scenario.client.PaymentAuditLog.Query().Count(scenario.ctx)
	require.NoError(t, err)
	require.Equal(t, 2, auditCount)
}

func TestRequestRefundMarksGatewaySucceededChangedEntitlementForManualReview(t *testing.T) {
	provider := &refundProviderStub{}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	requestID := "refund-manual-review"
	_, err := scenario.client.PaymentOrder.UpdateOneID(scenario.orderID).
		SetStatus(OrderStatusRefunding).
		SetRefundAmount(23.2).
		SetRefundRequestID(requestID).
		SetRefundRequestReason("权益已变化").
		SetRefundGatewayStatus(RefundGatewaySucceeded).
		SetRefundEntitlementStatus(RefundEntitlementNotStarted).
		Save(scenario.ctx)
	require.NoError(t, err)
	scenario.subRepo.byID[scenario.subID].ExpiresAt = scenario.subRepo.byID[scenario.subID].ExpiresAt.AddDate(0, 0, 7)

	err = scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "再次提交")
	require.Error(t, err)
	require.Equal(t, "SUBSCRIPTION_TERM_CHANGED_REFUND_REQUIRES_MANUAL", infraerrors.Reason(err))
	require.Empty(t, provider.requests)
	_, err = scenario.subRepo.GetByID(scenario.ctx, scenario.subID)
	require.NoError(t, err)
	reloaded, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundFailed, reloaded.Status)
	require.Equal(t, RefundGatewaySucceeded, reloaded.RefundGatewayStatus)
	require.Equal(t, RefundEntitlementManual, reloaded.RefundEntitlementStatus)
	require.False(t, paymentOrderRefundRetryable(reloaded))
}

func TestGatewayRefundFinalizationRollsBackEntitlementWhenOrderFinalizationFails(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	userEntity, err := client.User.Create().
		SetEmail("gateway-refund-finalization@example.com").
		SetPasswordHash("hash").
		SetUsername("gateway-refund-finalization-user").
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)
	groupEntity, err := client.Group.Create().
		SetName("gateway-refund-finalization-group").
		SetSubscriptionType(SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	startsAt := time.Now().AddDate(0, 0, -5)
	subscriptionEntity, err := client.UserSubscription.Create().
		SetUserID(userEntity.ID).
		SetGroupID(groupEntity.ID).
		SetStartsAt(startsAt).
		SetExpiresAt(startsAt.AddDate(0, 0, 30)).
		SetStatus(SubscriptionStatusActive).
		Save(ctx)
	require.NoError(t, err)
	requestID := "refund-finalization-rollback"
	order, err := client.PaymentOrder.Create().
		SetUserID(userEntity.ID).
		SetUserEmail(userEntity.Email).
		SetUserName(userEntity.Username).
		SetAmount(29).
		SetPayAmount(29.29).
		SetFeeRate(1).
		SetRechargeCode("GATEWAY-REFUND-FINALIZATION").
		SetOutTradeNo("sub2_gateway_refund_finalization").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-gateway-refund-finalization").
		SetOrderType(payment.OrderTypeSubscription).
		SetSubscriptionGroupID(groupEntity.ID).
		SetSubscriptionDays(30).
		SetSubscriptionID(subscriptionEntity.ID).
		SetStatus(OrderStatusRefunding).
		SetRefundAmount(23.2).
		SetRefundRequestID(requestID).
		SetRefundRequestReason("事务收尾").
		SetRefundGatewayStatus(RefundGatewaySucceeded).
		SetRefundEntitlementStatus(RefundEntitlementNotStarted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.test").
		Save(ctx)
	require.NoError(t, err)

	client.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			if paymentOrderMutation, ok := mutation.(*dbent.PaymentOrderMutation); ok {
				if status, exists := paymentOrderMutation.RefundEntitlementStatus(); exists && status == RefundEntitlementSucceeded {
					return nil, errors.New("final order update failed")
				}
			}
			return next.Mutate(ctx, mutation)
		})
	})
	repo := &refundEntSubscriptionRepo{client: client}
	svc := &PaymentService{
		entClient: client,
		subscriptionSvc: NewSubscriptionService(&subscriptionGroupRepoStub{
			group: &Group{ID: groupEntity.ID, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
		}, repo, nil, client, nil),
	}

	_, err = svc.completeGatewaySubscriptionRefundTransaction(ctx, order.ID, "事务收尾", "user", false)
	require.ErrorContains(t, err, "final order update failed")

	reloadedSubscription, err := client.UserSubscription.Get(ctx, subscriptionEntity.ID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusActive, reloadedSubscription.Status)
	require.Nil(t, reloadedSubscription.DeletedAt)
	reloadedOrder, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundFailed, reloadedOrder.Status)
	require.Equal(t, RefundGatewaySucceeded, reloadedOrder.RefundGatewayStatus)
	require.Equal(t, RefundEntitlementFailed, reloadedOrder.RefundEntitlementStatus)
}

func TestRequestRefundRejectsSharedSubscriptionEntitlement(t *testing.T) {
	provider := &refundProviderStub{responses: []*payment.RefundResponse{{RefundID: "unexpected", Status: payment.ProviderStatusSuccess}}}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	_, err := scenario.client.PaymentOrder.Create().
		SetUserID(scenario.userID).
		SetUserEmail("refund-state@example.com").
		SetUserName("refund-state-user").
		SetAmount(29).
		SetPayAmount(29.29).
		SetFeeRate(1).
		SetRechargeCode("REFUND-STATE-SHARED").
		SetOutTradeNo("sub2_refund_state_shared").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-refund-state-shared").
		SetOrderType(payment.OrderTypeSubscription).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(30).
		SetSubscriptionID(scenario.subID).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetCompletedAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.test").
		Save(scenario.ctx)
	require.NoError(t, err)

	err = scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "共享订阅退款")
	require.Error(t, err)
	require.Equal(t, "SHARED_SUBSCRIPTION_REFUND_REQUIRES_MANUAL", infraerrors.Reason(err))
	require.Empty(t, provider.requests)
}

func TestRequestRefundRejectsChangedSubscriptionTerm(t *testing.T) {
	provider := &refundProviderStub{responses: []*payment.RefundResponse{{RefundID: "unexpected", Status: payment.ProviderStatusSuccess}}}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	scenario.subRepo.byID[scenario.subID].ExpiresAt = scenario.subRepo.byID[scenario.subID].ExpiresAt.AddDate(0, 0, 7)

	err := scenario.svc.RequestRefund(scenario.ctx, scenario.orderID, scenario.userID, "已扩展权益退款")
	require.Error(t, err)
	require.Equal(t, "SUBSCRIPTION_TERM_CHANGED_REFUND_REQUIRES_MANUAL", infraerrors.Reason(err))
	require.Empty(t, provider.requests)
}

func TestGwRefundUsesOutTradeNoWithoutPaymentTradeNo(t *testing.T) {
	provider := &refundProviderStub{responses: []*payment.RefundResponse{{RefundID: "refund-success", Status: payment.ProviderStatusSuccess}}}
	svc := &PaymentService{refundProvider: provider}
	requestID := "refund-order-7"
	order := &dbent.PaymentOrder{ID: 7, OutTradeNo: "out-trade-7", RefundRequestID: &requestID}

	resp, err := svc.gwRefund(context.Background(), &RefundPlan{OrderID: order.ID, Order: order, RefundAmount: 1, GatewayAmount: 1, Reason: "test"})
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusSuccess, resp.Status)
	require.Len(t, provider.requests, 1)
	require.Equal(t, "out-trade-7", provider.requests[0].OrderID)
	require.Empty(t, provider.requests[0].TradeNo)
	require.Equal(t, requestID, provider.requests[0].RequestID)
}

func TestPrepareRefundUnknownGatewayResultRequiresManualReconciliation(t *testing.T) {
	provider := &refundProviderStub{}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	requestID := "refund-manual-unknown"
	_, err := scenario.client.PaymentOrder.UpdateOneID(scenario.orderID).
		SetStatus(OrderStatusRefundFailed).
		SetRefundAmount(23.2).
		SetRefundRequestID(requestID).
		SetRefundGatewayStatus(RefundGatewayUnknown).
		Save(scenario.ctx)
	require.NoError(t, err)

	plan, result, err := scenario.svc.PrepareRefund(scenario.ctx, scenario.orderID, 0, "", false, true)
	require.Nil(t, plan)
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "REFUND_RECONCILIATION_REQUIRED", infraerrors.Reason(err))
}

func TestPrepareRefundRetryReusesOriginalAmount(t *testing.T) {
	provider := &refundProviderStub{}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	requestID := "refund-admin-retry"
	_, err := scenario.client.PaymentOrder.UpdateOneID(scenario.orderID).
		SetStatus(OrderStatusRefundFailed).
		SetRefundAmount(23.2).
		SetRefundRequestID(requestID).
		SetRefundRequestReason("原退款原因").
		SetRefundGatewayStatus(RefundGatewayFailed).
		Save(scenario.ctx)
	require.NoError(t, err)

	plan, result, err := scenario.svc.PrepareRefund(scenario.ctx, scenario.orderID, 10, "新退款原因", false, false)
	require.NoError(t, err)
	require.Nil(t, result)
	require.NotNil(t, plan)
	require.Equal(t, 23.2, plan.RefundAmount)
	require.Equal(t, "原退款原因", plan.Reason)
	require.NotNil(t, plan.Order.RefundRequestID)
	require.Equal(t, requestID, *plan.Order.RefundRequestID)
}

func TestAdminSubscriptionRefundQuoteUsesEntitlementUsageFacts(t *testing.T) {
	provider := &refundProviderStub{}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	fixture := attachRefundQuoteEntitlement(t, &scenario, 58, 232, 29)

	quote, err := scenario.svc.AdminGetSubscriptionRefundQuote(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.True(t, quote.Eligible)
	require.False(t, quote.ManualReviewRequired)
	require.Equal(t, fixture.entitlementID, quote.EntitlementPeriodID)
	require.InDelta(t, 29, quote.PurchaseBaseAmount, 1e-9)
	require.InDelta(t, 0.29, quote.NonRefundableFee, 1e-9)
	require.InDelta(t, 232, quote.PeriodTotalQuotaUSD, 1e-9)
	require.InDelta(t, 29, quote.UsedQuotaUSD, 1e-9)
	require.InDelta(t, 0.125, quote.UsageRatio, 1e-9)
	require.InDelta(t, 25.375, quote.EstimatedRefundAmount, 1e-9)
}

func TestAdminSubscriptionRefundQuoteRequiresManualReviewForOverlappingEntitlement(t *testing.T) {
	provider := &refundProviderStub{}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	fixture := attachRefundQuoteEntitlement(t, &scenario, 58, 232, 0)

	_, err := scenario.client.SubscriptionEntitlementPeriod.Create().
		SetUserID(scenario.userID).
		SetSubscriptionID(fixture.subscriptionID).
		SetGroupID(fixture.groupID).
		SetSourceType(subscriptionEntitlementSourceTypePaymentOrder).
		SetSourceID("overlapping-renewal").
		SetStartsAt(fixture.startsAt).
		SetExpiresAt(fixture.expiresAt).
		SetPeriodDays(28).
		SetWeeklyLimitUsd(58).
		SetPeriodTotalQuotaUsd(232).
		SetQuotaWindowUnit("week").
		SetQuotaWindowDays(7).
		SetStatus(SubscriptionEntitlementPeriodStatusActive).
		Save(scenario.ctx)
	require.NoError(t, err)

	quote, err := scenario.svc.AdminGetSubscriptionRefundQuote(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.True(t, quote.ManualReviewRequired)
	require.False(t, quote.Eligible)
}

func TestAdminSubscriptionRefundQuoteRequiresManualReviewForUnallocatedUsageFact(t *testing.T) {
	provider := &refundProviderStub{}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	fixture := attachRefundQuoteEntitlement(t, &scenario, 58, 232, 0)
	insertRefundQuoteUnallocatedUsageFact(t, scenario.ctx, scenario.client, fixture.subscriptionID, scenario.userID, fixture.startsAt.Add(time.Hour), 18)

	quote, err := scenario.svc.AdminGetSubscriptionRefundQuote(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.True(t, quote.ManualReviewRequired)
	require.False(t, quote.Eligible)
}

func TestPrepareRefundUsesSubscriptionQuoteAndPersistsBasis(t *testing.T) {
	provider := &refundProviderStub{}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	fixture := attachRefundQuoteEntitlement(t, &scenario, 76, 304, 58)

	plan, earlyResult, err := scenario.svc.PrepareRefund(scenario.ctx, scenario.orderID, 29, "管理员按规则退款", false, false)
	require.NoError(t, err)
	require.Nil(t, earlyResult)
	require.NotNil(t, plan)
	require.InDelta(t, 23.463815789473685, plan.RefundAmount, 1e-9)

	reloaded, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.Equal(t, fixture.entitlementID, int64(reloaded.RefundBasis["entitlement_period_id"].(float64)))
	require.InDelta(t, 304, reloaded.RefundBasis["period_total_quota_usd"].(float64), 1e-9)
	require.InDelta(t, 58, reloaded.RefundBasis["used_quota_usd"].(float64), 1e-9)
	require.InDelta(t, 0.19078947368421054, reloaded.RefundBasis["usage_ratio"].(float64), 1e-9)
	require.InDelta(t, 29, reloaded.RefundBasis["purchase_base_amount"].(float64), 1e-9)
	require.InDelta(t, 0.29, reloaded.RefundBasis["non_refundable_fee"].(float64), 1e-9)
}

func TestExecuteRefundRecalculatesAdminSubscriptionQuoteInsideTransaction(t *testing.T) {
	provider := &refundProviderStub{responses: []*payment.RefundResponse{{RefundID: "refund-admin-requote", Status: payment.ProviderStatusSuccess}}}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	fixture := attachRefundQuoteEntitlement(t, &scenario, 58, 232, 0)

	plan, earlyResult, err := scenario.svc.PrepareRefund(scenario.ctx, scenario.orderID, 29, "管理员事务内重算", false, false)
	require.NoError(t, err)
	require.Nil(t, earlyResult)
	require.NotNil(t, plan)
	require.InDelta(t, 29, plan.RefundAmount, 1e-9)

	insertRefundQuoteUsageFact(t, scenario.ctx, scenario.client, fixture.entitlementID, scenario.userID, 58)

	result, err := scenario.svc.ExecuteRefund(scenario.ctx, plan)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Len(t, provider.requests, 1)
	require.Equal(t, "21.75", provider.requests[0].Amount)

	reloaded, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.InDelta(t, 21.75, reloaded.RefundAmount, 1e-9)
	require.Equal(t, fixture.entitlementID, int64(reloaded.RefundBasis["entitlement_period_id"].(float64)))
	require.InDelta(t, 58, reloaded.RefundBasis["used_quota_usd"].(float64), 1e-9)
	require.InDelta(t, 0.25, reloaded.RefundBasis["usage_ratio"].(float64), 1e-9)
}

func TestGatewaySubscriptionRefundRevokesOnlyTargetEntitlementPeriod(t *testing.T) {
	provider := &refundProviderStub{responses: []*payment.RefundResponse{{RefundID: "refund-target-only", Status: payment.ProviderStatusSuccess}}}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	fixture := attachRefundQuoteEntitlement(t, &scenario, 58, 232, 0)
	future, err := scenario.client.SubscriptionEntitlementPeriod.Create().
		SetUserID(scenario.userID).
		SetSubscriptionID(fixture.subscriptionID).
		SetGroupID(fixture.groupID).
		SetSourceType(subscriptionEntitlementSourceTypePaymentOrder).
		SetSourceID("future-renewal").
		SetStartsAt(fixture.expiresAt).
		SetExpiresAt(fixture.expiresAt.AddDate(0, 0, 28)).
		SetPeriodDays(28).
		SetWeeklyLimitUsd(58).
		SetPeriodTotalQuotaUsd(232).
		SetQuotaWindowUnit("week").
		SetQuotaWindowDays(7).
		SetStatus(SubscriptionEntitlementPeriodStatusActive).
		Save(scenario.ctx)
	require.NoError(t, err)

	plan, earlyResult, err := scenario.svc.PrepareRefund(scenario.ctx, scenario.orderID, 29, "管理员退款目标权益", false, false)
	require.NoError(t, err)
	require.Nil(t, earlyResult)
	result, err := scenario.svc.ExecuteRefund(scenario.ctx, plan)
	require.NoError(t, err)
	require.True(t, result.Success)

	target, err := scenario.client.SubscriptionEntitlementPeriod.Get(scenario.ctx, fixture.entitlementID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionEntitlementPeriodStatusRevoked, target.Status)
	require.Equal(t, "payment_refund", target.RevokedReason)
	reloadedFuture, err := scenario.client.SubscriptionEntitlementPeriod.Get(scenario.ctx, future.ID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionEntitlementPeriodStatusActive, reloadedFuture.Status)
	require.Equal(t, fixture.expiresAt, reloadedFuture.StartsAt)

	subscription, err := scenario.subRepo.GetByID(scenario.ctx, fixture.subscriptionID)
	require.NoError(t, err)
	require.Equal(t, future.ExpiresAt, subscription.ExpiresAt)
	require.Equal(t, SubscriptionStatusActive, subscription.Status)

	gapGroup := &Group{
		ID:               fixture.groupID,
		Name:             "codex-pool-19-usd",
		Status:           payment.EntityStatusActive,
		SubscriptionType: SubscriptionTypeSubscription,
		WeeklyLimitUSD:   ptrFloat64(58),
	}
	_, gapErr := scenario.svc.subscriptionSvc.ValidateAndCheckLimits(subscription, gapGroup)
	require.ErrorIs(t, gapErr, ErrSubscriptionInvalid)

	window, ok := subscription.RollingWeeklyWindowForEntitlement(gapGroup, &SubscriptionEntitlementPeriod{
		ID:             reloadedFuture.ID,
		StartsAt:       reloadedFuture.StartsAt,
		ExpiresAt:      reloadedFuture.ExpiresAt,
		Status:         SubscriptionEntitlementPeriodStatusActive,
		WeeklyLimitUSD: ptrFloat64(58),
	}, reloadedFuture.StartsAt.Add(time.Hour))
	require.True(t, ok)
	require.Equal(t, reloadedFuture.StartsAt, window.Start)
}

func TestExecuteRefundAfterGatewaySuccessOnlyRetriesEntitlement(t *testing.T) {
	provider := &refundProviderStub{}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	fixture := attachRefundQuoteEntitlement(t, &scenario, 58, 232, 0)
	requestID := "refund-admin-entitlement"
	order, err := scenario.client.PaymentOrder.UpdateOneID(scenario.orderID).
		SetStatus(OrderStatusRefundFailed).
		SetRefundAmount(23.2).
		SetRefundRequestID(requestID).
		SetRefundRequestReason("管理员退款").
		SetRefundGatewayStatus(RefundGatewaySucceeded).
		SetRefundEntitlementStatus(RefundEntitlementFailed).
		Save(scenario.ctx)
	require.NoError(t, err)

	result, err := scenario.svc.ExecuteRefund(scenario.ctx, &RefundPlan{
		OrderID:        order.ID,
		Order:          order,
		RefundAmount:   order.RefundAmount,
		GatewayAmount:  order.RefundAmount,
		Reason:         "管理员退款",
		DeductionType:  payment.DeductionTypeSubscription,
		SubscriptionID: scenario.subID,
	})
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Empty(t, provider.requests)
	target, err := scenario.client.SubscriptionEntitlementPeriod.Get(scenario.ctx, fixture.entitlementID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionEntitlementPeriodStatusRevoked, target.Status)

	reloaded, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	require.Equal(t, RefundGatewaySucceeded, reloaded.RefundGatewayStatus)
	require.Equal(t, RefundEntitlementSucceeded, reloaded.RefundEntitlementStatus)
}

func TestExecuteRefundContinuesGatewaySucceededRefundingState(t *testing.T) {
	provider := &refundProviderStub{}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	fixture := attachRefundQuoteEntitlement(t, &scenario, 58, 232, 0)
	requestID := "refund-admin-continuation"
	order, err := scenario.client.PaymentOrder.UpdateOneID(scenario.orderID).
		SetStatus(OrderStatusRefunding).
		SetRefundAmount(23.2).
		SetRefundRequestID(requestID).
		SetRefundRequestReason("管理员继续收尾").
		SetRefundGatewayStatus(RefundGatewaySucceeded).
		SetRefundEntitlementStatus(RefundEntitlementNotStarted).
		Save(scenario.ctx)
	require.NoError(t, err)

	result, err := scenario.svc.ExecuteRefund(scenario.ctx, &RefundPlan{
		OrderID:        order.ID,
		Order:          order,
		RefundAmount:   order.RefundAmount,
		GatewayAmount:  order.RefundAmount,
		Reason:         "管理员继续收尾",
		DeductionType:  payment.DeductionTypeSubscription,
		SubscriptionID: scenario.subID,
	})
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Empty(t, provider.requests)
	target, err := scenario.client.SubscriptionEntitlementPeriod.Get(scenario.ctx, fixture.entitlementID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionEntitlementPeriodStatusRevoked, target.Status)
	reloaded, err := scenario.client.PaymentOrder.Get(scenario.ctx, scenario.orderID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	require.Equal(t, RefundEntitlementSucceeded, reloaded.RefundEntitlementStatus)
}

func TestAdminSubscriptionRefundCannotSkipEntitlementRevocation(t *testing.T) {
	provider := &refundProviderStub{responses: []*payment.RefundResponse{{RefundID: "refund-admin", Status: payment.ProviderStatusSuccess}}}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)

	plan, earlyResult, err := scenario.svc.PrepareRefund(scenario.ctx, scenario.orderID, 23.2, "管理员退款", false, false)
	require.Error(t, err)
	require.Equal(t, "REFUND_MANUAL_REVIEW_REQUIRED", infraerrors.Reason(err))
	require.Nil(t, plan)
	require.Nil(t, earlyResult)
	require.Empty(t, provider.requests)
}

func TestPrepareRefundRejectsSubscriptionWithoutLink(t *testing.T) {
	provider := &refundProviderStub{}
	scenario := newAutoGatewayRefundScenario(t, provider, nil)
	_, err := scenario.client.PaymentOrder.UpdateOneID(scenario.orderID).ClearSubscriptionID().Save(scenario.ctx)
	require.NoError(t, err)

	plan, result, err := scenario.svc.PrepareRefund(scenario.ctx, scenario.orderID, 23.2, "缺少关联", false, false)
	require.Nil(t, plan)
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "SUBSCRIPTION_LINK_REQUIRED", infraerrors.Reason(err))
	require.Empty(t, provider.requests)
}

func TestBalanceRefundRollsBackAllState(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	userEntity, err := client.User.Create().
		SetEmail("balance-refund-transaction@example.com").
		SetPasswordHash("hash").
		SetUsername("balance-refund-transaction-user").
		SetStatus(payment.EntityStatusActive).
		SetBalance(0).
		Save(ctx)
	require.NoError(t, err)
	groupEntity, err := client.Group.Create().
		SetName("balance-refund-transaction-group").
		SetSubscriptionType(SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	startsAt := time.Now().AddDate(0, 0, -5)
	subscriptionEntity, err := client.UserSubscription.Create().
		SetUserID(userEntity.ID).
		SetGroupID(groupEntity.ID).
		SetStartsAt(startsAt).
		SetExpiresAt(startsAt.AddDate(0, 0, 30)).
		SetStatus(SubscriptionStatusActive).
		Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().
		SetUserID(userEntity.ID).
		SetUserEmail(userEntity.Email).
		SetUserName(userEntity.Username).
		SetAmount(29).
		SetPayAmount(29.29).
		SetFeeRate(1).
		SetRechargeCode("BALANCE-REFUND-TRANSACTION").
		SetOutTradeNo("sub2_balance_refund_transaction").
		SetPaymentType(payment.TypeBalance).
		SetPaymentTradeNo("balance").
		SetOrderType(payment.OrderTypeSubscription).
		SetSubscriptionGroupID(groupEntity.ID).
		SetSubscriptionDays(30).
		SetSubscriptionID(subscriptionEntity.ID).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(startsAt).
		SetCompletedAt(startsAt).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.test").
		Save(ctx)
	require.NoError(t, err)

	repo := &refundEntSubscriptionRepo{client: client, failDelete: true}
	svc := &PaymentService{
		entClient: client,
		subscriptionSvc: NewSubscriptionService(&subscriptionGroupRepoStub{
			group: &Group{ID: groupEntity.ID, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
		}, repo, nil, client, nil),
	}

	err = svc.RequestRefund(ctx, order.ID, userEntity.ID, "事务回滚")
	require.Error(t, err)

	reloadedUser, err := client.User.Get(ctx, userEntity.ID)
	require.NoError(t, err)
	require.Equal(t, 0.0, reloadedUser.Balance)
	reloadedOrder, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloadedOrder.Status)
	reloadedSubscription, err := client.UserSubscription.Get(ctx, subscriptionEntity.ID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusActive, reloadedSubscription.Status)
	require.Nil(t, reloadedSubscription.DeletedAt)
	auditCount, err := client.PaymentAuditLog.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, auditCount)
}

func TestBalanceRefundRevalidatesSubscriptionTermInsideTransaction(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	userEntity, err := client.User.Create().
		SetEmail("balance-refund-revalidate@example.com").
		SetPasswordHash("hash").
		SetUsername("balance-refund-revalidate-user").
		SetStatus(payment.EntityStatusActive).
		SetBalance(0).
		Save(ctx)
	require.NoError(t, err)
	groupEntity, err := client.Group.Create().
		SetName("balance-refund-revalidate-group").
		SetSubscriptionType(SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	startsAt := time.Now().AddDate(0, 0, -5)
	subscriptionEntity, err := client.UserSubscription.Create().
		SetUserID(userEntity.ID).
		SetGroupID(groupEntity.ID).
		SetStartsAt(startsAt).
		SetExpiresAt(startsAt.AddDate(0, 0, 37)).
		SetStatus(SubscriptionStatusActive).
		Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().
		SetUserID(userEntity.ID).
		SetUserEmail(userEntity.Email).
		SetUserName(userEntity.Username).
		SetAmount(29).
		SetPayAmount(29.29).
		SetFeeRate(1).
		SetRechargeCode("BALANCE-REFUND-REVALIDATE").
		SetOutTradeNo("sub2_balance_refund_revalidate").
		SetPaymentType(payment.TypeBalance).
		SetPaymentTradeNo("balance").
		SetOrderType(payment.OrderTypeSubscription).
		SetSubscriptionGroupID(groupEntity.ID).
		SetSubscriptionDays(30).
		SetSubscriptionID(subscriptionEntity.ID).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.test").
		Save(ctx)
	require.NoError(t, err)

	repo := &refundEntSubscriptionRepo{client: client}
	svc := &PaymentService{
		entClient: client,
		subscriptionSvc: NewSubscriptionService(&subscriptionGroupRepoStub{
			group: &Group{ID: groupEntity.ID, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
		}, repo, nil, client, nil),
	}
	staleSubscription := &UserSubscription{
		ID: subscriptionEntity.ID, UserID: userEntity.ID, GroupID: groupEntity.ID,
		StartsAt: startsAt, ExpiresAt: startsAt.AddDate(0, 0, 30), Status: SubscriptionStatusActive,
	}

	err = svc.executeBalanceSubscriptionRefundTransaction(ctx, order, staleSubscription, 23.2, "事务内复验", "user", "user", "refund-revalidate", false, OrderStatusCompleted, time.Now())
	require.Error(t, err)
	require.Equal(t, "REFUND_MANUAL_REVIEW_REQUIRED", infraerrors.Reason(err))
	reloadedUser, err := client.User.Get(ctx, userEntity.ID)
	require.NoError(t, err)
	require.Zero(t, reloadedUser.Balance)
	reloadedOrder, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloadedOrder.Status)
}

func TestAdminBalanceSubscriptionRefundCreditsBalanceAndRevokesSubscription(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	userEntity, err := client.User.Create().
		SetEmail("admin-balance-refund@example.com").
		SetPasswordHash("hash").
		SetUsername("admin-balance-refund-user").
		SetStatus(payment.EntityStatusActive).
		SetBalance(0).
		Save(ctx)
	require.NoError(t, err)
	startsAt := time.Now().AddDate(0, 0, -5)
	subID := int64(301)
	order, err := client.PaymentOrder.Create().
		SetUserID(userEntity.ID).
		SetUserEmail(userEntity.Email).
		SetUserName(userEntity.Username).
		SetAmount(29).
		SetPayAmount(29.29).
		SetFeeRate(1).
		SetRechargeCode("ADMIN-BALANCE-REFUND").
		SetOutTradeNo("sub2_admin_balance_refund").
		SetPaymentType(payment.TypeBalance).
		SetPaymentTradeNo("balance").
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
	svc := &PaymentService{
		entClient: client,
		subscriptionSvc: NewSubscriptionService(&subscriptionGroupRepoStub{
			group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
		}, subRepo, nil, nil, nil),
	}
	scenario := autoGatewayRefundScenario{ctx: ctx, client: client, userID: userEntity.ID, orderID: order.ID, subID: subID, subRepo: subRepo, svc: svc}
	fixture := attachRefundQuoteEntitlement(t, &scenario, 58, 232, 0)

	plan, earlyResult, err := svc.PrepareRefund(ctx, order.ID, 29, "管理员余额退款", false, true)
	require.NoError(t, err)
	require.Nil(t, earlyResult)
	require.InDelta(t, 29, plan.RefundAmount, 1e-9)
	insertRefundQuoteUsageFact(t, ctx, client, fixture.entitlementID, userEntity.ID, 58)

	result, err := svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.InDelta(t, -21.75, result.BalanceDeducted, 1e-9)

	reloadedUser, err := client.User.Get(ctx, userEntity.ID)
	require.NoError(t, err)
	require.InDelta(t, 21.75, reloadedUser.Balance, 1e-9)
	reloadedOrder, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.InDelta(t, 21.75, reloadedOrder.RefundAmount, 1e-9)
	require.InDelta(t, 21.75, reloadedOrder.RefundBalanceAmount, 1e-9)
	require.Equal(t, RefundGatewaySucceeded, reloadedOrder.RefundBalanceStatus)
	require.Equal(t, fixture.entitlementID, int64(reloadedOrder.RefundBasis["entitlement_period_id"].(float64)))
	require.InDelta(t, 58, reloadedOrder.RefundBasis["used_quota_usd"].(float64), 1e-9)
	subscription, err := subRepo.GetByID(ctx, scenario.subID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusExpired, subscription.Status)
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
	startsAt := time.Now().AddDate(0, 0, -5)
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
		SetSubscriptionID(99).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(startsAt).
		SetCompletedAt(startsAt).
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
	svc := &PaymentService{
		entClient:    client,
		loadBalancer: newWebhookProviderTestLoadBalancer(client),
		subscriptionSvc: NewSubscriptionService(&subscriptionGroupRepoStub{
			group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
		}, subRepo, nil, nil, nil),
	}
	scenario := autoGatewayRefundScenario{ctx: ctx, client: client, userID: user.ID, orderID: order.ID, subID: 99, subRepo: subRepo, svc: svc}
	attachRefundQuoteEntitlement(t, &scenario, 58, 232, 46.4)

	err = svc.RequestRefund(ctx, order.ID, user.ID, "用户自动退款")
	require.NoError(t, err)
	require.Equal(t, "23.20", refundForm.Get("money"))
	require.Equal(t, "sub2_auto_alipay_refund", refundForm.Get("out_trade_no"))

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	require.InDelta(t, 23.2, reloaded.RefundAmount, 1e-9)

	subscription, err := subRepo.GetByID(ctx, scenario.subID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusExpired, subscription.Status)
}

func TestRequestRefundAutomaticallyRefundsBalanceSubscriptionWithoutFee(t *testing.T) {
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

	startsAt := time.Now().AddDate(0, 0, -5)
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
		SetSubscriptionID(100).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(startsAt).
		SetCompletedAt(startsAt).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	subRepo := newSubscriptionUserSubRepoStub()
	svc := &PaymentService{
		entClient: client,
		subscriptionSvc: NewSubscriptionService(&subscriptionGroupRepoStub{
			group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
		}, subRepo, nil, nil, nil),
	}
	scenario := autoGatewayRefundScenario{ctx: ctx, client: client, userID: user.ID, orderID: order.ID, subID: 100, subRepo: subRepo, svc: svc}
	attachRefundQuoteEntitlement(t, &scenario, 58, 232, 46.4)
	require.NoError(t, svc.createAuditLogWithClient(ctx, client, order.ID, "REFUND_REQUESTED", "system", map[string]any{
		"amount": 23.2, "reason": "旧退款请求",
	}))

	err = svc.RequestRefund(ctx, order.ID, user.ID, "余额自动退款")
	require.NoError(t, err)

	reloadedUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, 23.2, reloadedUser.Balance, 1e-9)

	reloadedOrder, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloadedOrder.Status)
	require.InDelta(t, 23.2, reloadedOrder.RefundAmount, 1e-9)
	auditCount, err := client.PaymentAuditLog.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, auditCount)

	subscription, err := subRepo.GetByID(ctx, scenario.subID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusExpired, subscription.Status)
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
