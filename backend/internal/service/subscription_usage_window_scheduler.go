package service

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/google/uuid"
)

const (
	subscriptionUsageWindowLeaderLockKey = "subscription:usage_window:daily:leader"
	subscriptionUsageWindowLeaderLockTTL = 10 * time.Minute
	subscriptionUsageWindowTickInterval  = time.Minute
	subscriptionUsageWindowBatchSize     = 200
	subscriptionUsageWindowRunTimeout    = 2 * time.Minute
)

type SubscriptionUsageWindowScheduler struct {
	userSubRepo  UserSubscriptionRepository
	billingCache *BillingCacheService
	batchSize    int

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string

	stopCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
}

func NewSubscriptionUsageWindowScheduler(userSubRepo UserSubscriptionRepository, billingCache *BillingCacheService, batchSize int) *SubscriptionUsageWindowScheduler {
	if batchSize <= 0 {
		batchSize = subscriptionUsageWindowBatchSize
	}
	return &SubscriptionUsageWindowScheduler{
		userSubRepo:  userSubRepo,
		billingCache: billingCache,
		batchSize:    batchSize,
		instanceID:   uuid.NewString(),
		stopCh:       make(chan struct{}),
	}
}

func (s *SubscriptionUsageWindowScheduler) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *SubscriptionUsageWindowScheduler) Start() {
	if s == nil || s.userSubRepo == nil {
		return
	}
	s.startOnce.Do(func() {
		s.wg.Add(1)
		go s.loop()
	})
}

func (s *SubscriptionUsageWindowScheduler) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *SubscriptionUsageWindowScheduler) loop() {
	defer s.wg.Done()

	s.runOnce(context.Background(), timezone.Now())

	ticker := time.NewTicker(subscriptionUsageWindowTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.runOnce(context.Background(), timezone.Now())
		case <-s.stopCh:
			return
		}
	}
}

func (s *SubscriptionUsageWindowScheduler) runOnce(parent context.Context, now time.Time) {
	if s == nil || s.userSubRepo == nil {
		return
	}
	if parent == nil {
		parent = context.Background()
	}
	if now.IsZero() {
		now = timezone.Now()
	}

	ctx, cancel := context.WithTimeout(parent, subscriptionUsageWindowRunTimeout)
	defer cancel()

	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, subscriptionUsageWindowLeaderLockKey, s.instanceID, subscriptionUsageWindowLeaderLockTTL)
	if !ok {
		return
	}
	defer release()

	dailyStart := timezone.StartOfDay(now)
	upperBound := now
	totalUpdated := int64(0)
	staleRemaining := int64(0)
	for {
		result, err := s.userSubRepo.CalibrateActiveDailyUsageWindows(ctx, dailyStart, upperBound, now, s.batchSize)
		if err != nil {
			logger.LegacyPrintf("service.subscription_usage_window", "[SubscriptionUsageWindow] calibrate failed: %v", err)
			return
		}
		if result == nil {
			break
		}
		updatedCount := result.UpdatedCount
		if updatedCount == 0 && len(result.Updated) > 0 {
			updatedCount = int64(len(result.Updated))
		}
		totalUpdated += updatedCount
		staleRemaining = result.StaleRemaining
		for _, key := range result.Updated {
			if s.billingCache != nil {
				if err := s.billingCache.InvalidateSubscription(ctx, key.UserID, key.GroupID); err != nil {
					logger.LegacyPrintf("service.subscription_usage_window", "[SubscriptionUsageWindow] invalidate subscription cache failed user=%d group=%d subscription=%d: %v", key.UserID, key.GroupID, key.SubscriptionID, err)
				}
			}
		}
		if updatedCount == 0 || result.StaleRemaining == 0 {
			break
		}
	}

	if totalUpdated > 0 {
		logger.LegacyPrintf("service.subscription_usage_window", "[SubscriptionUsageWindow] calibrated active daily windows updated=%d", totalUpdated)
	}
	if staleRemaining > 0 {
		logger.LegacyPrintf("service.subscription_usage_window", "ALERT: subscription usage daily window stale remaining=%d", staleRemaining)
	}
}
