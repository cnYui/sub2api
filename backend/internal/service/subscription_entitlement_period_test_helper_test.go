package service

import (
	"context"
	"fmt"
	"time"
)

type subscriptionEntitlementPeriodRepoStub struct {
	periods                 map[string]*SubscriptionEntitlementPeriod
	nextID                  int64
	revokeSubscriptionCalls []int64
	revokeSourceCalls       []SubscriptionEntitlementSource
	revokeReasons           []string
}

type wrappedSourceConflictEntitlementPeriodRepoStub struct {
	*subscriptionEntitlementPeriodRepoStub
	returnConflictOnce bool
}

func newSubscriptionEntitlementPeriodRepoStub() *subscriptionEntitlementPeriodRepoStub {
	return &subscriptionEntitlementPeriodRepoStub{
		periods: make(map[string]*SubscriptionEntitlementPeriod),
		nextID:  1,
	}
}

func (s *subscriptionEntitlementPeriodRepoStub) GetBySource(_ context.Context, source SubscriptionEntitlementSource) (*SubscriptionEntitlementPeriod, error) {
	period := s.periods[subscriptionEntitlementSourceKey(source)]
	if period == nil {
		return nil, ErrSubscriptionEntitlementPeriodNotFound
	}
	return cloneSubscriptionEntitlementPeriod(period), nil
}

func (s *subscriptionEntitlementPeriodRepoStub) Create(_ context.Context, period *SubscriptionEntitlementPeriod) error {
	key := subscriptionEntitlementSourceKey(period.Source)
	if _, exists := s.periods[key]; exists {
		return ErrSubscriptionEntitlementPeriodSourceExists
	}
	clone := cloneSubscriptionEntitlementPeriod(period)
	clone.ID = s.nextID
	s.nextID++
	period.ID = clone.ID
	s.periods[key] = clone
	return nil
}

func (s *subscriptionEntitlementPeriodRepoStub) RevokeUnexpiredBySubscription(_ context.Context, subscriptionID int64, now time.Time, reason string) error {
	s.revokeSubscriptionCalls = append(s.revokeSubscriptionCalls, subscriptionID)
	for _, period := range s.periods {
		if period.SubscriptionID != subscriptionID || period.Status != "active" || !period.ExpiresAt.After(now) {
			continue
		}
		period.Status = "revoked"
		period.RevokedAt = &now
		period.RevokedReason = reason
	}
	return nil
}

func (s *subscriptionEntitlementPeriodRepoStub) RevokeBySource(_ context.Context, source SubscriptionEntitlementSource, now time.Time, reason string) error {
	s.revokeSourceCalls = append(s.revokeSourceCalls, source)
	s.revokeReasons = append(s.revokeReasons, reason)
	period := s.periods[subscriptionEntitlementSourceKey(source)]
	if period == nil || period.Status != "active" || !period.ExpiresAt.After(now) {
		return nil
	}
	period.Status = "revoked"
	period.RevokedAt = &now
	period.RevokedReason = reason
	return nil
}

func (s *wrappedSourceConflictEntitlementPeriodRepoStub) Create(ctx context.Context, period *SubscriptionEntitlementPeriod) error {
	if !s.returnConflictOnce {
		return s.subscriptionEntitlementPeriodRepoStub.Create(ctx, period)
	}

	s.returnConflictOnce = false
	key := subscriptionEntitlementSourceKey(period.Source)
	stored := cloneSubscriptionEntitlementPeriod(period)
	stored.ID = s.nextID
	s.nextID++
	s.periods[key] = stored
	return fmt.Errorf("insert entitlement period: %w", fmt.Errorf("source unique violation: %w", ErrSubscriptionEntitlementPeriodSourceExists))
}

func cloneSubscriptionEntitlementPeriod(period *SubscriptionEntitlementPeriod) *SubscriptionEntitlementPeriod {
	if period == nil {
		return nil
	}
	clone := *period
	if period.DailyLimitUSD != nil {
		limit := *period.DailyLimitUSD
		clone.DailyLimitUSD = &limit
	}
	if period.RevokedAt != nil {
		revokedAt := *period.RevokedAt
		clone.RevokedAt = &revokedAt
	}
	return &clone
}

func subscriptionEntitlementSourceKey(source SubscriptionEntitlementSource) string {
	return source.Type + "\x00" + source.ID
}
