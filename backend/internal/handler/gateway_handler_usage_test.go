//go:build unit

package handler

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNormalizedSubscriptionForUsageResponseZerosExpiredDailyWindow(t *testing.T) {
	now := timezone.StartOfDay(timezone.Now()).Add(2 * time.Hour)
	yesterday := timezone.StartOfDay(now).Add(-24 * time.Hour)
	weeklyStart := timezone.StartOfWeek(now)
	monthlyStart := now.Add(-24 * time.Hour)
	sub := &service.UserSubscription{
		DailyUsageUSD:      12,
		WeeklyUsageUSD:     5,
		MonthlyUsageUSD:    9,
		DailyWindowStart:   &yesterday,
		WeeklyWindowStart:  &weeklyStart,
		MonthlyWindowStart: &monthlyStart,
	}

	got := normalizedSubscriptionForUsageResponse(sub, now)

	require.NotSame(t, sub, got)
	require.Equal(t, 0.0, got.DailyUsageUSD)
	require.Equal(t, 5.0, got.WeeklyUsageUSD)
	require.Equal(t, 9.0, got.MonthlyUsageUSD)
	require.Equal(t, 12.0, sub.DailyUsageUSD)
}

func TestNormalizedSubscriptionForUsageResponseKeepsCurrentDailyWindow(t *testing.T) {
	now := timezone.StartOfDay(timezone.Now()).Add(2 * time.Hour)
	today := timezone.StartOfDay(now)
	sub := &service.UserSubscription{
		DailyUsageUSD:    3.5,
		DailyWindowStart: &today,
	}

	got := normalizedSubscriptionForUsageResponse(sub, now)

	require.Equal(t, 3.5, got.DailyUsageUSD)
}
