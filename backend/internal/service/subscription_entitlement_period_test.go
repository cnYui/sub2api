//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func newEntitlementSubscriptionServiceForTest(
	groupRepo GroupRepository,
	userSubRepo UserSubscriptionRepository,
	entitlementRepo SubscriptionEntitlementPeriodRepository,
	now time.Time,
) *SubscriptionService {
	svc := NewSubscriptionService(groupRepo, userSubRepo, nil, nil, nil)
	svc.entitlementPeriodRepo = entitlementRepo
	svc.now = func() time.Time { return now }
	return svc
}

func TestGrantSubscriptionEntitlement_ReplaysSameSourceWithoutExtendingSubscription(t *testing.T) {
	now := time.Date(2030, 7, 16, 9, 0, 0, 0, time.UTC)
	limit := 19.0
	groupRepo := &subscriptionGroupRepoStub{group: &Group{
		ID:               7,
		SubscriptionType: SubscriptionTypeSubscription,
		DailyLimitUSD:    &limit,
	}}
	userSubRepo := newSubscriptionUserSubRepoStub()
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	svc := newEntitlementSubscriptionServiceForTest(groupRepo, userSubRepo, entitlementRepo, now)
	input := &AssignSubscriptionInput{
		UserID:       11,
		GroupID:      7,
		ValidityDays: 30,
		EntitlementSource: SubscriptionEntitlementSource{
			Type: "payment_order",
			ID:   "order-11",
		},
	}

	first, err := svc.GrantSubscriptionEntitlement(context.Background(), input)
	require.NoError(t, err)
	require.False(t, first.Replayed)
	require.NotNil(t, first.Period)

	replayed, err := svc.GrantSubscriptionEntitlement(context.Background(), input)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, first.Period.ID, replayed.Period.ID)
	require.Equal(t, first.Subscription.ExpiresAt, replayed.Subscription.ExpiresAt)
	require.Equal(t, 1, userSubRepo.createCalls)
}

