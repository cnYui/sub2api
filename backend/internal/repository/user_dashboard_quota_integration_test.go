//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	entgroup "github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepository_GetUserDashboardQuota_UsesCurrentPrecisePeriodAndDeduplicatesFacts(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)

	user := mustCreateUser(t, client, &service.User{Email: "quota-precise@example.com"})
	dailyLimit := 19.0
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "quota-precise-group",
		SubscriptionType: service.SubscriptionTypeSubscription,
		DailyLimitUSD:    &dailyLimit,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-quota-precise"})
	account := mustCreateAccount(t, client, &service.Account{Name: "quota-precise-account"})
	now := time.Now().UTC()
	startsAt := now.Add(-48 * time.Hour)
	expiresAt := now.Add(28 * 24 * time.Hour)
	sub := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:    user.ID,
		GroupID:   group.ID,
		StartsAt:  startsAt,
		ExpiresAt: expiresAt,
	})
	periodLimit := 23.0
	periodRepo := NewSubscriptionEntitlementPeriodRepository(client)
	require.NoError(t, periodRepo.Create(ctx, &service.SubscriptionEntitlementPeriod{
		UserID:         user.ID,
		SubscriptionID: sub.ID,
		GroupID:        group.ID,
		Source: service.SubscriptionEntitlementSource{
			Type: "payment_order",
			ID:   "quota-precise-" + uuid.NewString(),
		},
		StartsAt:      startsAt,
		ExpiresAt:     expiresAt,
		PeriodDays:    30,
		DailyLimitUSD: &periodLimit,
		Status:        service.SubscriptionEntitlementPeriodStatusActive,
	}))

	todayStart := timezone.Today()
	todayAt1 := now.Add(-4 * time.Second)
	todayAt2 := now.Add(-3 * time.Second)
	todayAt3 := now.Add(-2 * time.Second)
	todayAt4 := now.Add(-1 * time.Second)
	require.False(t, todayAt1.Before(todayStart), "test setup assumes now is inside the service day")
	dedupRequestID := "quota-dedup-" + uuid.NewString()
	insertDashboardQuotaUsageFact(t, ctx, tx, dashboardQuotaUsageFactInput{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
		RequestID: dedupRequestID, ActualCost: 0.70, CompletedAt: todayAt1,
		Status: service.UsageFactStatusPending,
	})
	insertDashboardQuotaUsageLog(t, ctx, repo, dashboardQuotaUsageLogInput{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID, GroupID: group.ID, SubscriptionID: sub.ID,
		RequestID: dedupRequestID, ActualCost: 9.99, CreatedAt: todayAt1,
	})
	insertDashboardQuotaUsageLog(t, ctx, repo, dashboardQuotaUsageLogInput{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID, GroupID: group.ID, SubscriptionID: sub.ID,
		RequestID: "quota-log-" + uuid.NewString(), ActualCost: 0.30, CreatedAt: todayAt2,
	})
	insertDashboardQuotaUsageFact(t, ctx, tx, dashboardQuotaUsageFactInput{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
		RequestID: "quota-debt-" + uuid.NewString(), ActualCost: 0.20, CompletedAt: todayAt3,
		Status: service.UsageFactStatusDebt,
	})
	zeroEffectsCost := 0.0
	insertDashboardQuotaUsageFact(t, ctx, tx, dashboardQuotaUsageFactInput{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
		RequestID: "quota-log-payload-" + uuid.NewString(), ActualCost: 0.40, CompletedAt: todayAt4,
		Status: service.UsageFactStatusSettling, EffectsActualCost: &zeroEffectsCost,
	})
	insertDashboardQuotaUsageFact(t, ctx, tx, dashboardQuotaUsageFactInput{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
		RequestID: "quota-old-" + uuid.NewString(), ActualCost: 1.10, CompletedAt: todayStart.AddDate(0, 0, -1),
		Status: service.UsageFactStatusSettled,
	})

	quota, err := repo.GetUserDashboardQuota(ctx, user.ID)

	require.NoError(t, err)
	require.Equal(t, usagestats.UserDashboardQuotaModeEntitlementPeriod, quota.PeriodMode)
	require.InDelta(t, 1.60, quota.TodayUsageUSD, 0.0000001)
	require.InDelta(t, periodLimit, quota.TodayLimitUSD, 0.0000001)
	require.False(t, quota.TodayLimitUnlimited)
	require.InDelta(t, 2.70, quota.PeriodUsageUSD, 0.0000001)
	require.InDelta(t, periodLimit*30, quota.PeriodLimitUSD, 0.0000001)
	require.False(t, quota.PeriodLimitUnlimited)
	require.Equal(t, 30, quota.PeriodDays)
	require.NotNil(t, quota.PeriodStartsAt)
	require.NotNil(t, quota.PeriodExpiresAt)
	require.WithinDuration(t, startsAt, *quota.PeriodStartsAt, time.Second)
	require.WithinDuration(t, expiresAt, *quota.PeriodExpiresAt, time.Second)
}

