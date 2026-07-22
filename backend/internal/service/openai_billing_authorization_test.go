//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAIBillingAuthorizationReservationRepoStub struct {
	TrafficCreditReservationRepository
	availableUSD float64
	hasDebt      bool
	reservation  *TrafficCreditReservation
	reserveCalls int
}

func (s *openAIBillingAuthorizationReservationRepoStub) GetAvailableUSD(ctx context.Context, userID int64, platform string, now time.Time) (float64, error) {
	return s.availableUSD, nil
}

func (s *openAIBillingAuthorizationReservationRepoStub) HasOutstandingDebt(ctx context.Context, userID int64, platform string) (bool, error) {
	return s.hasDebt, nil
}

func (s *openAIBillingAuthorizationReservationRepoStub) Reserve(ctx context.Context, input TrafficCreditReservationInput) (*TrafficCreditReservation, bool, error) {
	s.reserveCalls++
	if s.reservation == nil {
		s.reservation = &TrafficCreditReservation{
			ID:                            91,
			TrafficCreditReservationInput: input,
			Status:                        TrafficCreditReservationReserved,
		}
		return s.reservation, true, nil
	}
	return s.reservation, false, nil
}

type openAIBillingAuthorizationEstimatorStub struct {
	budget *OpenAITrafficCreditBudget
	err    error
}

func (s *openAIBillingAuthorizationEstimatorStub) Estimate(ctx context.Context, input OpenAITrafficBudgetInput) (*OpenAITrafficCreditBudget, error) {
	return s.budget, s.err
}

func TestOpenAIBillingAuthorization_UsesSubscriptionWhenBudgetFits(t *testing.T) {
	dailyLimit := 10.0
	svc, repo := newOpenAIBillingAuthorizationTestService(0.25)
	input := newOpenAIBillingAuthorizationTestInput()
	input.Group = &Group{ID: 2, DailyLimitUSD: &dailyLimit}
	input.Subscription = &UserSubscription{ID: 3, DailyUsageUSD: 1}

	got, err := svc.Authorize(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, BillingSourceSubscription, got.Source)
	require.Nil(t, got.ReservationID)
	require.Zero(t, repo.reserveCalls)
}

func TestOpenAIBillingAuthorization_IgnoresBalanceAndReservesTrafficCreditWhenNoSubscription(t *testing.T) {
	svc, repo := newOpenAIBillingAuthorizationTestService(0.25)
	input := newOpenAIBillingAuthorizationTestInput()

	got, err := svc.Authorize(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, BillingSourceTrafficCredit, got.Source)
	require.Equal(t, int64(91), *got.ReservationID)
	require.Equal(t, 1, repo.reserveCalls)
}

func TestOpenAIBillingAuthorization_ShadowModeDoesNotRejectInsufficientTrafficCredit(t *testing.T) {
	repo := &openAIBillingAuthorizationReservationRepoStub{availableUSD: 0}
	estimator := &openAIBillingAuthorizationEstimatorStub{err: ErrTrafficCreditInsufficient}
	svc := NewOpenAIBillingAuthorizationService(repo, estimator, 15*time.Minute, false, true)
	input := newOpenAIBillingAuthorizationTestInput()

	got, err := svc.Authorize(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, BillingSourceShadow, got.Source)
	require.False(t, got.Enforced)
	require.Nil(t, got.ReservationID)
	require.Equal(t, input.Body, got.EffectiveBody)
	require.Zero(t, repo.reserveCalls)
}

func TestOpenAIBillingAuthorization_ReservesTrafficCreditWhenSubscriptionExceeded(t *testing.T) {
	dailyLimit := 1.1
	svc, repo := newOpenAIBillingAuthorizationTestService(0.25)
	input := newOpenAIBillingAuthorizationTestInput()
	input.Group = &Group{ID: 2, DailyLimitUSD: &dailyLimit}
	input.Subscription = &UserSubscription{ID: 3, DailyUsageUSD: 1}

	got, err := svc.Authorize(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, BillingSourceTrafficCredit, got.Source)
	require.NotNil(t, got.ReservationID)
	require.Equal(t, int64(91), *got.ReservationID)
	require.Equal(t, 1, repo.reserveCalls)
}