func TestGrantSubscriptionEntitlement_ReplaysExistingSourceBeforeTakingUserLock(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })

	userSubRepo := newSubscriptionUserSubRepoStub()
	userSubRepo.seed(&UserSubscription{
		ID:        1,
		UserID:    11,
		GroupID:   7,
		Status:    SubscriptionStatusActive,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	})
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	source := SubscriptionEntitlementSource{Type: "payment_order", ID: "order-replay-before-lock"}
	entitlementRepo.periods[subscriptionEntitlementSourceKey(source)] = &SubscriptionEntitlementPeriod{
		ID:             1,
		UserID:         11,
		SubscriptionID: 1,
		GroupID:        7,
		Source:         source,
		StartsAt:       time.Now().Add(-24 * time.Hour),
		ExpiresAt:      time.Now().Add(29 * 24 * time.Hour),
		PeriodDays:     30,
		Status:         SubscriptionEntitlementPeriodStatusActive,
	}
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepo, nil, client, nil)
	svc.entitlementPeriodRepo = entitlementRepo

	mock.ExpectBegin()
	mock.ExpectCommit()
	result, err := svc.GrantSubscriptionEntitlement(ctx, &AssignSubscriptionInput{
		UserID:            11,
		GroupID:           7,
		EntitlementSource: source,
	})

	require.NoError(t, err)
	require.True(t, result.Replayed)
	require.Equal(t, int64(1), result.Subscription.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrantSubscriptionEntitlement_ReplaysWrappedSourceUniqueConflict(t *testing.T) {
	now := time.Date(2030, 7, 16, 9, 0, 0, 0, time.UTC)
	groupRepo := &subscriptionGroupRepoStub{group: &Group{
		ID:               7,
		SubscriptionType: SubscriptionTypeSubscription,
	}}
	userSubRepo := newSubscriptionUserSubRepoStub()
	entitlementRepo := &wrappedSourceConflictEntitlementPeriodRepoStub{
		subscriptionEntitlementPeriodRepoStub: newSubscriptionEntitlementPeriodRepoStub(),
		returnConflictOnce:                    true,
	}
	svc := newEntitlementSubscriptionServiceForTest(groupRepo, userSubRepo, entitlementRepo, now)

	result, err := svc.GrantSubscriptionEntitlement(context.Background(), &AssignSubscriptionInput{
		UserID:       11,
		GroupID:      7,
		ValidityDays: 30,
		EntitlementSource: SubscriptionEntitlementSource{
			Type: "payment_order",
			ID:   "order-11-race",
		},
	})

	require.NoError(t, err)
	require.True(t, result.Replayed)
	require.Len(t, entitlementRepo.periods, 1)
	require.Equal(t, 1, userSubRepo.createCalls)
}

func TestGrantSubscriptionEntitlement_ContinuesEarlyRenewalAndSnapshotsDailyLimit(t *testing.T) {
	now := time.Date(2030, 7, 16, 9, 0, 0, 0, time.UTC)
	firstLimit := 19.0
	secondLimit := 23.0
	groupRepo := &subscriptionGroupRepoStub{group: &Group{
		ID:               7,
		SubscriptionType: SubscriptionTypeSubscription,
		DailyLimitUSD:    &firstLimit,
	}}
	userSubRepo := newSubscriptionUserSubRepoStub()
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	svc := newEntitlementSubscriptionServiceForTest(groupRepo, userSubRepo, entitlementRepo, now)

	first, err := svc.GrantSubscriptionEntitlement(context.Background(), &AssignSubscriptionInput{
		UserID:       11,
		GroupID:      7,
		ValidityDays: 30,
		EntitlementSource: SubscriptionEntitlementSource{
			Type: "payment_order",
			ID:   "order-11-first",
		},
	})
	require.NoError(t, err)

	groupRepo.group.DailyLimitUSD = &secondLimit
	second, err := svc.GrantSubscriptionEntitlement(context.Background(), &AssignSubscriptionInput{
		UserID:       11,
		GroupID:      7,
		ValidityDays: 30,
		EntitlementSource: SubscriptionEntitlementSource{
			Type: "payment_order",
			ID:   "order-11-second",
		},
	})
	require.NoError(t, err)
	require.Equal(t, first.Period.ExpiresAt, second.Period.StartsAt)
	require.Equal(t, 30, second.Period.PeriodDays)
	require.InDelta(t, 19.0, *first.Period.DailyLimitUSD, 0.0000001)
	require.InDelta(t, 23.0, *second.Period.DailyLimitUSD, 0.0000001)
	require.Equal(t, second.Period.ExpiresAt, second.Subscription.ExpiresAt)
}

func TestAssignOrExtendSubscription_WithEntitlementSource_ReplaysWithoutExtending(t *testing.T) {
	now := time.Date(2030, 7, 16, 9, 0, 0, 0, time.UTC)
	limit := 19.0
	groupRepo := &subscriptionGroupRepoStub{group: &Group{
		ID:               7,
		SubscriptionType: SubscriptionTypeSubscription,
		DailyLimitUSD:    &limit,
	}}
	userSubRepo := newSubscriptionUserSubRepoStub()
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	svc := newEntitlementSubscriptionServiceForTest(groupRepo, userSubRepo, entitlementRepo, now)
	input := &AssignSubscriptionInput{
		UserID:       11,
		GroupID:      7,
		ValidityDays: 30,
		EntitlementSource: SubscriptionEntitlementSource{
			Type: "payment_order",
			ID:   "order-assign-or-extend",
		},
	}

	first, extended, err := svc.AssignOrExtendSubscription(context.Background(), input)
	require.NoError(t, err)
	require.False(t, extended)

	replayed, replayExtended, err := svc.AssignOrExtendSubscription(context.Background(), input)
	require.NoError(t, err)
	require.True(t, replayExtended)
	require.Equal(t, first.ID, replayed.ID)
	require.Equal(t, first.ExpiresAt, replayed.ExpiresAt)
	require.Len(t, entitlementRepo.periods, 1)
}

func TestAssignSubscription_WithEntitlementSource_ContinuesFromExistingExpiry(t *testing.T) {
	now := time.Date(2030, 7, 16, 9, 0, 0, 0, time.UTC)
	limit := 19.0
	groupRepo := &subscriptionGroupRepoStub{group: &Group{
		ID:               7,
		SubscriptionType: SubscriptionTypeSubscription,
		DailyLimitUSD:    &limit,
	}}
	userSubRepo := newSubscriptionUserSubRepoStub()
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	svc := newEntitlementSubscriptionServiceForTest(groupRepo, userSubRepo, entitlementRepo, now)

	first, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       11,
		GroupID:      7,
		ValidityDays: 30,
		EntitlementSource: SubscriptionEntitlementSource{
			Type: "payment_order",
			ID:   "order-assign-first",
		},
	})
	require.NoError(t, err)

	second, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       11,
		GroupID:      7,
		ValidityDays: 30,
		EntitlementSource: SubscriptionEntitlementSource{
			Type: "payment_order",
			ID:   "order-assign-second",
		},
	})
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.ExpiresAt.AddDate(0, 0, 30), second.ExpiresAt)
	require.Len(t, entitlementRepo.periods, 2)
}

