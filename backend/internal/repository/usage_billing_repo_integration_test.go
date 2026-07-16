//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestUsageBillingRepositoryApply_DeduplicatesBalanceBilling(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-" + uuid.NewString(),
		Name:   "billing",
		Quota:  1,
	})
	account := mustCreateAccount(t, client, &service.Account{
		Name: "usage-billing-account-" + uuid.NewString(),
		Type: service.AccountTypeAPIKey,
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:           requestID,
		APIKeyID:            apiKey.ID,
		UserID:              user.ID,
		AccountID:           account.ID,
		AccountType:         service.AccountTypeAPIKey,
		BalanceCost:         1.25,
		APIKeyQuotaCost:     1.25,
		APIKeyRateLimitCost: 1.25,
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, result1)
	require.True(t, result1.Applied)
	require.True(t, result1.APIKeyQuotaExhausted)

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, result2)
	require.False(t, result2.Applied)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, 0.000001)

	var quotaUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used FROM api_keys WHERE id = $1", apiKey.ID).Scan(&quotaUsed))
	require.InDelta(t, 1.25, quotaUsed, 0.000001)

	var usage5h float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT usage_5h FROM api_keys WHERE id = $1", apiKey.ID).Scan(&usage5h))
	require.InDelta(t, 1.25, usage5h, 0.000001)

	var status string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT status FROM api_keys WHERE id = $1", apiKey.ID).Scan(&status))
	require.Equal(t, service.StatusAPIKeyQuotaExhausted, status)

	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID).Scan(&dedupCount))
	require.Equal(t, 1, dedupCount)
}

func TestUsageBillingRepositoryApply_DeduplicatesSubscriptionBilling(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-sub-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-group-" + uuid.NewString(),
		Platform:         service.PlatformAnthropic,
		SubscriptionType: service.SubscriptionTypeSubscription,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  user.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-billing-sub-" + uuid.NewString(),
		Name:    "billing-sub",
	})
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:  user.ID,
		GroupID: group.ID,
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:        requestID,
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		AccountID:        0,
		SubscriptionID:   &subscription.ID,
		SubscriptionCost: 2.5,
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result1.Applied)

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result2.Applied)

	var dailyUsage float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id = $1", subscription.ID).Scan(&dailyUsage))
	require.InDelta(t, 2.5, dailyUsage, 0.000001)
}

func TestUsageBillingRepositoryApply_SubscriptionBillingAdvancesExpiredDailyWindowAtCompletedAt(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-sub-window-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-sub-window-" + uuid.NewString(),
		Platform:         service.PlatformOpenAI,
		SubscriptionType: service.SubscriptionTypeSubscription,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  user.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-billing-sub-window-" + uuid.NewString(),
		Name:    "billing-sub-window",
	})

	today := timezone.StartOfDay(timezone.Now())
	yesterday := today.Add(-24 * time.Hour)
	completedAt := today.Add(90 * time.Second)
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:  user.ID,
		GroupID: group.ID,
	})
	_, err := integrationDB.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET daily_usage_usd = $1,
			weekly_usage_usd = $2,
			monthly_usage_usd = $3,
			daily_window_start = $4,
			weekly_window_start = $4,
			monthly_window_start = $4
		WHERE id = $5
	`, 19.5, 20.5, 21.5, yesterday, subscription.ID)
	require.NoError(t, err)

	cmd := &service.UsageBillingCommand{
		RequestID:        uuid.NewString(),
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		SubscriptionID:   &subscription.ID,
		SubscriptionCost: 0.75,
		CompletedAt:      completedAt,
	}
	result, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)

	var dailyUsage float64
	var dailyWindow time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT daily_usage_usd, daily_window_start
		FROM user_subscriptions
		WHERE id = $1
	`, subscription.ID).Scan(&dailyUsage, &dailyWindow))
	require.InDelta(t, 0.75, dailyUsage, 0.000001)
	require.WithinDuration(t, today, dailyWindow, time.Microsecond)
}

