package service

import (
	"encoding/json"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

const subscriptionSnapshotVersion = 1

// SubscriptionOrderSnapshot 固化下单时的套餐权益，履约和退款不得回读可变套餐配置。
type SubscriptionOrderSnapshot struct {
	Version             int      `json:"version"`
	PlanID              int64    `json:"plan_id"`
	PlanName            string   `json:"plan_name"`
	GroupID             int64    `json:"group_id"`
	GroupName           string   `json:"group_name"`
	ValidityDays        int      `json:"validity_days"`
	WeeklyLimitUSD      *float64 `json:"weekly_limit_usd,omitempty"`
	PeriodTotalQuotaUSD *float64 `json:"period_total_quota_usd,omitempty"`
	QuotaWindowUnit     string   `json:"quota_window_unit"`
	QuotaWindowDays     int      `json:"quota_window_days"`
}

func newSubscriptionOrderSnapshot(plan *dbent.SubscriptionPlan, group *Group) (*SubscriptionOrderSnapshot, error) {
	if plan == nil || group == nil || plan.GroupID != group.ID {
		return nil, fmt.Errorf("invalid subscription plan snapshot input")
	}
	days := psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit)
	snapshot := &SubscriptionOrderSnapshot{
		Version:         subscriptionSnapshotVersion,
		PlanID:          plan.ID,
		PlanName:        plan.Name,
		GroupID:         group.ID,
		GroupName:       group.Name,
		ValidityDays:    days,
		QuotaWindowUnit: "day",
		QuotaWindowDays: 1,
	}
	if group.UsesRollingWeeklyQuota() {
		snapshot.ValidityDays = publicCodexSubscriptionValidityDays
		snapshot.WeeklyLimitUSD = cloneOptionalFloat64(group.EffectiveWeeklyLimitUSD())
		if snapshot.WeeklyLimitUSD != nil {
			total := *snapshot.WeeklyLimitUSD * 4
			snapshot.PeriodTotalQuotaUSD = &total
		}
		snapshot.QuotaWindowUnit = "week"
		snapshot.QuotaWindowDays = subscriptionWeeklyWindowDays
	}
	return snapshot, nil
}

func (s SubscriptionOrderSnapshot) mapValue() map[string]any {
	data, _ := json.Marshal(s)
	var value map[string]any
	_ = json.Unmarshal(data, &value)
	return value
}

func subscriptionOrderSnapshotFromOrder(order *dbent.PaymentOrder) (*SubscriptionOrderSnapshot, error) {
	if order == nil || len(order.SubscriptionSnapshot) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(order.SubscriptionSnapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal subscription snapshot: %w", err)
	}
	var snapshot SubscriptionOrderSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("decode subscription snapshot: %w", err)
	}
	if snapshot.Version != subscriptionSnapshotVersion || snapshot.GroupID <= 0 || snapshot.ValidityDays <= 0 {
		return nil, fmt.Errorf("invalid subscription snapshot")
	}
	return &snapshot, nil
}

func (s *SubscriptionOrderSnapshot) rollingWeeklyGroupSnapshot() (*Group, bool) {
	if s == nil || s.GroupID <= 0 || s.WeeklyLimitUSD == nil || *s.WeeklyLimitUSD <= 0 || s.QuotaWindowUnit != "week" {
		return nil, false
	}
	group := &Group{
		ID:                  s.GroupID,
		Name:                s.GroupName,
		Status:              StatusActive,
		SubscriptionType:    SubscriptionTypeSubscription,
		WeeklyLimitUSD:      cloneOptionalFloat64(s.WeeklyLimitUSD),
		DefaultValidityDays: s.ValidityDays,
		Hydrated:            true,
	}
	return group, true
}