func TestGrantSubscriptionEntitlement_StoresActualTruncatedPeriodDays(t *testing.T) {
	now := MaxExpiresAt.Add(-24 * time.Hour)
	groupRepo := &subscriptionGroupRepoStub{group: &Group{
		ID:               7,
		SubscriptionType: SubscriptionTypeSubscription,
	}}
	userSubRepo := newSubscriptionUserSubRepoStub()
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	svc := newEntitlementSubscriptionServiceForTest(groupRepo, userSubRepo, entitlementRepo, now)

	result, err := svc.GrantSubscriptionEntitlement(context.Background(), &AssignSubscriptionInput{
		UserID:       11,
		GroupID:      7,
		ValidityDays: 30,
		EntitlementSource: SubscriptionEntitlementSource{
			Type: "payment_order",
			ID:   "order-truncated-term",
		},
	})
	require.NoError(t, err)
	require.Equal(t, MaxExpiresAt, result.Period.ExpiresAt)
	require.Equal(t, 1, result.Period.PeriodDays)
}

func TestAssignOrExtendSubscription_WithoutEntitlementSource_DoesNotCreatePeriod(t *testing.T) {
	now := time.Date(2030, 7, 16, 9, 0, 0, 0, time.UTC)
	groupRepo := &subscriptionGroupRepoStub{group: &Group{
		ID:               7,
		SubscriptionType: SubscriptionTypeSubscription,
	}}
	userSubRepo := newSubscriptionUserSubRepoStub()
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	svc := newEntitlementSubscriptionServiceForTest(groupRepo, userSubRepo, entitlementRepo, now)

	_, _, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       11,
		GroupID:      7,
		ValidityDays: 30,
	})
	require.NoError(t, err)
	require.Empty(t, entitlementRepo.periods)
}

func TestAssignSubscription_WithoutEntitlementSource_DoesNotCreatePeriod(t *testing.T) {
	now := time.Date(2030, 7, 16, 9, 0, 0, 0, time.UTC)
	groupRepo := &subscriptionGroupRepoStub{group: &Group{
		ID:               7,
		SubscriptionType: SubscriptionTypeSubscription,
	}}
	userSubRepo := newSubscriptionUserSubRepoStub()
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	svc := newEntitlementSubscriptionServiceForTest(groupRepo, userSubRepo, entitlementRepo, now)

	subscription, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       11,
		GroupID:      7,
		ValidityDays: 30,
	})

	require.NoError(t, err)
	require.NotNil(t, subscription)
	require.Equal(t, 1, userSubRepo.createCalls)
	require.Empty(t, entitlementRepo.periods)
}

func TestRevokeSubscription_RevokesUnexpiredEntitlementPeriodsBeforeDeletingSubscription(t *testing.T) {
	now := time.Date(2030, 7, 16, 9, 0, 0, 0, time.UTC)
	userSubRepo := &subscriptionRevokeRepoStub{sub: &UserSubscription{
		ID:        88,
		UserID:    69,
		GroupID:   8,
		Status:    SubscriptionStatusActive,
		ExpiresAt: now.AddDate(0, 0, 30),
	}}
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	period := &SubscriptionEntitlementPeriod{
		ID:             1,
		UserID:         69,
		SubscriptionID: 88,
		GroupID:        8,
		Source:         SubscriptionEntitlementSource{Type: "payment_order", ID: "order-88"},
		StartsAt:       now,
		ExpiresAt:      now.AddDate(0, 0, 30),
		PeriodDays:     30,
		Status:         "active",
	}
	entitlementRepo.periods[subscriptionEntitlementSourceKey(period.Source)] = period
	svc := newEntitlementSubscriptionServiceForTest(groupRepoNoop{}, userSubRepo, entitlementRepo, now)

	require.NoError(t, svc.RevokeSubscription(context.Background(), 88))
	require.Equal(t, []int64{88}, entitlementRepo.revokeSubscriptionCalls)
	require.Equal(t, "revoked", period.Status)
	require.NotNil(t, period.RevokedAt)
	require.Equal(t, "subscription_revoked", period.RevokedReason)
	require.Equal(t, []string{"get", "status", "delete"}, userSubRepo.callOrder)
}