func TestUsageBillingRepositoryApply_SubscriptionBillingAccumulatesCurrentDailyWindow(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-sub-current-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-sub-current-" + uuid.NewString(),
		Platform:         service.PlatformOpenAI,
		SubscriptionType: service.SubscriptionTypeSubscription,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  user.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-billing-sub-current-" + uuid.NewString(),
		Name:    "billing-sub-current",
	})

	today := timezone.StartOfDay(timezone.Now())
	completedAt := today.Add(2 * time.Minute)
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:  user.ID,
		GroupID: group.ID,
	})
	_, err := integrationDB.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET daily_usage_usd = $1,
			daily_window_start = $2,
			weekly_window_start = $2,
			monthly_window_start = $2
		WHERE id = $3
	`, 1.25, today, subscription.ID)
	require.NoError(t, err)

	cmd := &service.UsageBillingCommand{
		RequestID:        uuid.NewString(),
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		SubscriptionID:   &subscription.ID,
		SubscriptionCost: 0.75,
		CompletedAt:      completedAt,
	}
	_, err = repo.Apply(ctx, cmd)
	require.NoError(t, err)

	var dailyUsage float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id = $1", subscription.ID).Scan(&dailyUsage))
	require.InDelta(t, 2.0, dailyUsage, 0.000001)
}

func TestUsageBillingRepositoryApply_RequestFingerprintConflict(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-conflict-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-conflict-" + uuid.NewString(),
		Name:   "billing-conflict",
	})

	requestID := uuid.NewString()
	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 1.25,
	})
	require.NoError(t, err)

	_, err = repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 2.50,
	})
	require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
}

func TestUsageBillingRepository_SettlesReservationAndReleasesRemainder(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	fixture := createUsageBillingReservationFixture(t, 1.00, 1.00)

	result, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:                  fixture.requestID,
		APIKeyID:                   fixture.apiKeyID,
		UserID:                     fixture.userID,
		RequestFingerprint:         fixture.fingerprint,
		TrafficPackCost:            0.25,
		TrafficCreditReservationID: &fixture.reservationID,
	})

	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Zero(t, result.TrafficCreditDebtUSD)
	assertTrafficCreditReservationState(t, fixture.creditID, fixture.reservationID, 0.75, 0, 0.25, 0, service.TrafficCreditReservationSettled)
	assertTrafficCreditLedgerTotal(t, fixture.requestID, 0.25, 1)
}

func TestUsageBillingRepository_ReservationDebtDoesNotRollbackUsageFact(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	fixture := createUsageBillingReservationFixture(t, 1.00, 1.00)
	insertUsageBillingFactFixture(t, fixture)

	result, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:                  fixture.requestID,
		APIKeyID:                   fixture.apiKeyID,
		UserID:                     fixture.userID,
		RequestFingerprint:         fixture.fingerprint,
		TrafficPackCost:            1.25,
		TrafficCreditReservationID: &fixture.reservationID,
	})

	require.NoError(t, err)
	require.True(t, result.Applied)
	require.InDelta(t, 0.25, result.TrafficCreditDebtUSD, 1e-10)
	assertTrafficCreditReservationState(t, fixture.creditID, fixture.reservationID, 0, 0, 1.00, 0.25, service.TrafficCreditReservationDebt)
	assertTrafficCreditLedgerTotal(t, fixture.requestID, 1.00, 1)

	var factCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM usage_facts
		WHERE request_id = $1 AND api_key_id = $2 AND reservation_id = $3
	`, fixture.requestID, fixture.apiKeyID, fixture.reservationID).Scan(&factCount))
	require.Equal(t, 1, factCount)
}

func TestUsageBillingRepository_ReplayDoesNotDoubleSettleReservation(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	fixture := createUsageBillingReservationFixture(t, 1.00, 1.00)
	cmd := &service.UsageBillingCommand{
		RequestID:                  fixture.requestID,
		APIKeyID:                   fixture.apiKeyID,
		UserID:                     fixture.userID,
		RequestFingerprint:         fixture.fingerprint,
		TrafficPackCost:            0.40,
		TrafficCreditReservationID: &fixture.reservationID,
	}

	first, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, first.Applied)

	second, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, second.Applied)

	assertTrafficCreditReservationState(t, fixture.creditID, fixture.reservationID, 0.60, 0, 0.40, 0, service.TrafficCreditReservationSettled)
	assertTrafficCreditLedgerTotal(t, fixture.requestID, 0.40, 1)
}

func TestUsageBillingRepositoryApply_UpdatesAccountQuota(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-account-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-account-" + uuid.NewString(),
		Name:   "billing-account",
	})
	account := mustCreateAccount(t, client, &service.Account{
		Name: "usage-billing-account-quota-" + uuid.NewString(),
		Type: service.AccountTypeAPIKey,
		Extra: map[string]any{
			"quota_limit": 100.0,
		},
	})

	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:        uuid.NewString(),
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		AccountID:        account.ID,
		AccountType:      service.AccountTypeAPIKey,
		AccountQuotaCost: 3.5,
	})
	require.NoError(t, err)

	var quotaUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COALESCE((extra->>'quota_used')::numeric, 0) FROM accounts WHERE id = $1", account.ID).Scan(&quotaUsed))
	require.InDelta(t, 3.5, quotaUsed, 0.000001)
}

