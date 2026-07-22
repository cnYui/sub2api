package dto

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserSubscriptionFromService_RollingWeeklyDTOResetsStaleWindowUsage(t *testing.T) {
	t.Parallel()

	anchor := time.Now().AddDate(0, 0, -8).Truncate(time.Second)
	expiresAt := anchor.AddDate(0, 0, 28)
	weeklyLimit := 58.0
	sub := &service.UserSubscription{
		ID:                1001,
		UserID:            2001,
		GroupID:           2,
		StartsAt:          anchor,
		ExpiresAt:         expiresAt,
		Status:            service.SubscriptionStatusActive,
		WeeklyAnchorAt:    &anchor,
		WeeklyWindowStart: &anchor,
		WeeklyUsageUSD:    60,
		Group: &service.Group{
			ID:               2,
			Name:             "codex-pool-19-usd",
			SubscriptionType: service.SubscriptionTypeSubscription,
			WeeklyLimitUSD:   &weeklyLimit,
		},
		CurrentEntitlementPeriod: &service.SubscriptionEntitlementPeriod{
			ID:              3001,
			SubscriptionID:  1001,
			GroupID:         2,
			StartsAt:        anchor,
			ExpiresAt:       expiresAt,
			Status:          service.SubscriptionEntitlementPeriodStatusActive,
			WeeklyLimitUSD:  &weeklyLimit,
			QuotaWindowUnit: "week",
			QuotaWindowDays: 7,
		},
	}

	got := UserSubscriptionFromService(sub)

	require.NotNil(t, got)
	require.NotNil(t, got.WeeklyWindowStart)
	require.True(t, got.WeeklyWindowStart.Equal(anchor.AddDate(0, 0, 7)))
	require.Zero(t, got.WeeklyUsageUSD)
	require.NotNil(t, got.EffectiveWeeklyLimitUSD)
	require.InDelta(t, 58, *got.EffectiveWeeklyLimitUSD, 1e-9)
	require.NotNil(t, got.WeeklyRemainingUSD)
	require.InDelta(t, 58, *got.WeeklyRemainingUSD, 1e-9)
	require.NotNil(t, got.WeeklyWindowResetsAt)
	require.True(t, got.WeeklyWindowResetsAt.Equal(anchor.AddDate(0, 0, 14)))
}

func TestUserSubscriptionFromServiceAdmin_NormalizesPublicCodexGroupQuota(t *testing.T) {
	t.Parallel()

	anchor := time.Now().Add(-time.Hour).Truncate(time.Second)
	expiresAt := anchor.AddDate(0, 0, 28)
	legacyDailyLimit := 15.0
	weeklyLimit := 58.0
	sub := &service.UserSubscription{
		ID:                1002,
		UserID:            2002,
		GroupID:           2,
		StartsAt:          anchor,
		ExpiresAt:         expiresAt,
		Status:            service.SubscriptionStatusActive,
		WeeklyAnchorAt:    &anchor,
		WeeklyWindowStart: &anchor,
		WeeklyUsageUSD:    7.9,
		Group: &service.Group{
			ID:               2,
			Name:             "codex-pool-19-usd",
			SubscriptionType: service.SubscriptionTypeSubscription,
			DailyLimitUSD:    &legacyDailyLimit,
		},
		CurrentEntitlementPeriod: &service.SubscriptionEntitlementPeriod{
			ID:              3002,
			SubscriptionID:  1002,
			GroupID:         2,
			StartsAt:        anchor,
			ExpiresAt:       expiresAt,
			Status:          service.SubscriptionEntitlementPeriodStatusActive,
			WeeklyLimitUSD:  &weeklyLimit,
			QuotaWindowUnit: "week",
			QuotaWindowDays: 7,
		},
	}

	got := UserSubscriptionFromServiceAdmin(sub)

	require.NotNil(t, got)
	require.NotNil(t, got.Group)
	require.Nil(t, got.Group.DailyLimitUSD)
	require.NotNil(t, got.Group.WeeklyLimitUSD)
	require.InDelta(t, 58, *got.Group.WeeklyLimitUSD, 1e-9)
	require.NotNil(t, got.EffectiveWeeklyLimitUSD)
	require.InDelta(t, 58, *got.EffectiveWeeklyLimitUSD, 1e-9)
	require.NotNil(t, got.WeeklyWindowResetsAt)
}