func TestUsageLogRepository_RollingWeeklyQuotaStartsAtCurrentEntitlementBoundary(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)

	user := mustCreateUser(t, client, &service.User{Email: "quota-weekly-boundary@example.com"})
	weeklyLimit := 76.0
	group, err := client.Group.Query().Where(entgroup.NameEQ("codex-pool-19-usd")).Only(ctx)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	legacyStartsAt := now.AddDate(0, 0, -31)
	legacyExpiresAt := legacyStartsAt.AddDate(0, 0, 30)
	legacyWindowStart := legacyStartsAt.AddDate(0, 0, 28)
	currentExpiresAt := legacyExpiresAt.AddDate(0, 0, 28)
	sub := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:         user.ID,
		GroupID:        group.ID,
		StartsAt:       legacyStartsAt,
		ExpiresAt:      currentExpiresAt,
		WeeklyUsageUSD: 11,
	})
	_, err = client.UserSubscription.UpdateOneID(sub.ID).
		SetWeeklyAnchorAt(legacyStartsAt).
		SetWeeklyWindowStart(legacyWindowStart).
		Save(ctx)
	require.NoError(t, err)

	periodRepo := NewSubscriptionEntitlementPeriodRepository(client)
	require.NoError(t, periodRepo.Create(ctx, &service.SubscriptionEntitlementPeriod{
		UserID:         user.ID,
		SubscriptionID: sub.ID,
		GroupID:        group.ID,
		Source: service.SubscriptionEntitlementSource{
			Type: "payment_order",
			ID:   "quota-weekly-boundary-" + uuid.NewString(),
		},
		StartsAt:        legacyExpiresAt,
		ExpiresAt:       currentExpiresAt,
		PeriodDays:      28,
		WeeklyLimitUSD:  &weeklyLimit,
		QuotaWindowUnit: "week",
		QuotaWindowDays: 7,
		Status:          service.SubscriptionEntitlementPeriodStatusActive,
	}))

	quota, found, err := repo.getCurrentRollingWeeklyDashboardQuota(ctx, user.ID, now)

	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, legacyExpiresAt, quota.startsAt)
	require.Equal(t, legacyExpiresAt.AddDate(0, 0, 7), quota.resetsAt)
	require.InDelta(t, weeklyLimit, quota.limitUSD, 0.0000001)
	require.Zero(t, quota.usageUSD)
}

func TestUsageLogRepository_GetUserDashboardQuota_FallsBackToRolling30LegacyForActiveSubscriptionWithoutPeriod(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)

	user := mustCreateUser(t, client, &service.User{Email: "quota-legacy@example.com"})
	dailyLimit := 11.0
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "quota-legacy-group",
		SubscriptionType: service.SubscriptionTypeSubscription,
		DailyLimitUSD:    &dailyLimit,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-quota-legacy"})
	account := mustCreateAccount(t, client, &service.Account{Name: "quota-legacy-account"})
	now := time.Now().UTC()
	sub := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:    user.ID,
		GroupID:   group.ID,
		StartsAt:  now.Add(-60 * 24 * time.Hour),
		ExpiresAt: now.Add(20 * 24 * time.Hour),
	})

	todayStart := timezone.Today()
	todayAt := now.Add(-time.Second)
	require.False(t, todayAt.Before(todayStart), "test setup assumes now is inside the service day")
	insertDashboardQuotaUsageLog(t, ctx, repo, dashboardQuotaUsageLogInput{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID, GroupID: group.ID, SubscriptionID: sub.ID,
		RequestID: "quota-legacy-today-" + uuid.NewString(), ActualCost: 0.45, CreatedAt: todayAt,
	})
	insertDashboardQuotaUsageLog(t, ctx, repo, dashboardQuotaUsageLogInput{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID, GroupID: group.ID, SubscriptionID: sub.ID,
		RequestID: "quota-legacy-window-" + uuid.NewString(), ActualCost: 0.55, CreatedAt: now.Add(-10 * 24 * time.Hour),
	})
	insertDashboardQuotaUsageLog(t, ctx, repo, dashboardQuotaUsageLogInput{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID, GroupID: group.ID, SubscriptionID: sub.ID,
		RequestID: "quota-legacy-outside-" + uuid.NewString(), ActualCost: 99, CreatedAt: now.Add(-31 * 24 * time.Hour),
	})

	quota, err := repo.GetUserDashboardQuota(ctx, user.ID)

	require.NoError(t, err)
	require.Equal(t, usagestats.UserDashboardQuotaModeRolling30Legacy, quota.PeriodMode)
	require.InDelta(t, 0.45, quota.TodayUsageUSD, 0.0000001)
	require.InDelta(t, dailyLimit, quota.TodayLimitUSD, 0.0000001)
	require.False(t, quota.TodayLimitUnlimited)
	require.InDelta(t, 1.00, quota.PeriodUsageUSD, 0.0000001)
	require.InDelta(t, dailyLimit*30, quota.PeriodLimitUSD, 0.0000001)
	require.False(t, quota.PeriodLimitUnlimited)
	require.Equal(t, 30, quota.PeriodDays)
}

