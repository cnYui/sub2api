//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

type subscriptionWindowRepoStub struct {
	userSubRepoNoop

	sub            *UserSubscription
	getActiveCalls int
	refreshCalls   int
	refreshErr     error
}

func (r *subscriptionWindowRepoStub) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*UserSubscription, error) {
	r.getActiveCalls++
	if r.sub == nil || r.sub.UserID != userID || r.sub.GroupID != groupID {
		return nil, ErrSubscriptionNotFound
	}
	return cloneSubscriptionForWindowTest(r.sub), nil
}

func (r *subscriptionWindowRepoStub) RefreshExpiredUsageWindows(_ context.Context, id int64, dailyStart, weeklyStart, monthlyStart, now time.Time) (bool, error) {
	r.refreshCalls++
	if r.refreshErr != nil {
		return false, r.refreshErr
	}
	if r.sub == nil || r.sub.ID != id {
		return false, ErrSubscriptionNotFound
	}

	changed := false
	if r.sub.DailyWindowStart == nil {
		r.sub.DailyWindowStart = timePtrForWindowTest(dailyStart)
		changed = true
	} else if r.sub.DailyWindowStart.Before(dailyStart) {
		r.sub.DailyUsageUSD = 0
		r.sub.DailyWindowStart = timePtrForWindowTest(dailyStart)
		changed = true
	}
	if r.sub.WeeklyWindowStart == nil {
		r.sub.WeeklyWindowStart = timePtrForWindowTest(weeklyStart)
		changed = true
	} else if r.sub.WeeklyWindowStart.Before(weeklyStart) {
		r.sub.WeeklyUsageUSD = 0
		r.sub.WeeklyWindowStart = timePtrForWindowTest(weeklyStart)
		changed = true
	}
	if r.sub.MonthlyWindowStart == nil {
		r.sub.MonthlyWindowStart = timePtrForWindowTest(monthlyStart)
		changed = true
	} else if !r.sub.MonthlyWindowStart.Add(30 * 24 * time.Hour).After(now) {
		r.sub.MonthlyUsageUSD = 0
		r.sub.MonthlyWindowStart = timePtrForWindowTest(monthlyStart)
		changed = true
	}
	return changed, nil
}

type subscriptionWindowCacheStub struct {
	BillingCache

	sub      *SubscriptionCacheData
	cacheErr error
	setCalls int
}

func (c *subscriptionWindowCacheStub) GetSubscriptionCache(_ context.Context, _, _ int64) (*SubscriptionCacheData, error) {
	if c.cacheErr != nil {
		return nil, c.cacheErr
	}
	if c.sub == nil {
		return nil, errors.New("cache miss")
	}
	cp := *c.sub
	return &cp, nil
}

func (c *subscriptionWindowCacheStub) SetSubscriptionCache(_ context.Context, _, _ int64, data *SubscriptionCacheData) error {
	c.setCalls++
	if data == nil {
		c.sub = nil
		return nil
	}
	cp := *data
	c.sub = &cp
	return nil
}

func (c *subscriptionWindowCacheStub) InvalidateSubscriptionCache(_ context.Context, _, _ int64) error {
	c.sub = nil
	return nil
}

