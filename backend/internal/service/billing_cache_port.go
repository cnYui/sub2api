package service

import (
	"time"
)

// SubscriptionCacheData represents cached subscription data
type SubscriptionCacheData struct {
	Status       string
	ExpiresAt    time.Time
	DailyUsage   float64
	WeeklyUsage  float64
	MonthlyUsage float64
	Version      int64

	DailyWindowStart   *time.Time
	WeeklyWindowStart  *time.Time
	WeeklyAnchorAt     *time.Time
	MonthlyWindowStart *time.Time

	EntitlementPeriodID       *int64
	EntitlementWeeklyLimitUSD *float64
	PeriodTotalQuotaUSD       *float64
	EntitlementExpiresAt      *time.Time
	QuotaWindowUnit           string
	QuotaWindowDays           int
}