func TestUsageLogRepository_GetUserDashboardQuota_IncludesSubscriptionDailyCarryover(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)

	user := mustCreateUser(t, client, &service.User{Email: "quota-carryover@example.com"})
	dailyLimit := 15.0
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "quota-carryover-group",
		SubscriptionType: service.SubscriptionTypeSubscription,
		DailyLimitUSD:    &dailyLimit,
	})
	now := time.Now().UTC()
	sub := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:    user.ID,
		GroupID:   group.ID,
		StartsAt:  now.Add(-48 * time.Hour),
		ExpiresAt: now.Add(28 * 24 * time.Hour),
	})
	todayStart := timezone.Today()
	yesterday := todayStart.Add(-24 * time.Hour)
	_, err := tx.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET daily_usage_usd = $1,
			daily_window_start = $2
		WHERE id = $3
	`, 25.0, yesterday, sub.ID)
	require.NoError(t, err)

	quota, err := repo.GetUserDashboardQuota(ctx, user.ID)

	require.NoError(t, err)
	require.Equal(t, usagestats.UserDashboardQuotaModeRolling30Legacy, quota.PeriodMode)
	require.InDelta(t, 10.0, quota.TodayUsageUSD, 0.0000001)
	require.InDelta(t, dailyLimit, quota.TodayLimitUSD, 0.0000001)
	require.False(t, quota.TodayLimitUnlimited)
}

func TestUsageLogRepository_GetUserDashboardQuota_ActiveUnlimitedSubscriptionDisplaysUsage(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)

	user := mustCreateUser(t, client, &service.User{Email: "quota-unlimited@example.com"})
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "quota-unlimited-group",
		SubscriptionType: service.SubscriptionTypeSubscription,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-quota-unlimited"})
	account := mustCreateAccount(t, client, &service.Account{Name: "quota-unlimited-account"})
	now := time.Now().UTC()
	sub := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:    user.ID,
		GroupID:   group.ID,
		StartsAt:  now.Add(-24 * time.Hour),
		ExpiresAt: now.Add(29 * 24 * time.Hour),
	})
	todayAt := now.Add(-time.Second)
	insertDashboardQuotaUsageLog(t, ctx, repo, dashboardQuotaUsageLogInput{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID, GroupID: group.ID, SubscriptionID: sub.ID,
		RequestID: "quota-unlimited-" + uuid.NewString(), ActualCost: 1.23, CreatedAt: todayAt,
	})

	quota, err := repo.GetUserDashboardQuota(ctx, user.ID)

	require.NoError(t, err)
	require.Equal(t, usagestats.UserDashboardQuotaModeRolling30Legacy, quota.PeriodMode)
	require.InDelta(t, 1.23, quota.TodayUsageUSD, 0.0000001)
	require.Zero(t, quota.TodayLimitUSD)
	require.True(t, quota.TodayLimitUnlimited)
	require.InDelta(t, 1.23, quota.PeriodUsageUSD, 0.0000001)
	require.Zero(t, quota.PeriodLimitUSD)
	require.True(t, quota.PeriodLimitUnlimited)
	require.Equal(t, 30, quota.PeriodDays)
}

func TestUsageLogRepository_GetUserDashboardQuota_NoSubscriptionUsesZeroLimitsAndRolling30Usage(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)

	user := mustCreateUser(t, client, &service.User{Email: "quota-none@example.com"})
	group := mustCreateGroup(t, client, &service.Group{Name: "quota-none-group"})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-quota-none"})
	account := mustCreateAccount(t, client, &service.Account{Name: "quota-none-account"})
	now := time.Now().UTC()
	todayStart := timezone.Today()
	todayAt := now.Add(-time.Second)
	require.False(t, todayAt.Before(todayStart), "test setup assumes now is inside the service day")
	insertDashboardQuotaUsageLog(t, ctx, repo, dashboardQuotaUsageLogInput{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID, GroupID: group.ID,
		RequestID: "quota-none-today-" + uuid.NewString(), ActualCost: 0.80, CreatedAt: todayAt,
	})
	insertDashboardQuotaUsageLog(t, ctx, repo, dashboardQuotaUsageLogInput{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID, GroupID: group.ID,
		RequestID: "quota-none-window-" + uuid.NewString(), ActualCost: 0.40, CreatedAt: now.Add(-5 * 24 * time.Hour),
	})

	quota, err := repo.GetUserDashboardQuota(ctx, user.ID)

	require.NoError(t, err)
	require.Equal(t, usagestats.UserDashboardQuotaModeNoSubscription, quota.PeriodMode)
	require.InDelta(t, 0.80, quota.TodayUsageUSD, 0.0000001)
	require.Zero(t, quota.TodayLimitUSD)
	require.False(t, quota.TodayLimitUnlimited)
	require.InDelta(t, 1.20, quota.PeriodUsageUSD, 0.0000001)
	require.Zero(t, quota.PeriodLimitUSD)
	require.False(t, quota.PeriodLimitUnlimited)
	require.Equal(t, 30, quota.PeriodDays)
}

type dashboardQuotaUsageLogInput struct {
	UserID         int64
	APIKeyID       int64
	AccountID      int64
	GroupID        int64
	SubscriptionID int64
	RequestID      string
	ActualCost     float64
	CreatedAt      time.Time
}

func insertDashboardQuotaUsageLog(t *testing.T, ctx context.Context, repo *usageLogRepository, in dashboardQuotaUsageLogInput) {
	t.Helper()
	log := &service.UsageLog{
		UserID:     in.UserID,
		APIKeyID:   in.APIKeyID,
		AccountID:  in.AccountID,
		RequestID:  in.RequestID,
		Model:      "gpt-test",
		TotalCost:  in.ActualCost,
		ActualCost: in.ActualCost,
		CreatedAt:  in.CreatedAt,
	}
	if in.GroupID > 0 {
		log.GroupID = &in.GroupID
	}
	if in.SubscriptionID > 0 {
		log.SubscriptionID = &in.SubscriptionID
	}
	inserted, err := repo.Create(ctx, log)
	require.NoError(t, err)
	require.True(t, inserted)
}

type dashboardQuotaUsageFactInput struct {
	UserID            int64
	APIKeyID          int64
	AccountID         int64
	RequestID         string
	ActualCost        float64
	EffectsActualCost *float64
	CompletedAt       time.Time
	Status            string
}

func insertDashboardQuotaUsageFact(t *testing.T, ctx context.Context, tx *dbent.Tx, in dashboardQuotaUsageFactInput) {
	t.Helper()
	effectsActualCost := in.ActualCost
	if in.EffectsActualCost != nil {
		effectsActualCost = *in.EffectsActualCost
	}
	payload, err := json.Marshal(service.UsageFactPayload{
		UsageLog: service.UsageLog{
			UserID:     in.UserID,
			APIKeyID:   in.APIKeyID,
			AccountID:  in.AccountID,
			RequestID:  in.RequestID,
			ActualCost: in.ActualCost,
		},
		Effects: service.UsageSettlementEffectsPayload{
			UserID:     in.UserID,
			APIKeyID:   in.APIKeyID,
			AccountID:  in.AccountID,
			ActualCost: effectsActualCost,
		},
	})
	require.NoError(t, err)
	status := in.Status
	if status == "" {
		status = service.UsageFactStatusPending
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO usage_facts (
			request_id, api_key_id, user_id, account_id, request_fingerprint,
			payload_version, payload, billing_status, next_attempt_at, completed_at
		)
		VALUES ($1, $2, $3, $4, $5, 1, $6, $7, $8, $8)
	`, in.RequestID, in.APIKeyID, in.UserID, in.AccountID, strings.ReplaceAll(uuid.NewString(), "-", ""), string(payload), status, in.CompletedAt)
	require.NoError(t, err)
}