func TestCheckBillingEligibility_RefreshesExpiredDailyWindowBeforeLimitCheck(t *testing.T) {
	today := timezone.StartOfDay(time.Now())
	yesterday := today.Add(-24 * time.Hour)
	now := time.Now()
	limit := 19.0

	repo := &subscriptionWindowRepoStub{sub: &UserSubscription{
		ID:                 101,
		UserID:             60,
		GroupID:            4,
		Status:             SubscriptionStatusActive,
		ExpiresAt:          now.Add(time.Hour),
		DailyWindowStart:   &yesterday,
		WeeklyWindowStart:  &today,
		MonthlyWindowStart: &now,
		DailyUsageUSD:      19.5,
	}}
	svc := NewBillingCacheService(nil, nil, repo, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(
		context.Background(),
		&User{ID: 60},
		nil,
		&Group{ID: 4, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &limit},
		repo.sub,
		PlatformOpenAI,
	)

	require.NoError(t, err)
	require.Equal(t, 1, repo.refreshCalls)
	require.Zero(t, repo.sub.DailyUsageUSD)
	require.True(t, repo.sub.DailyWindowStart.Equal(today))
}

func TestCheckBillingEligibility_OldSubscriptionCacheWithoutWindowsFallsBackToDB(t *testing.T) {
	today := timezone.StartOfDay(time.Now())
	now := time.Now()
	limit := 19.0

	cache := &subscriptionWindowCacheStub{sub: &SubscriptionCacheData{
		Status:     SubscriptionStatusActive,
		ExpiresAt:  now.Add(time.Hour),
		DailyUsage: 19.5,
	}}
	repo := &subscriptionWindowRepoStub{sub: &UserSubscription{
		ID:                 102,
		UserID:             60,
		GroupID:            4,
		Status:             SubscriptionStatusActive,
		ExpiresAt:          now.Add(time.Hour),
		DailyWindowStart:   &today,
		WeeklyWindowStart:  &today,
		MonthlyWindowStart: &now,
		DailyUsageUSD:      0,
	}}
	svc := NewBillingCacheService(cache, nil, repo, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(
		context.Background(),
		&User{ID: 60},
		nil,
		&Group{ID: 4, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &limit},
		repo.sub,
		PlatformOpenAI,
	)

	require.NoError(t, err)
	require.Equal(t, 1, repo.getActiveCalls)
	require.Eventually(t, func() bool {
		return cache.setCalls >= 1
	}, time.Second, 10*time.Millisecond)
}

func TestCheckBillingEligibility_RefreshesExpiredWeeklyAndMonthlyWindowsBeforeLimitCheck(t *testing.T) {
	today := timezone.StartOfDay(time.Now())
	thisWeek := timezone.StartOfWeek(time.Now())
	lastWeek := thisWeek.Add(-7 * 24 * time.Hour)
	oldMonth := time.Now().Add(-31 * 24 * time.Hour)
	now := time.Now()
	weeklyLimit := 20.0
	monthlyLimit := 50.0

	repo := &subscriptionWindowRepoStub{sub: &UserSubscription{
		ID:                 103,
		UserID:             60,
		GroupID:            4,
		Status:             SubscriptionStatusActive,
		ExpiresAt:          now.Add(time.Hour),
		DailyWindowStart:   &today,
		WeeklyWindowStart:  &lastWeek,
		MonthlyWindowStart: &oldMonth,
		WeeklyUsageUSD:     20.5,
		MonthlyUsageUSD:    51,
	}}
	svc := NewBillingCacheService(nil, nil, repo, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(
		context.Background(),
		&User{ID: 60},
		nil,
		&Group{ID: 4, SubscriptionType: SubscriptionTypeSubscription, WeeklyLimitUSD: &weeklyLimit, MonthlyLimitUSD: &monthlyLimit},
		repo.sub,
		PlatformOpenAI,
	)

	require.NoError(t, err)
	require.Equal(t, 1, repo.refreshCalls)
	require.Zero(t, repo.sub.WeeklyUsageUSD)
	require.Zero(t, repo.sub.MonthlyUsageUSD)
	require.True(t, repo.sub.WeeklyWindowStart.Equal(thisWeek))
}

func TestCheckBillingEligibility_CurrentDailyWindowStillRejectsRealExhaustion(t *testing.T) {
	today := timezone.StartOfDay(time.Now())
	now := time.Now()
	limit := 19.0

	repo := &subscriptionWindowRepoStub{sub: &UserSubscription{
		ID:                 104,
		UserID:             60,
		GroupID:            4,
		Status:             SubscriptionStatusActive,
		ExpiresAt:          now.Add(time.Hour),
		DailyWindowStart:   &today,
		WeeklyWindowStart:  &today,
		MonthlyWindowStart: &now,
		DailyUsageUSD:      19.5,
	}}
	svc := NewBillingCacheService(nil, nil, repo, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(
		context.Background(),
		&User{ID: 60},
		nil,
		&Group{ID: 4, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &limit},
		repo.sub,
		PlatformOpenAI,
	)

	require.ErrorIs(t, err, ErrDailyLimitExceeded)
	require.Zero(t, repo.refreshCalls)
}

func cloneSubscriptionForWindowTest(in *UserSubscription) *UserSubscription {
	if in == nil {
		return nil
	}
	cp := *in
	cp.DailyWindowStart = cloneTimePtrForWindowTest(in.DailyWindowStart)
	cp.WeeklyWindowStart = cloneTimePtrForWindowTest(in.WeeklyWindowStart)
	cp.MonthlyWindowStart = cloneTimePtrForWindowTest(in.MonthlyWindowStart)
	return &cp
}

func cloneTimePtrForWindowTest(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	return timePtrForWindowTest(*in)
}

func timePtrForWindowTest(t time.Time) *time.Time {
	v := t
	return &v
}
