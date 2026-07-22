package service

import (
	"math"
	"time"
)

const subscriptionWeeklyWindowDays = 7

// SubscriptionWeeklyWindow 是公共 Codex 订阅唯一的额度窗口事实。
// 金额保持精确值；展示层才可做整数四舍五入。
type SubscriptionWeeklyWindow struct {
	Start             time.Time
	End               time.Time
	EffectiveLimitUSD float64
	ResetsAt          time.Time
	Expired           bool
}

// CalculateSubscriptionWeeklyWindow 按权益锚点推进窗口，最后一个窗口按实际时长折算。
// weeklyWindowStart 是持久化的上次窗口起点，只用于校验和减少回放，不得改变锚点节奏。
func CalculateSubscriptionWeeklyWindow(anchorAt time.Time, weeklyWindowStart *time.Time, expiresAt, now time.Time, weeklyLimitUSD float64) SubscriptionWeeklyWindow {
	if anchorAt.IsZero() || !expiresAt.After(anchorAt) || !now.Before(expiresAt) {
		return SubscriptionWeeklyWindow{Expired: true, End: expiresAt, ResetsAt: expiresAt}
	}

	start := anchorAt
	if weeklyWindowStart != nil && subscriptionWeeklyWindowStartAligned(anchorAt, *weeklyWindowStart, expiresAt) && !weeklyWindowStart.After(now) {
		start = *weeklyWindowStart
	}
	for {
		end := start.AddDate(0, 0, subscriptionWeeklyWindowDays)
		if end.After(expiresAt) {
			end = expiresAt
		}
		if now.Before(end) {
			windowDays := end.Sub(start).Hours() / 24
			limit := weeklyLimitUSD * windowDays / subscriptionWeeklyWindowDays
			return SubscriptionWeeklyWindow{
				Start:             start,
				End:               end,
				EffectiveLimitUSD: limit,
				ResetsAt:          end,
			}
		}
		if !end.Before(expiresAt) {
			return SubscriptionWeeklyWindow{Expired: true, End: expiresAt, ResetsAt: expiresAt}
		}
		start = end
	}
}

func subscriptionWeeklyWindowStartAligned(anchorAt, candidate, expiresAt time.Time) bool {
	if candidate.Before(anchorAt) || !candidate.Before(expiresAt) {
		return false
	}
	for start := anchorAt; start.Before(expiresAt); start = start.AddDate(0, 0, subscriptionWeeklyWindowDays) {
		if start.Equal(candidate) {
			return true
		}
		if start.After(candidate) {
			return false
		}
	}
	return false
}

func (w SubscriptionWeeklyWindow) RemainingUSD(usageUSD float64) float64 {
	return math.Max(0, w.EffectiveLimitUSD-usageUSD)
}

func (w SubscriptionWeeklyWindow) Allows(usageUSD, additionalCostUSD float64) bool {
	return !w.Expired && usageUSD+additionalCostUSD <= w.EffectiveLimitUSD
}

func (s *UserSubscription) RollingWeeklyUsageUSD(window SubscriptionWeeklyWindow) float64 {
	if s == nil || window.Expired {
		return 0
	}
	if s.WeeklyWindowStart == nil || !s.WeeklyWindowStart.Equal(window.Start) {
		// 窗口事实已跨越但持久化行尚未结算时，本次判断必须从当前窗口零用量开始。
		return 0
	}
	return s.WeeklyUsageUSD
}

func (s *UserSubscription) WeeklyWindowAt(now time.Time, weeklyLimitUSD float64) SubscriptionWeeklyWindow {
	if s == nil {
		return SubscriptionWeeklyWindow{Expired: true}
	}
	anchor := s.StartsAt
	if s.WeeklyAnchorAt != nil {
		anchor = *s.WeeklyAnchorAt
	}
	return CalculateSubscriptionWeeklyWindow(anchor, s.WeeklyWindowStart, s.ExpiresAt, now, weeklyLimitUSD)
}

func (s *UserSubscription) CurrentRollingWeeklyWindow(group *Group, now time.Time) (SubscriptionWeeklyWindow, bool) {
	return s.RollingWeeklyWindowForEntitlement(group, s.CurrentEntitlementPeriod, now)
}

func (s *UserSubscription) HasRollingWeeklyQuotaFacts(group *Group, now time.Time) bool {
	if s == nil || group == nil || !group.UsesRollingWeeklyQuota() {
		return false
	}
	_, ok := s.CurrentRollingWeeklyWindow(group, now)
	return ok
}

func (s *UserSubscription) RollingWeeklyWindowForEntitlement(group *Group, period *SubscriptionEntitlementPeriod, now time.Time) (SubscriptionWeeklyWindow, bool) {
	if s == nil || group == nil || !group.UsesRollingWeeklyQuota() {
		return SubscriptionWeeklyWindow{Expired: true}, false
	}
	if period == nil || period.Status == SubscriptionEntitlementPeriodStatusRevoked || period.WeeklyLimitUSD == nil || *period.WeeklyLimitUSD <= 0 {
		return SubscriptionWeeklyWindow{Expired: true}, false
	}
	if now.IsZero() {
		now = time.Now()
	}
	if !period.StartsAt.IsZero() && now.Before(period.StartsAt) {
		return SubscriptionWeeklyWindow{Expired: true, End: period.StartsAt, ResetsAt: period.StartsAt}, false
	}
	if !period.ExpiresAt.IsZero() && !now.Before(period.ExpiresAt) {
		return SubscriptionWeeklyWindow{Expired: true, End: period.ExpiresAt, ResetsAt: period.ExpiresAt}, false
	}
	window := CalculateSubscriptionWeeklyWindow(period.StartsAt, s.WeeklyWindowStart, period.ExpiresAt, now, *period.WeeklyLimitUSD)
	return window, !window.Expired
}

func (s *UserSubscription) CurrentEntitlementPeriodID() *int64 {
	if s == nil || s.CurrentEntitlementPeriod == nil || s.CurrentEntitlementPeriod.ID <= 0 {
		return nil
	}
	id := s.CurrentEntitlementPeriod.ID
	return &id
}
