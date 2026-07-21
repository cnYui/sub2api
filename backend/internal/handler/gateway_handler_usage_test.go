package handler

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCalculateSubscriptionRemainingRollingWeeklyRequiresWindowFacts(t *testing.T) {
	weeklyLimit := 72.0
	group := &service.Group{
		ID:               2,
		Name:             "codex-pool-19-usd",
		SubscriptionType: service.SubscriptionTypeSubscription,
		WeeklyLimitUSD:   &weeklyLimit,
	}
	now := time.Now()
	sub := &service.UserSubscription{
		ID:             21,
		GroupID:        group.ID,
		Group:          group,
		Status:         service.SubscriptionStatusActive,
		StartsAt:       now.Add(-time.Hour),
		ExpiresAt:      now.Add(24 * time.Hour),
		WeeklyUsageUSD: 10,
	}

	got := (&GatewayHandler{}).calculateSubscriptionRemaining(group, sub)

	require.Equal(t, 0.0, got)
}

func TestCalculateSubscriptionRemainingRollingWeeklyUsesEffectiveWindow(t *testing.T) {
	weeklyLimit := 72.0
	group := &service.Group{
		ID:               2,
		Name:             "codex-pool-19-usd",
		SubscriptionType: service.SubscriptionTypeSubscription,
		WeeklyLimitUSD:   &weeklyLimit,
	}
	now := time.Now()
	anchor := now.Add(-time.Hour)
	sub := &service.UserSubscription{
		ID:                21,
		GroupID:           group.ID,
		Group:             group,
		Status:            service.SubscriptionStatusActive,
		StartsAt:          anchor,
		ExpiresAt:         now.AddDate(0, 0, 27),
		WeeklyAnchorAt:    &anchor,
		WeeklyWindowStart: &anchor,
		WeeklyUsageUSD:    10,
		CurrentEntitlementPeriod: &service.SubscriptionEntitlementPeriod{
			ID:             1001,
			GroupID:        group.ID,
			StartsAt:       anchor,
			ExpiresAt:      now.AddDate(0, 0, 27),
			WeeklyLimitUSD: &weeklyLimit,
			Status:         service.SubscriptionEntitlementPeriodStatusActive,
		},
	}

	got := (&GatewayHandler{}).calculateSubscriptionRemaining(group, sub)

	require.InDelta(t, 62, got, 0.0000001)
}
