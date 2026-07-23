package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestCalculateSubscriptionWeeklyWindow_NewPurchaseHasFourFullWindows(t *testing.T) {
	anchor := time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC)
	expires := anchor.AddDate(0, 0, 28)

	for week := 0; week < 4; week++ {
		now := anchor.AddDate(0, 0, week*7+1)
		window := CalculateSubscriptionWeeklyWindow(anchor, nil, expires, now, 58)
		require.False(t, window.Expired)
		require.Equal(t, anchor.AddDate(0, 0, week*7), window.Start)
		require.Equal(t, anchor.AddDate(0, 0, (week+1)*7), window.End)
		require.Equal(t, 58.0, window.EffectiveLimitUSD)
	}
}

func TestCalculateSubscriptionWeeklyWindow_TailWindowUsesExactProportion(t *testing.T) {
	anchor := time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC)
	expires := anchor.AddDate(0, 0, 13)
	now := anchor.AddDate(0, 0, 8)

	window := CalculateSubscriptionWeeklyWindow(anchor, nil, expires, now, 58)
	require.Equal(t, anchor.AddDate(0, 0, 7), window.Start)
	require.Equal(t, expires, window.End)
	require.InDelta(t, 58.0*6.0/7.0, window.EffectiveLimitUSD, 0.000000001)
	require.False(t, window.Allows(0, window.EffectiveLimitUSD+0.000001))
	require.True(t, window.Allows(0, window.EffectiveLimitUSD))
}

func TestUserSubscriptionWeeklyWindowUsesPersistedAnchor(t *testing.T) {
	anchor := time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC)
	storedStart := anchor.AddDate(0, 0, 7)
	sub := &UserSubscription{
		StartsAt:          anchor.Add(-24 * time.Hour),
		ExpiresAt:         anchor.AddDate(0, 0, 28),
		WeeklyAnchorAt:    &anchor,
		WeeklyWindowStart: &storedStart,
	}

	window := sub.WeeklyWindowAt(anchor.AddDate(0, 0, 9), 198)
	require.Equal(t, storedStart, window.Start)
	require.Equal(t, storedStart.AddDate(0, 0, 7), window.ResetsAt)
}

func TestCalculateSubscriptionWeeklyWindowIgnoresMisalignedPersistedStart(t *testing.T) {
	anchor := time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC)
	expires := anchor.AddDate(0, 0, 28)
	misalignedStart := anchor.AddDate(0, 0, 8)
	now := anchor.AddDate(0, 0, 10)

	window := CalculateSubscriptionWeeklyWindow(anchor, &misalignedStart, expires, now, 58)

	require.False(t, window.Expired)
	require.Equal(t, anchor.AddDate(0, 0, 7), window.Start)
	require.Equal(t, anchor.AddDate(0, 0, 14), window.ResetsAt)
	require.Equal(t, 58.0, window.EffectiveLimitUSD)
}

func TestCheckWeeklyLimit_RollingWeeklyIgnoresStaleWindowUsage(t *testing.T) {
	anchor := time.Now().UTC().AddDate(0, 0, -8).Truncate(time.Second)
	weeklyLimit := 58.0
	group := &Group{
		Name:             "codex-pool-19-usd",
		Status:           StatusActive,
		SubscriptionType: SubscriptionTypeSubscription,
		WeeklyLimitUSD:   &weeklyLimit,
	}
	sub := &UserSubscription{
		StartsAt:          anchor,
		ExpiresAt:         anchor.AddDate(0, 0, 28),
		WeeklyAnchorAt:    &anchor,
		WeeklyWindowStart: &anchor,
		WeeklyUsageUSD:    64,
		CurrentEntitlementPeriod: &SubscriptionEntitlementPeriod{
			ID:             101,
			StartsAt:       anchor,
			ExpiresAt:      anchor.AddDate(0, 0, 28),
			Status:         SubscriptionEntitlementPeriodStatusActive,
			WeeklyLimitUSD: &weeklyLimit,
		},
	}

	require.True(t, sub.CheckWeeklyLimit(group, 56))
	require.False(t, sub.CheckWeeklyLimit(group, 59))
}

func TestCurrentRollingWeeklyWindowUsesEntitlementExpiry(t *testing.T) {
	anchor := time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC)
	weeklyLimit := 58.0
	group := &Group{
		Name:             "codex-pool-19-usd",
		Status:           StatusActive,
		SubscriptionType: SubscriptionTypeSubscription,
		WeeklyLimitUSD:   &weeklyLimit,
	}
	sub := &UserSubscription{
		StartsAt:       anchor,
		ExpiresAt:      anchor.AddDate(0, 0, 56),
		WeeklyAnchorAt: &anchor,
		CurrentEntitlementPeriod: &SubscriptionEntitlementPeriod{
			ID:             101,
			StartsAt:       anchor,
			ExpiresAt:      anchor.AddDate(0, 0, 26),
			Status:         SubscriptionEntitlementPeriodStatusActive,
			WeeklyLimitUSD: &weeklyLimit,
		},
	}

	window, ok := sub.CurrentRollingWeeklyWindow(group, anchor.AddDate(0, 0, 22))
	require.True(t, ok)
	require.Equal(t, anchor.AddDate(0, 0, 21), window.Start)
	require.Equal(t, anchor.AddDate(0, 0, 26), window.ResetsAt)
	require.InDelta(t, weeklyLimit*5.0/7.0, window.EffectiveLimitUSD, 0.000000001)
}