type usageBillingReservationFixture struct {
	userID        int64
	apiKeyID      int64
	creditID      int64
	reservationID int64
	requestID     string
	fingerprint   string
}

func createUsageBillingReservationFixture(t *testing.T, remainingUSD, reserveUSD float64) usageBillingReservationFixture {
	t.Helper()
	ctx := context.Background()
	client := testEntClient(t)
	userID, creditID := createTrafficCreditReservationFixture(t, remainingUSD)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: userID,
		Key:    "sk-usage-billing-reservation-" + uuid.NewString(),
		Name:   "billing-reservation",
	})
	requestID := "usage-billing-reservation-" + uuid.NewString()
	fingerprint := service.HashUsageRequestPayload([]byte(requestID))
	reservation, _, err := NewTrafficCreditReservationRepository(integrationDB).Reserve(ctx, service.TrafficCreditReservationInput{
		RequestID:          requestID,
		APIKeyID:           apiKey.ID,
		UserID:             userID,
		Platform:           service.TrafficPackPlatformOpenAI,
		Model:              "gpt-5.1",
		RequestFingerprint: fingerprint,
		PricingSnapshot:    json.RawMessage(`{"model":"gpt-5.1","reserve":"test"}`),
		ReserveUSD:         reserveUSD,
		ExpiresAt:          time.Now().UTC().Add(5 * time.Minute),
	})
	require.NoError(t, err)
	require.NoError(t, NewTrafficCreditReservationRepository(integrationDB).MarkDispatched(ctx, reservation.ID))
	return usageBillingReservationFixture{
		userID:        userID,
		apiKeyID:      apiKey.ID,
		creditID:      creditID,
		reservationID: reservation.ID,
		requestID:     requestID,
		fingerprint:   fingerprint,
	}
}

func insertUsageBillingFactFixture(t *testing.T, fixture usageBillingReservationFixture) {
	t.Helper()
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO usage_facts (
			request_id, api_key_id, user_id, account_id, request_fingerprint,
			payload_version, payload, billing_status, completed_at, reservation_id
		)
		VALUES ($1, $2, $3, 0, $4, 1, '{}'::jsonb, 'pending', NOW(), $5)
	`, fixture.requestID, fixture.apiKeyID, fixture.userID, fixture.fingerprint, fixture.reservationID)
	require.NoError(t, err)
}

func assertTrafficCreditReservationState(
	t *testing.T,
	creditID int64,
	reservationID int64,
	wantRemainingUSD float64,
	wantReservedUSD float64,
	wantSettledUSD float64,
	wantDebtUSD float64,
	wantStatus service.TrafficCreditReservationStatus,
) {
	t.Helper()
	ctx := context.Background()
	var remainingUSD, reservedUSD float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT remaining_usd, reserved_usd
		FROM user_traffic_credits
		WHERE id = $1
	`, creditID).Scan(&remainingUSD, &reservedUSD))
	require.InDelta(t, wantRemainingUSD, remainingUSD, 1e-10)
	require.InDelta(t, wantReservedUSD, reservedUSD, 1e-10)

	var settledUSD, debtUSD float64
	var status string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT settled_usd, debt_usd, status
		FROM traffic_credit_reservations
		WHERE id = $1
	`, reservationID).Scan(&settledUSD, &debtUSD, &status))
	require.InDelta(t, wantSettledUSD, settledUSD, 1e-10)
	require.InDelta(t, wantDebtUSD, debtUSD, 1e-10)
	require.Equal(t, string(wantStatus), status)
}

func assertTrafficCreditLedgerTotal(t *testing.T, requestID string, wantAmountUSD float64, wantCount int) {
	t.Helper()
	ctx := context.Background()
	var amountUSD float64
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_usd), 0), COUNT(*)
		FROM traffic_credit_ledger
		WHERE request_id = $1 AND entry_type = $2
	`, requestID, service.TrafficCreditLedgerTypeDeduction).Scan(&amountUSD, &count))
	require.InDelta(t, wantAmountUSD, amountUSD, 1e-10)
	require.Equal(t, wantCount, count)
}

