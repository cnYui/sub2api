package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type dailyResetTrackingUserSubRepo struct {
	userSubRepoNoop

	resetDailyCalled   bool
	refreshUsageCalled bool
}

func (r *dailyResetTrackingUserSubRepo) ResetDailyUsage(context.Context, int64, time.Time) error {
	r.resetDailyCalled = true
	return nil
}

func (r *dailyResetTrackingUserSubRepo) RefreshExpiredUsageWindows(context.Context, int64, time.Time, time.Time, time.Time, time.Time) (bool, error) {
	r.refreshUsageCalled = true
	return true, nil
}

func TestAssignOrExtendSubscription_ExpiredDailyCardStartsNewOneTimeQuota(t *testing.T) {
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	oldStart := time.Now().AddDate(0, 0, -3)
	oldWindowStart := startOfDay(oldStart)
	subRepo.seed(&UserSubscription{
		ID:                 100,
		UserID:             200,
		GroupID:            1,
		StartsAt:           oldStart,
		ExpiresAt:          oldStart.AddDate(0, 0, 1),
		Status:             SubscriptionStatusExpired,
		DailyWindowStart:   &oldWindowStart,
		WeeklyWindowStart:  &oldWindowStart,
		MonthlyWindowStart: &oldWindowStart,
		DailyUsageUSD:      10,
		WeeklyUsageUSD:     20,
		MonthlyUsageUSD:    30,
		Notes:              "old",
	})
	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)

	renewed, reused, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       200,
		GroupID:      1,
		ValidityDays: 1,
		Notes:        "new",
	})

	require.NoError(t, err)
	require.True(t, reused)
	require.True(t, renewed.HasOneTimeDailyQuota(), "过期后重新购买 1 日卡仍应被识别为一次性日额度")
	require.Equal(t, SubscriptionStatusActive, renewed.Status)
	require.True(t, renewed.StartsAt.After(oldStart), "重新购买过期订阅时应重置当前周期 StartsAt")
	require.False(t, renewed.ExpiresAt.After(renewed.StartsAt.AddDate(0, 0, 1)))
	require.NotNil(t, renewed.DailyWindowStart)
	require.Equal(t, startOfDay(renewed.StartsAt), *renewed.DailyWindowStart)
	require.Equal(t, 0.0, renewed.DailyUsageUSD)
	require.Equal(t, 0.0, renewed.WeeklyUsageUSD)
	require.Equal(t, 0.0, renewed.MonthlyUsageUSD)
	require.Equal(t, "old\nnew", renewed.Notes)
}

func TestUserSubscriptionNeedsDailyReset_DailyCardKeepsOneTimeQuota(t *testing.T) {
	start := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	dailyWindowStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	sub := &UserSubscription{
		StartsAt:         start,
		ExpiresAt:        start.Add(24 * time.Hour),
		DailyWindowStart: &dailyWindowStart,
		DailyUsageUSD:    10,
	}

	require.True(t, sub.HasOneTimeDailyQuota())
	require.False(t, sub.NeedsDailyResetAt(dailyWindowStart.Add(25*time.Hour)), "日卡应作为一次性配额，跨 0 点后不再刷新日额度")
}

func TestUserSubscriptionNeedsDailyReset_MultiDaySubscriptionStillRefreshes(t *testing.T) {
	start := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	dailyWindowStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	sub := &UserSubscription{
		StartsAt:         start,
		ExpiresAt:        start.AddDate(0, 0, 2),
		DailyWindowStart: &dailyWindowStart,
	}

	require.False(t, sub.HasOneTimeDailyQuota())
	require.True(t, sub.NeedsDailyResetAt(dailyWindowStart.Add(24*time.Hour)), "多日订阅仍应按 24 小时日窗口刷新")
}

func TestUserSubscriptionDailyResetTime_DailyCardReturnsExpiry(t *testing.T) {
	start := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	dailyWindowStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	expiresAt := start.Add(24 * time.Hour)
	sub := &UserSubscription{
		StartsAt:         start,
		ExpiresAt:        expiresAt,
		DailyWindowStart: &dailyWindowStart,
	}

	resetAt := sub.DailyResetTime()
	require.NotNil(t, resetAt)
	require.Equal(t, expiresAt, *resetAt, "日卡展示的日额度结束时间应为订阅过期时间")
}