func TestCurrentRollingWeeklyWindowPreservesHistoricalMigrationAnchorUsage(t *testing.T) {
	migrationAnchor := time.Date(2026, 7, 22, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	originalPurchaseAt := time.Date(2026, 7, 21, 9, 26, 12, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	weeklyLimit := 76.0
	group := &Group{
		Name:             "codex-pool-19-usd",
		Status:           StatusActive,
		SubscriptionType: SubscriptionTypeSubscription,
		WeeklyLimitUSD:   &weeklyLimit,
	}
	sub := &UserSubscription{
		StartsAt:          originalPurchaseAt,
		ExpiresAt:         originalPurchaseAt.AddDate(0, 0, 30),
		WeeklyAnchorAt:    &migrationAnchor,
		WeeklyWindowStart: &migrationAnchor,
		WeeklyUsageUSD:    11.75,
		CurrentEntitlementPeriod: &SubscriptionEntitlementPeriod{
			ID:                  101,
			StartsAt:            originalPurchaseAt,
			ExpiresAt:           originalPurchaseAt.AddDate(0, 0, 30),
			QuotaWindowAnchorAt: &migrationAnchor,
			Status:              SubscriptionEntitlementPeriodStatusActive,
			WeeklyLimitUSD:      &weeklyLimit,
		},
	}

	window, ok := sub.CurrentRollingWeeklyWindow(group, migrationAnchor.Add(24*time.Hour))

	require.True(t, ok)
	require.Equal(t, migrationAnchor, window.Start)
	require.Equal(t, migrationAnchor.AddDate(0, 0, 7), window.ResetsAt)
	require.Equal(t, sub.WeeklyUsageUSD, sub.RollingWeeklyUsageUSD(window))
}

func TestCurrentRollingWeeklyWindowStartsAtNewEntitlementBoundary(t *testing.T) {
	legacyAnchor := time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC)
	legacyExpiresAt := legacyAnchor.AddDate(0, 0, 30)
	weeklyLimit := 76.0
	legacyWindowStart := legacyAnchor.AddDate(0, 0, 28)
	group := &Group{
		Name:             "codex-pool-19-usd",
		Status:           StatusActive,
		SubscriptionType: SubscriptionTypeSubscription,
		WeeklyLimitUSD:   &weeklyLimit,
	}
	sub := &UserSubscription{
		StartsAt:          legacyAnchor,
		ExpiresAt:         legacyExpiresAt.AddDate(0, 0, 28),
		WeeklyAnchorAt:    &legacyAnchor,
		WeeklyWindowStart: &legacyWindowStart,
		WeeklyUsageUSD:    11,
		CurrentEntitlementPeriod: &SubscriptionEntitlementPeriod{
			ID:             102,
			StartsAt:       legacyExpiresAt,
			ExpiresAt:      legacyExpiresAt.AddDate(0, 0, 28),
			Status:         SubscriptionEntitlementPeriodStatusActive,
			WeeklyLimitUSD: &weeklyLimit,
		},
	}

	window, ok := sub.CurrentRollingWeeklyWindow(group, legacyExpiresAt.Add(time.Hour))

	require.True(t, ok)
	require.Equal(t, legacyExpiresAt, window.Start)
	require.Equal(t, legacyExpiresAt.AddDate(0, 0, 7), window.End)
	require.Equal(t, weeklyLimit, window.EffectiveLimitUSD)
	require.Zero(t, sub.RollingWeeklyUsageUSD(window))
}

func TestValidateAndCheckLimits_RollingWeeklyLimitExceededIncludesResetMetadata(t *testing.T) {
	anchor := time.Now().UTC().Add(-time.Hour)
	weeklyLimit := 58.0
	group := &Group{
		Name:             "codex-pool-19-usd",
		Status:           StatusActive,
		SubscriptionType: SubscriptionTypeSubscription,
		WeeklyLimitUSD:   &weeklyLimit,
	}
	sub := &UserSubscription{
		Status:            SubscriptionStatusActive,
		StartsAt:          anchor,
		ExpiresAt:         anchor.AddDate(0, 0, 28),
		WeeklyAnchorAt:    &anchor,
		WeeklyWindowStart: &anchor,
		WeeklyUsageUSD:    weeklyLimit + 0.01,
		CurrentEntitlementPeriod: &SubscriptionEntitlementPeriod{
			ID:             101,
			StartsAt:       anchor,
			ExpiresAt:      anchor.AddDate(0, 0, 28),
			Status:         SubscriptionEntitlementPeriodStatusActive,
			WeeklyLimitUSD: &weeklyLimit,
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)

	needsMaintenance, err := svc.ValidateAndCheckLimits(sub, group)

	require.False(t, needsMaintenance)
	require.ErrorIs(t, err, ErrWeeklyLimitExceeded)
	appErr := infraerrors.FromError(err)
	require.Equal(t, anchor.AddDate(0, 0, 7).UTC().Format(time.RFC3339), appErr.Metadata["window_resets_at"])
}

func TestCheckAndActivateWindow_RollingWeeklyUsesAnchoredWindowStart(t *testing.T) {
	anchor := time.Now().UTC().AddDate(0, 0, -8).Truncate(time.Second)
	weeklyLimit := 58.0
	group := &Group{
		Name:             "codex-pool-19-usd",
		Status:           StatusActive,
		SubscriptionType: SubscriptionTypeSubscription,
		WeeklyLimitUSD:   &weeklyLimit,
	}
	repo := &activateRollingWeeklyWindowRepo{}
	sub := &UserSubscription{
		ID:             801,
		UserID:         901,
		GroupID:        2,
		Status:         SubscriptionStatusActive,
		StartsAt:       anchor,
		ExpiresAt:      anchor.AddDate(0, 0, 28),
		WeeklyAnchorAt: &anchor,
		Group:          group,
		CurrentEntitlementPeriod: &SubscriptionEntitlementPeriod{
			ID:             701,
			StartsAt:       anchor,
			ExpiresAt:      anchor.AddDate(0, 0, 28),
			Status:         SubscriptionEntitlementPeriodStatusActive,
			WeeklyLimitUSD: &weeklyLimit,
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)

	err := svc.CheckAndActivateWindow(context.Background(), sub)

	require.NoError(t, err)
	require.True(t, repo.called)
	require.Equal(t, anchor.AddDate(0, 0, 7), repo.weeklyStart)
	require.NotNil(t, sub.WeeklyWindowStart)
	require.Equal(t, anchor.AddDate(0, 0, 7), *sub.WeeklyWindowStart)
}

type activateRollingWeeklyWindowRepo struct {
	userSubRepoNoop
	called      bool
	weeklyStart time.Time
}

func (r *activateRollingWeeklyWindowRepo) RefreshExpiredUsageWindows(_ context.Context, _ int64, _, weeklyStart, _ time.Time, _ time.Time) (bool, error) {
	r.called = true
	r.weeklyStart = weeklyStart
	return true, nil
}

func TestPublicCodexSnapshotUsesFixedWeeklyQuotaWithoutMigratedGroupField(t *testing.T) {
	dailyLimit := 15.0
	plan := &dbent.SubscriptionPlan{ID: 7, GroupID: 2, Name: "29 元订阅池", ValidityDays: 30, ValidityUnit: "day"}
	group := &Group{
		ID:               2,
		Name:             "codex-pool-19-usd",
		Status:           StatusActive,
		SubscriptionType: SubscriptionTypeSubscription,
		DailyLimitUSD:    &dailyLimit,
	}

	snapshot, err := newSubscriptionOrderSnapshot(plan, group)
	require.NoError(t, err)
	require.Equal(t, publicCodexSubscriptionValidityDays, snapshot.ValidityDays)
	require.Equal(t, "week", snapshot.QuotaWindowUnit)
	require.Equal(t, subscriptionWeeklyWindowDays, snapshot.QuotaWindowDays)
	require.NotNil(t, snapshot.WeeklyLimitUSD)
	require.Equal(t, 76.0, *snapshot.WeeklyLimitUSD)
	require.NotNil(t, snapshot.PeriodTotalQuotaUSD)
	require.Equal(t, 304.0, *snapshot.PeriodTotalQuotaUSD)
}

func TestPublicCodexPlanQuotaSnapshotUsesSameWindowContract(t *testing.T) {
	dailyLimit := 15.0

	snapshot := BuildPlanQuotaSnapshot("codex-pool-29-usd", &dailyLimit, nil, nil, 30, "day")

	require.Nil(t, snapshot.DailyLimitUSD)
	require.Nil(t, snapshot.MonthlyLimitUSD)
	require.NotNil(t, snapshot.WeeklyLimitUSD)
	require.Equal(t, 102.0, *snapshot.WeeklyLimitUSD)
	require.NotNil(t, snapshot.PeriodTotalQuotaUSD)
	require.Equal(t, 408.0, *snapshot.PeriodTotalQuotaUSD)
	require.Equal(t, "week", snapshot.QuotaWindowUnit)
	require.Equal(t, subscriptionWeeklyWindowDays, snapshot.QuotaWindowDays)
	require.Equal(t, publicCodexSubscriptionValidityDays, snapshot.EffectiveValidityDays)
}
