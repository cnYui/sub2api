//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

type subscriptionUsageWindowRepoStub struct {
	userSubRepoNoop

	calibrateCalls int
	countCalls     int
	result         *SubscriptionDailyWindowCalibrationResult
	err            error
}

func (r *subscriptionUsageWindowRepoStub) CalibrateActiveDailyUsageWindows(context.Context, time.Time, time.Time, time.Time, int) (*SubscriptionDailyWindowCalibrationResult, error) {
	r.calibrateCalls++
	if r.err != nil {
		return nil, r.err
	}
	if r.result != nil {
		return r.result, nil
	}
	return &SubscriptionDailyWindowCalibrationResult{}, nil
}

func (r *subscriptionUsageWindowRepoStub) CountStaleActiveDailyWindows(context.Context, time.Time, time.Time) (int64, error) {
	r.countCalls++
	return 0, nil
}

func TestSubscriptionUsageWindowScheduler_RunOnceSkipsWhenNotLeader(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	_, _ = cache.TryAcquireLeaderLock(context.Background(), subscriptionUsageWindowLeaderLockKey, "peer", time.Minute)
	repo := &subscriptionUsageWindowRepoStub{}
	svc := NewSubscriptionUsageWindowScheduler(repo, nil, 200)
	svc.SetLeaderLock(cache, nil)

	svc.runOnce(context.Background(), timezone.StartOfDay(timezone.Now()).Add(time.Minute))

	require.Zero(t, repo.calibrateCalls)
}

func TestSubscriptionUsageWindowScheduler_RunOnceCalibratesBatches(t *testing.T) {
	repo := &subscriptionUsageWindowRepoStub{
		result: &SubscriptionDailyWindowCalibrationResult{
			Updated: []SubscriptionWindowCacheKey{{SubscriptionID: 1, UserID: 2, GroupID: 3}},
		},
	}
	cache := &fakeLeaderLockCache{}
	svc := NewSubscriptionUsageWindowScheduler(repo, nil, 200)
	svc.SetLeaderLock(cache, nil)

	svc.runOnce(context.Background(), timezone.StartOfDay(timezone.Now()).Add(time.Minute))

	require.Equal(t, 1, repo.calibrateCalls)
}

type subscriptionUsageWindowCacheStub struct {
	billingCacheWorkerStub

	invalidated []SubscriptionWindowCacheKey
}

func (c *subscriptionUsageWindowCacheStub) InvalidateSubscriptionCache(_ context.Context, userID, groupID int64) error {
	c.invalidated = append(c.invalidated, SubscriptionWindowCacheKey{UserID: userID, GroupID: groupID})
	return nil
}

func TestSubscriptionUsageWindowScheduler_RunOnceInvalidatesUpdatedSubscriptionCaches(t *testing.T) {
	updated := []SubscriptionWindowCacheKey{
		{SubscriptionID: 1, UserID: 2, GroupID: 3},
		{SubscriptionID: 4, UserID: 5, GroupID: 6},
	}
	repo := &subscriptionUsageWindowRepoStub{
		result: &SubscriptionDailyWindowCalibrationResult{Updated: updated},
	}
	cache := &subscriptionUsageWindowCacheStub{}
	billingCache := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(billingCache.Stop)

	svc := NewSubscriptionUsageWindowScheduler(repo, billingCache, 200)
	svc.SetLeaderLock(&fakeLeaderLockCache{}, nil)

	svc.runOnce(context.Background(), timezone.StartOfDay(timezone.Now()).Add(time.Minute))

	require.Equal(t, []SubscriptionWindowCacheKey{
		{UserID: 2, GroupID: 3},
		{UserID: 5, GroupID: 6},
	}, cache.invalidated)
}
