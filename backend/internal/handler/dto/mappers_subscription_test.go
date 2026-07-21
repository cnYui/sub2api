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
	weeklyLimit := 72.0
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
	require.InDelta(t, 72, *got.EffectiveWeeklyLimitUSD, 1e-9)
	require.NotNil(t, got.WeeklyRemainingUSD)
	require.InDelta(t, 72, *got.WeeklyRemainingUSD, 1e-9)
	require.NotNil(t, got.WeeklyWindowResetsAt)
	require.True(t, got.WeeklyWindowResetsAt.Equal(anchor.AddDate(0, 0, 14)))
}