func TestOpenAIBillingAuthorization_RollingWeeklyIgnoresStaleWindowUsage(t *testing.T) {
	weeklyLimit := 58.0
	anchor := time.Now().UTC().AddDate(0, 0, -8).Truncate(time.Second)
	svc, repo := newOpenAIBillingAuthorizationTestService(56)
	input := newOpenAIBillingAuthorizationTestInput()
	input.Group = &Group{
		ID:               2,
		Name:             "codex-pool-19-usd",
		SubscriptionType: SubscriptionTypeSubscription,
		WeeklyLimitUSD:   &weeklyLimit,
	}
	input.Subscription = &UserSubscription{
		ID:                3,
		StartsAt:          anchor,
		ExpiresAt:         anchor.AddDate(0, 0, 28),
		WeeklyAnchorAt:    &anchor,
		WeeklyWindowStart: &anchor,
		WeeklyUsageUSD:    64,
		CurrentEntitlementPeriod: &SubscriptionEntitlementPeriod{
			ID:             30,
			StartsAt:       anchor,
			ExpiresAt:      anchor.AddDate(0, 0, 28),
			Status:         SubscriptionEntitlementPeriodStatusActive,
			WeeklyLimitUSD: &weeklyLimit,
		},
	}

	got, err := svc.Authorize(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, BillingSourceSubscription, got.Source)
	require.NotNil(t, got.EntitlementPeriodID)
	require.Equal(t, int64(30), *got.EntitlementPeriodID)
	require.Zero(t, repo.reserveCalls)
}

func TestOpenAIBillingAuthorization_RejectsOutstandingDebt(t *testing.T) {
	svc, repo := newOpenAIBillingAuthorizationTestService(0.25)
	repo.hasDebt = true

	_, err := svc.Authorize(context.Background(), newOpenAIBillingAuthorizationTestInput())

	require.ErrorIs(t, err, ErrTrafficCreditDebtOutstanding)
	require.Zero(t, repo.reserveCalls)
}

func TestOpenAIBillingAuthorization_ReusesReservationAcrossRetry(t *testing.T) {
	svc, repo := newOpenAIBillingAuthorizationTestService(0.25)
	input := newOpenAIBillingAuthorizationTestInput()

	first, err := svc.Authorize(context.Background(), input)
	require.NoError(t, err)
	second, err := svc.Authorize(context.Background(), input)
	require.NoError(t, err)

	require.Equal(t, BillingSourceTrafficCredit, first.Source)
	require.Equal(t, BillingSourceTrafficCredit, second.Source)
	require.Equal(t, first.ReservationID, second.ReservationID)
	require.Equal(t, 2, repo.reserveCalls)
}

func newOpenAIBillingAuthorizationTestService(reserveUSD float64) (*OpenAIBillingAuthorizationService, *openAIBillingAuthorizationReservationRepoStub) {
	repo := &openAIBillingAuthorizationReservationRepoStub{availableUSD: 10}
	estimator := &openAIBillingAuthorizationEstimatorStub{budget: &OpenAITrafficCreditBudget{
		Body:                     []byte(`{"model":"gpt-5.1","max_output_tokens":256}`),
		InputTokenUpperBound:     64,
		EffectiveMaxOutputTokens: 256,
		ReserveUSD:               reserveUSD,
		PricingSnapshot:          json.RawMessage(`{"model":"gpt-5.1"}`),
	}}
	return NewOpenAIBillingAuthorizationService(repo, estimator, 15*time.Minute, true, false), repo
}

func newOpenAIBillingAuthorizationTestInput() OpenAIBillingAuthorizationInput {
	return OpenAIBillingAuthorizationInput{
		RequestID:          "req-1",
		RequestFingerprint: "fingerprint-1",
		APIKeyID:           9,
		UserID:             7,
		Platform:           PlatformOpenAI,
		Model:              "gpt-5.1",
		Body:               []byte(`{"model":"gpt-5.1"}`),
		RateMultiplier:     1,
	}
}
