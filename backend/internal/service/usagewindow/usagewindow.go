package usagewindow

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type DailyPolicy string

const (
	DailyPolicyNaturalDay DailyPolicy = "natural_day"
	DailyPolicyOneTime    DailyPolicy = "one_time"
)

func CurrentDailyStart(now time.Time) time.Time {
	return timezone.StartOfDay(now)
}

func CurrentWeeklyStart(now time.Time) time.Time {
	return timezone.StartOfWeek(now)
}

func CurrentMonthlyStart(existingStart *time.Time, now time.Time) time.Time {
	if existingStart == nil || MonthlyExpired(existingStart, now) {
		return now
	}
	return *existingStart
}

func DailyExpired(start *time.Time, now time.Time, policy DailyPolicy) bool {
	if start == nil || policy == DailyPolicyOneTime {
		return false
	}
	return start.Before(CurrentDailyStart(now))
}

func WeeklyExpired(start *time.Time, now time.Time) bool {
	if start == nil {
		return false
	}
	return start.Before(CurrentWeeklyStart(now))
}

func MonthlyExpired(start *time.Time, now time.Time) bool {
	if start == nil {
		return false
	}
	return now.Sub(*start) >= 30*24*time.Hour
}

func QuotaDailyExpired(start *time.Time, now time.Time) bool {
	return start == nil || DailyExpired(start, now, DailyPolicyNaturalDay)
}

func QuotaWeeklyExpired(start *time.Time, now time.Time) bool {
	return start == nil || WeeklyExpired(start, now)
}

func QuotaMonthlyExpired(start *time.Time, now time.Time) bool {
	return start == nil || MonthlyExpired(start, now)
}

func NextDailyReset(now time.Time) time.Time {
	return CurrentDailyStart(now).AddDate(0, 0, 1)
}

func NextWeeklyReset(now time.Time) time.Time {
	return CurrentWeeklyStart(now).AddDate(0, 0, 7)
}

func NextMonthlyReset(start *time.Time, now time.Time) time.Time {
	if start == nil || MonthlyExpired(start, now) {
		return now.Add(30 * 24 * time.Hour)
	}
	return start.Add(30 * 24 * time.Hour)
}