func TestExtendSubscription_PositiveAdjustmentAppendsAdminAdjustmentEntitlementPeriod(t *testing.T) {
	now := time.Date(2030, 7, 16, 9, 0, 0, 0, time.UTC)
	oldExpiresAt := now.AddDate(0, 0, 20)
	newExpiresAt := oldExpiresAt.AddDate(0, 0, 10)
	dailyLimit := 19.0
	userSubRepo := newSubscriptionUserSubRepoStub()
	userSubRepo.seed(&UserSubscription{
		ID:        90,
		UserID:    70,
		GroupID:   9,
		Status:    SubscriptionStatusActive,
		StartsAt:  now.AddDate(0, 0, -10),
		ExpiresAt: oldExpiresAt,
	})
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	svc := newEntitlementSubscriptionServiceForTest(&subscriptionGroupRepoStub{
		group: &Group{
			ID:               9,
			SubscriptionType: SubscriptionTypeSubscription,
			DailyLimitUSD:    &dailyLimit,
		},
	}, userSubRepo, entitlementRepo, now)

	subscription, err := svc.ExtendSubscription(context.Background(), 90, 10)

	require.NoError(t, err)
	require.Equal(t, newExpiresAt, subscription.ExpiresAt)
	period, err := entitlementRepo.GetBySource(context.Background(), adminAdjustmentSubscriptionEntitlementSource(90, newExpiresAt))
	require.NoError(t, err)
	require.Equal(t, int64(70), period.UserID)
	require.Equal(t, int64(9), period.GroupID)
	require.Equal(t, int64(90), period.SubscriptionID)
	require.Equal(t, oldExpiresAt, period.StartsAt)
	require.Equal(t, newExpiresAt, period.ExpiresAt)
	require.Equal(t, 10, period.PeriodDays)
	require.InDelta(t, dailyLimit, *period.DailyLimitUSD, 0.0000001)
}

func TestExtendSubscription_NegativeAdjustmentRevokesUnexpiredEntitlementPeriods(t *testing.T) {
	now := time.Date(2030, 7, 16, 9, 0, 0, 0, time.UTC)
	userSubRepo := newSubscriptionUserSubRepoStub()
	userSubRepo.seed(&UserSubscription{
		ID:        91,
		UserID:    71,
		GroupID:   10,
		Status:    SubscriptionStatusActive,
		StartsAt:  now.AddDate(0, 0, -10),
		ExpiresAt: now.AddDate(0, 0, 20),
	})
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	entitlementRepo.periods[subscriptionEntitlementSourceKey(SubscriptionEntitlementSource{Type: "payment_order", ID: "order-91"})] = &SubscriptionEntitlementPeriod{
		ID:             1,
		UserID:         71,
		SubscriptionID: 91,
		GroupID:        10,
		Source:         SubscriptionEntitlementSource{Type: "payment_order", ID: "order-91"},
		StartsAt:       now.AddDate(0, 0, -10),
		ExpiresAt:      now.AddDate(0, 0, 20),
		PeriodDays:     30,
		Status:         SubscriptionEntitlementPeriodStatusActive,
	}
	svc := newEntitlementSubscriptionServiceForTest(&subscriptionGroupRepoStub{
		group: &Group{ID: 10, SubscriptionType: SubscriptionTypeSubscription},
	}, userSubRepo, entitlementRepo, now)

	subscription, err := svc.ExtendSubscription(context.Background(), 91, -5)

	require.NoError(t, err)
	require.Equal(t, now.AddDate(0, 0, 15), subscription.ExpiresAt)
	require.Equal(t, []int64{91}, entitlementRepo.revokeSubscriptionCalls)
	require.Equal(t, "revoked", entitlementRepo.periods[subscriptionEntitlementSourceKey(SubscriptionEntitlementSource{Type: "payment_order", ID: "order-91"})].Status)
	require.Equal(t, "admin_adjustment_negative", entitlementRepo.periods[subscriptionEntitlementSourceKey(SubscriptionEntitlementSource{Type: "payment_order", ID: "order-91"})].RevokedReason)
}