func TestCheckAndResetWindows_DailyCardDoesNotResetDailyUsage(t *testing.T) {
	now := time.Now()
	startsAt := now.Add(-23 * time.Hour)
	dailyWindowStart := now.Add(-25 * time.Hour)
	repo := &dailyResetTrackingUserSubRepo{}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	sub := &UserSubscription{
		ID:               1,
		UserID:           10,
		GroupID:          20,
		StartsAt:         startsAt,
		ExpiresAt:        startsAt.Add(24 * time.Hour),
		DailyUsageUSD:    10,
		DailyWindowStart: &dailyWindowStart,
	}

	err := svc.CheckAndResetWindows(context.Background(), sub)

	require.NoError(t, err)
	require.False(t, repo.resetDailyCalled, "日卡作为一次性配额，过了 24 小时日窗口也不应重置 daily usage")
	require.False(t, repo.refreshUsageCalled, "日卡作为一次性配额，不应刷新订阅日窗口")
	require.Equal(t, 10.0, sub.DailyUsageUSD)
}

func TestCheckAndResetWindows_MultiDaySubscriptionStillResetsDailyUsage(t *testing.T) {
	now := time.Now()
	startsAt := now.Add(-48 * time.Hour)
	dailyWindowStart := now.Add(-25 * time.Hour)
	repo := &dailyResetTrackingUserSubRepo{}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	sub := &UserSubscription{
		ID:               1,
		UserID:           10,
		GroupID:          20,
		StartsAt:         startsAt,
		ExpiresAt:        startsAt.AddDate(0, 0, 2),
		DailyUsageUSD:    10,
		DailyWindowStart: &dailyWindowStart,
	}

	err := svc.CheckAndResetWindows(context.Background(), sub)

	require.NoError(t, err)
	require.True(t, repo.refreshUsageCalled, "多日订阅仍应刷新过期 daily window")
	require.Equal(t, 0.0, sub.DailyUsageUSD)
}

func TestCheckAndResetWindows_MultiDaySubscriptionCarriesOverDailyOverage(t *testing.T) {
	now := time.Now()
	startsAt := now.AddDate(0, 0, -5)
	dailyWindowStart := startOfDay(now.AddDate(0, 0, -1))
	dailyLimit := 15.0
	repo := &dailyResetTrackingUserSubRepo{}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	sub := &UserSubscription{
		ID:               1,
		UserID:           10,
		GroupID:          20,
		StartsAt:         startsAt,
		ExpiresAt:        startsAt.AddDate(0, 0, 10),
		DailyUsageUSD:    25,
		DailyWindowStart: &dailyWindowStart,
		Group:            &Group{DailyLimitUSD: &dailyLimit},
	}

	err := svc.CheckAndResetWindows(context.Background(), sub)

	require.NoError(t, err)
	require.True(t, repo.refreshUsageCalled)
	require.InDelta(t, 10.0, sub.DailyUsageUSD, 0.000001)
}

func TestUserSubscriptionDailyCarryoverUsageAt_CarriesOnlyOverage(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.Local)
	yesterday := startOfDay(now.AddDate(0, 0, -1))
	dailyLimit := 15.0
	sub := &UserSubscription{
		StartsAt:         now.AddDate(0, 0, -10),
		ExpiresAt:        now.AddDate(0, 0, 10),
		DailyWindowStart: &yesterday,
		DailyUsageUSD:    25,
	}
	group := &Group{DailyLimitUSD: &dailyLimit}

	require.InDelta(t, 10.0, sub.DailyCarryoverUsageAt(group, now), 0.000001)
}

