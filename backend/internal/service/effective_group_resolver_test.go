package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type effectiveGroupSubRepoStub struct {
	subs []UserSubscription
	err  error
}

func (s *effectiveGroupSubRepoStub) ListActiveByUserID(ctx context.Context, userID int64) ([]UserSubscription, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]UserSubscription, 0, len(s.subs))
	for _, sub := range s.subs {
		if sub.UserID == userID {
			out = append(out, sub)
		}
	}
	return out, nil
}

type effectiveGroupGroupRepoStub struct {
	groups []Group
	err    error
}

func (s *effectiveGroupGroupRepoStub) ListActiveByPlatform(ctx context.Context, platform string) ([]Group, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]Group, 0, len(s.groups))
	for _, group := range s.groups {
		if group.Platform == platform && group.Status == StatusActive {
			out = append(out, group)
		}
	}
	return out, nil
}

type effectiveGroupTrafficRepoStub struct {
	has bool
	err error
}

func (s *effectiveGroupTrafficRepoStub) HasAvailableCredit(ctx context.Context, userID int64, now time.Time) (bool, error) {
	return s.has, s.err
}

func (s *effectiveGroupTrafficRepoStub) ListForSale(ctx context.Context) ([]TrafficPack, error) {
	return nil, nil
}

func (s *effectiveGroupTrafficRepoStub) GetForSaleByID(ctx context.Context, id int64) (*TrafficPack, error) {
	return nil, ErrInvalidInput
}

func (s *effectiveGroupTrafficRepoStub) GetSummary(ctx context.Context, userID int64, now time.Time) (*TrafficCreditSummary, error) {
	return &TrafficCreditSummary{}, nil
}

func (s *effectiveGroupTrafficRepoStub) CreditPurchase(ctx context.Context, input CreditTrafficPackInput) error {
	return nil
}

func (s *effectiveGroupTrafficRepoStub) Deduct(ctx context.Context, userID int64, amountUSD float64, requestID string, now time.Time) (bool, []TrafficCreditDeduction, error) {
	return false, nil, nil
}

func TestEffectiveGroupResolver_SubscriptionBeatsTrafficPack(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	limit19 := 19.0
	limit69 := 69.0
	lowGroup := &Group{ID: 2, Name: "codex-pool-19-usd", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &limit19}
	highGroup := &Group{ID: 9, Name: "codex-pool-69-usd", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &limit69}

	resolver := NewEffectiveGroupResolver(
		&effectiveGroupSubRepoStub{subs: []UserSubscription{
			{ID: 10, UserID: 7, GroupID: lowGroup.ID, Group: lowGroup, Status: SubscriptionStatusActive, ExpiresAt: now.Add(30 * 24 * time.Hour), CreatedAt: now.Add(-2 * time.Hour)},
			{ID: 11, UserID: 7, GroupID: highGroup.ID, Group: highGroup, Status: SubscriptionStatusActive, ExpiresAt: now.Add(30 * 24 * time.Hour), CreatedAt: now.Add(-1 * time.Hour)},
		}},
		&effectiveGroupGroupRepoStub{},
		NewTrafficPackService(&effectiveGroupTrafficRepoStub{has: true}),
	)
	resolver.now = func() time.Time { return now }

	result, err := resolver.ResolveEffectiveGroup(context.Background(), 7, PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, EffectiveGroupSourceSubscription, result.Source)
	require.Equal(t, int64(9), result.Group.ID)
	require.NotNil(t, result.Subscription)
	require.Equal(t, int64(11), result.Subscription.ID)
}

func TestEffectiveGroupResolver_TrafficPackUsesInternalOpenAIGroup(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	trafficGroup := Group{ID: 77, Name: TrafficPackOpenAIGroupName, Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, IsExclusive: true}

	resolver := NewEffectiveGroupResolver(
		&effectiveGroupSubRepoStub{},
		&effectiveGroupGroupRepoStub{groups: []Group{trafficGroup}},
		NewTrafficPackService(&effectiveGroupTrafficRepoStub{has: true}),
	)
	resolver.now = func() time.Time { return now }

	result, err := resolver.ResolveEffectiveGroup(context.Background(), 62, PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, EffectiveGroupSourceTrafficPack, result.Source)
	require.Equal(t, trafficGroup.ID, result.Group.ID)
	require.Nil(t, result.Subscription)
}

func TestEffectiveGroupResolver_NoOpenAIEntitlement(t *testing.T) {
	resolver := NewEffectiveGroupResolver(
		&effectiveGroupSubRepoStub{},
		&effectiveGroupGroupRepoStub{},
		NewTrafficPackService(&effectiveGroupTrafficRepoStub{has: false}),
	)

	result, err := resolver.ResolveEffectiveGroup(context.Background(), 62, PlatformOpenAI)
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrNoOpenAIEntitlement)
}

func TestEffectiveGroupResolver_SubscriptionLoadErrorDoesNotFallbackToTrafficPack(t *testing.T) {
	resolver := NewEffectiveGroupResolver(
		&effectiveGroupSubRepoStub{err: errors.New("db down")},
		&effectiveGroupGroupRepoStub{},
		NewTrafficPackService(&effectiveGroupTrafficRepoStub{has: true}),
	)

	result, err := resolver.ResolveEffectiveGroup(context.Background(), 62, PlatformOpenAI)
	require.Nil(t, result)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrNoOpenAIEntitlement)
}