func TestUsageBillingRepositoryApply_EnqueuesSchedulerOutboxOnQuotaCrossing(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	newFixture := func(t *testing.T, extra map[string]any) (int64, int64) {
		t.Helper()
		user := mustCreateUser(t, client, &service.User{
			Email:        fmt.Sprintf("usage-billing-outbox-user-%d-%s@example.com", time.Now().UnixNano(), uuid.NewString()),
			PasswordHash: "hash",
		})
		apiKey := mustCreateApiKey(t, client, &service.APIKey{
			UserID: user.ID,
			Key:    "sk-usage-billing-outbox-" + uuid.NewString(),
			Name:   "billing-outbox",
		})
		account := mustCreateAccount(t, client, &service.Account{
			Name:  "usage-billing-outbox-" + uuid.NewString(),
			Type:  service.AccountTypeAPIKey,
			Extra: extra,
		})
		return apiKey.ID, account.ID
	}

	outboxCountFor := func(t *testing.T, accountID int64) int {
		t.Helper()
		var count int
		require.NoError(t, integrationDB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = $1 AND account_id = $2",
			service.SchedulerOutboxEventAccountChanged, accountID,
		).Scan(&count))
		return count
	}

	t.Run("daily_first_crossing_enqueues", func(t *testing.T) {
		apiKeyID, accountID := newFixture(t, map[string]any{
			"quota_daily_limit": 10.0,
		})
		// 第一次低于日限额：不应入队 outbox
		_, err := repo.Apply(ctx, &service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKeyID,
			AccountID:        accountID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 4,
		})
		require.NoError(t, err)
		require.Equal(t, 0, outboxCountFor(t, accountID), "below limit should not enqueue")

		// 第二次跨越日限额：应入队一次 outbox
		_, err = repo.Apply(ctx, &service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKeyID,
			AccountID:        accountID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 8,
		})
		require.NoError(t, err)
		require.Equal(t, 1, outboxCountFor(t, accountID), "crossing daily limit should enqueue once")

		// 再次递增（已超）：不应重复入队
		_, err = repo.Apply(ctx, &service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKeyID,
			AccountID:        accountID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 2,
		})
		require.NoError(t, err)
		require.Equal(t, 1, outboxCountFor(t, accountID), "subsequent increments beyond limit should not re-enqueue")
	})

	t.Run("weekly_first_crossing_enqueues", func(t *testing.T) {
		apiKeyID, accountID := newFixture(t, map[string]any{
			"quota_weekly_limit": 10.0,
		})
		_, err := repo.Apply(ctx, &service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKeyID,
			AccountID:        accountID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 15, // 单次即跨越
		})
		require.NoError(t, err)
		require.Equal(t, 1, outboxCountFor(t, accountID), "single-shot crossing weekly limit should enqueue once")
	})
}

func TestDashboardAggregationRepositoryCleanupUsageBillingDedup_BatchDeletesOldRows(t *testing.T) {
	ctx := context.Background()
	repo := newDashboardAggregationRepositoryWithSQL(integrationDB)

	oldRequestID := "dedup-old-" + uuid.NewString()
	newRequestID := "dedup-new-" + uuid.NewString()
	oldCreatedAt := time.Now().UTC().AddDate(0, 0, -400)
	newCreatedAt := time.Now().UTC().Add(-time.Hour)

	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint, created_at)
		VALUES ($1, 1, $2, $3), ($4, 1, $5, $6)
	`,
		oldRequestID, strings.Repeat("a", 64), oldCreatedAt,
		newRequestID, strings.Repeat("b", 64), newCreatedAt,
	)
	require.NoError(t, err)

	require.NoError(t, repo.CleanupUsageBillingDedup(ctx, time.Now().UTC().AddDate(0, 0, -365)))

	var oldCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1", oldRequestID).Scan(&oldCount))
	require.Equal(t, 0, oldCount)

	var newCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1", newRequestID).Scan(&newCount))
	require.Equal(t, 1, newCount)

	var archivedCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup_archive WHERE request_id = $1", oldRequestID).Scan(&archivedCount))
	require.Equal(t, 1, archivedCount)
}

func TestUsageBillingRepositoryApply_DeduplicatesAgainstArchivedKey(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	aggRepo := newDashboardAggregationRepositoryWithSQL(integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-archive-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-archive-" + uuid.NewString(),
		Name:   "billing-archive",
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 1.25,
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result1.Applied)

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE usage_billing_dedup
		SET created_at = $1
		WHERE request_id = $2 AND api_key_id = $3
	`, time.Now().UTC().AddDate(0, 0, -400), requestID, apiKey.ID)
	require.NoError(t, err)
	require.NoError(t, aggRepo.CleanupUsageBillingDedup(ctx, time.Now().UTC().AddDate(0, 0, -365)))

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result2.Applied)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, 0.000001)
}