func TestUserSubscriptionDailyCarryoverUsageAt_ReducesByElapsedDays(t *testing.T) {
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.Local)
	twoDaysAgo := startOfDay(now.AddDate(0, 0, -2))
	dailyLimit := 15.0
	sub := &UserSubscription{
		StartsAt:         now.AddDate(0, 0, -10),
		ExpiresAt:        now.AddDate(0, 0, 10),
		DailyWindowStart: &twoDaysAgo,
		DailyUsageUSD:    40,
	}
	group := &Group{DailyLimitUSD: &dailyLimit}

	require.InDelta(t, 10.0, sub.DailyCarryoverUsageAt(group, now), 0.000001)
}

func TestUserSubscriptionDailyCarryoverUsageAt_UnlimitedDoesNotCarry(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.Local)
	yesterday := startOfDay(now.AddDate(0, 0, -1))
	sub := &UserSubscription{
		StartsAt:         now.AddDate(0, 0, -10),
		ExpiresAt:        now.AddDate(0, 0, 10),
		DailyWindowStart: &yesterday,
		DailyUsageUSD:    25,
	}
	group := &Group{}

	require.Zero(t, sub.DailyCarryoverUsageAt(group, now))
}

func TestValidateAndCheckLimits_DailyCardDoesNotAllowSecondQuotaAfterMidnight(t *testing.T) {
	start := time.Now().Add(-23 * time.Hour)
	dailyWindowStart := time.Now().Add(-25 * time.Hour)
	dailyLimit := 10.0
	sub := &UserSubscription{
		Status:           SubscriptionStatusActive,
		StartsAt:         start,
		ExpiresAt:        start.Add(24 * time.Hour),
		DailyWindowStart: &dailyWindowStart,
		DailyUsageUSD:    dailyLimit + 0.01,
	}
	group := &Group{
		SubscriptionType: SubscriptionTypeSubscription,
		DailyLimitUSD:    &dailyLimit,
	}
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)

	needsMaintenance, err := svc.ValidateAndCheckLimits(sub, group)

	require.False(t, needsMaintenance, "日卡跨过日窗口后不应触发 daily reset 维护")
	require.True(t, errors.Is(err, ErrDailyLimitExceeded))
	require.Equal(t, dailyLimit+0.01, sub.DailyUsageUSD, "热路径不应清零日卡已用额度")
}

func TestValidateAndCheckLimits_MultiDaySubscriptionUsesDailyCarryover(t *testing.T) {
	now := time.Now()
	dailyWindowStart := startOfDay(now.AddDate(0, 0, -1))
	dailyLimit := 15.0
	sub := &UserSubscription{
		Status:           SubscriptionStatusActive,
		StartsAt:         now.AddDate(0, 0, -5),
		ExpiresAt:        now.AddDate(0, 0, 5),
		DailyWindowStart: &dailyWindowStart,
		DailyUsageUSD:    25.0,
	}
	group := &Group{
		SubscriptionType: SubscriptionTypeSubscription,
		DailyLimitUSD:    &dailyLimit,
	}
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)

	needsMaintenance, err := svc.ValidateAndCheckLimits(sub, group)

	require.NoError(t, err)
	require.True(t, needsMaintenance)
	require.InDelta(t, 10.0, sub.DailyUsageUSD, 0.000001)
}

func TestValidateAndCheckLimits_MultiDaySubscriptionRejectsCarryoverExhaustedDay(t *testing.T) {
	now := time.Now()
	dailyWindowStart := startOfDay(now.AddDate(0, 0, -1))
	dailyLimit := 15.0
	sub := &UserSubscription{
		Status:           SubscriptionStatusActive,
		StartsAt:         now.AddDate(0, 0, -5),
		ExpiresAt:        now.AddDate(0, 0, 5),
		DailyWindowStart: &dailyWindowStart,
		DailyUsageUSD:    31.0,
	}
	group := &Group{
		SubscriptionType: SubscriptionTypeSubscription,
		DailyLimitUSD:    &dailyLimit,
	}
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)

	needsMaintenance, err := svc.ValidateAndCheckLimits(sub, group)

	require.True(t, needsMaintenance)
	require.True(t, errors.Is(err, ErrDailyLimitExceeded))
	require.InDelta(t, 16.0, sub.DailyUsageUSD, 0.000001)
}