func TestWithSubscriptionUpdateTx_ReusesOuterTransactionWithoutNestedBegin(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	outerTx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := dbent.NewTxContext(ctx, outerTx)
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, client, nil)
	callbackErr := errors.New("rollback outer transaction")

	err = svc.withSubscriptionUpdateTx(txCtx, func(receivedCtx context.Context) error {
		require.Same(t, outerTx, dbent.TxFromContext(receivedCtx))
		return callbackErr
	})
	require.ErrorIs(t, err, callbackErr)

	mock.ExpectRollback()
	require.NoError(t, outerTx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithSubscriptionUpdateTx_RollsBackOwnedTransactionOnCallbackFailure(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	mock.ExpectRollback()
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, client, nil)
	callbackErr := fmt.Errorf("force rollback")

	err = svc.withSubscriptionUpdateTx(ctx, func(context.Context) error {
		return callbackErr
	})
	require.ErrorIs(t, err, callbackErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRevokeSubscription_DefersCacheInvalidationToOuterTransactionOwner(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })

	repo := &subscriptionRevokeRepoStub{
		sub: &UserSubscription{
			ID:        88,
			UserID:    69,
			GroupID:   8,
			Status:    SubscriptionStatusActive,
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, client, &config.Config{
		SubscriptionCache: config.SubscriptionCacheConfig{
			L1Size:       1000,
			L1TTLSeconds: 60,
		},
	})
	require.NotNil(t, svc.subCacheL1)
	require.True(t, svc.subCacheL1.Set(subCacheKey(69, 8), "cached", 1))
	svc.subCacheL1.Wait()
	_, cachedBeforeRevoke := svc.subCacheL1.Get(subCacheKey(69, 8))
	require.True(t, cachedBeforeRevoke)

	mock.ExpectBegin()
	outerTx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := dbent.NewTxContext(ctx, outerTx)

	require.NoError(t, svc.RevokeSubscription(txCtx, 88))
	svc.subCacheL1.Wait()
	_, cached := svc.subCacheL1.Get(subCacheKey(69, 8))
	require.True(t, cached)

	mock.ExpectRollback()
	require.NoError(t, outerTx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRevokeSubscription_InvalidatesCacheAfterOwnedTransactionCommits(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })

	repo := &subscriptionRevokeRepoStub{
		sub: &UserSubscription{
			ID:        89,
			UserID:    70,
			GroupID:   9,
			Status:    SubscriptionStatusActive,
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, client, &config.Config{
		SubscriptionCache: config.SubscriptionCacheConfig{
			L1Size:       1000,
			L1TTLSeconds: 60,
		},
	})
	require.True(t, svc.subCacheL1.Set(subCacheKey(70, 9), "cached", 1))
	svc.subCacheL1.Wait()
	_, cachedBeforeRevoke := svc.subCacheL1.Get(subCacheKey(70, 9))
	require.True(t, cachedBeforeRevoke)

	mock.ExpectBegin()
	mock.ExpectCommit()
	require.NoError(t, svc.RevokeSubscription(ctx, 89))
	svc.subCacheL1.Wait()
	_, cached := svc.subCacheL1.Get(subCacheKey(70, 9))
	require.False(t, cached)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRevokeSubscription_RegistersOneCacheInvalidationAfterOuterTransactionCommits(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })

	repo := &subscriptionRevokeRepoStub{
		sub: &UserSubscription{
			ID:        90,
			UserID:    71,
			GroupID:   10,
			Status:    SubscriptionStatusActive,
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		},
	}
	cache := newBillingCacheStub(2)
	billingCacheService := &BillingCacheService{cache: cache}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, billingCacheService, client, nil)

	mock.ExpectBegin()
	outerTx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := dbent.NewTxContext(ctx, outerTx)

	require.NoError(t, svc.RevokeSubscription(txCtx, 90))
	require.NoError(t, svc.RevokeSubscription(txCtx, 90))
	select {
	case invalidation := <-cache.invalidations:
		t.Fatalf("cache invalidated before outer commit: %+v", invalidation)
	default:
	}

	mock.ExpectCommit()
	require.NoError(t, outerTx.Commit())
	calls := waitForInvalidations(t, cache.invalidations, 1)
	require.Equal(t, subscriptionInvalidateCall{userID: 71, groupID: 10}, calls[0])
	require.Never(t, func() bool {
		select {
		case <-cache.invalidations:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 10*time.Millisecond)
	require.NoError(t, mock.ExpectationsWereMet())
}
