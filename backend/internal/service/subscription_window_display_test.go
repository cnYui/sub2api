package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeExpiredWindows_NullWindowsWithUsageAreHidden(t *testing.T) {
	subs := []UserSubscription{{
		ID:              1,
		DailyUsageUSD:   9.9,
		WeeklyUsageUSD:  19.9,
		MonthlyUsageUSD: 29.9,
	}}

	normalizeExpiredWindows(subs)

	require.InDelta(t, 0.0, subs[0].DailyUsageUSD, 1e-9)
	require.InDelta(t, 0.0, subs[0].WeeklyUsageUSD, 1e-9)
	require.InDelta(t, 0.0, subs[0].MonthlyUsageUSD, 1e-9)
	require.Nil(t, subs[0].DailyWindowStart)
	require.Nil(t, subs[0].WeeklyWindowStart)
	require.Nil(t, subs[0].MonthlyWindowStart)
}

func TestNormalizeExpiredWindows_CurrentWindowsKeepUsage(t *testing.T) {
	now := time.Now()
	subs := []UserSubscription{{
		ID:                 2,
		DailyWindowStart:   ptrTime(now),
		WeeklyWindowStart:  ptrTime(now),
		MonthlyWindowStart: ptrTime(now),
		DailyUsageUSD:      1.1,
		WeeklyUsageUSD:     2.2,
		MonthlyUsageUSD:    3.3,
		StartsAt:           now.Add(-time.Hour),
		ExpiresAt:          now.Add(24 * time.Hour),
	}}

	normalizeExpiredWindows(subs)

	require.InDelta(t, 1.1, subs[0].DailyUsageUSD, 1e-9)
	require.InDelta(t, 2.2, subs[0].WeeklyUsageUSD, 1e-9)
	require.InDelta(t, 3.3, subs[0].MonthlyUsageUSD, 1e-9)
}

type subscriptionProgressWindowRepoStub struct {
	userSubRepoNoop
	sub *UserSubscription
}

func (r *subscriptionProgressWindowRepoStub) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	if r.sub == nil || r.sub.ID != id {
		return nil, ErrSubscriptionNotFound
	}
	cp := *r.sub
	return &cp, nil
}

func TestGetSubscriptionProgress_NormalizesExpiredWindowBeforeProgress(t *testing.T) {
	pastStart := time.Now().Add(-48 * time.Hour)
	limit := 10.0
	repo := &subscriptionProgressWindowRepoStub{sub: &UserSubscription{
		ID:               3,
		GroupID:          4,
		StartsAt:         time.Now().Add(-72 * time.Hour),
		ExpiresAt:        time.Now().Add(24 * time.Hour),
		Status:           SubscriptionStatusActive,
		DailyWindowStart: ptrTime(pastStart),
		DailyUsageUSD:    9.9,
	}}
	group := &Group{ID: 4, Name: "Pro", DailyLimitUSD: &limit}
	svc := NewSubscriptionService(&subscriptionGroupRepoStub{group: group}, repo, nil, nil, nil)
	t.Cleanup(svc.Stop)

	progress, err := svc.GetSubscriptionProgress(context.Background(), 3)

	require.NoError(t, err)
	require.Nil(t, progress.Daily)
}
