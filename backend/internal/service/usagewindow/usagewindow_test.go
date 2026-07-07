package usagewindow

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func TestDailyExpiredUsesGlobalNaturalDay(t *testing.T) {
	require.NoError(t, timezone.Init("Asia/Shanghai"))
	loc := timezone.Location()
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, loc)
	yesterday := time.Date(2026, 7, 6, 23, 0, 0, 0, loc)
	today := time.Date(2026, 7, 7, 0, 0, 0, 0, loc)

	require.True(t, DailyExpired(&yesterday, now, DailyPolicyNaturalDay))
	require.False(t, DailyExpired(&today, now, DailyPolicyNaturalDay))
}

func TestDailyExpiredKeepsOneTimeDailyQuota(t *testing.T) {
	require.NoError(t, timezone.Init("Asia/Shanghai"))
	loc := timezone.Location()
	start := time.Date(2026, 7, 6, 0, 0, 0, 0, loc)
	now := start.Add(48 * time.Hour)

	require.False(t, DailyExpired(&start, now, DailyPolicyOneTime))
}

func TestWeeklyExpiredUsesGlobalNaturalWeek(t *testing.T) {
	require.NoError(t, timezone.Init("Asia/Shanghai"))
	loc := timezone.Location()
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, loc)
	lastWeek := time.Date(2026, 6, 30, 0, 0, 0, 0, loc)
	thisWeek := timezone.StartOfWeek(now)

	require.True(t, WeeklyExpired(&lastWeek, now))
	require.False(t, WeeklyExpired(&thisWeek, now))
}

func TestMonthlyExpiredUsesThirtyDayRollingWindow(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	notExpired := now.Add(-29 * 24 * time.Hour)
	expired := now.Add(-30 * 24 * time.Hour)

	require.False(t, MonthlyExpired(&notExpired, now))
	require.True(t, MonthlyExpired(&expired, now))
}

func TestNextResetTimes(t *testing.T) {
	require.NoError(t, timezone.Init("Asia/Shanghai"))
	loc := timezone.Location()
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, loc)
	monthlyStart := time.Date(2026, 7, 1, 8, 30, 0, 0, loc)

	require.Equal(t, timezone.StartOfDay(now).AddDate(0, 0, 1), NextDailyReset(now))
	require.Equal(t, timezone.StartOfWeek(now).AddDate(0, 0, 7), NextWeeklyReset(now))
	require.Equal(t, monthlyStart.Add(30*24*time.Hour), NextMonthlyReset(&monthlyStart, now))
	require.Equal(t, now.Add(30*24*time.Hour), NextMonthlyReset(nil, now))
}
