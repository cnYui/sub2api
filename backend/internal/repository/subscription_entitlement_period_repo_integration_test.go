//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionentitlementperiod"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionEntitlementPeriodRepository_CreateGetAndRejectDuplicateSource(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := NewSubscriptionEntitlementPeriodRepository(client)
	user, group, subscription := createSubscriptionEntitlementPeriodFixture(t, ctx, client)

	period := newSubscriptionEntitlementPeriodFixture(user.ID, group.ID, subscription.ID, service.SubscriptionEntitlementSource{
		Type: "payment_order",
		ID:   fmt.Sprintf("order-%d", time.Now().UnixNano()),
	})
	require.NoError(t, repo.Create(ctx, period))
	require.NotZero(t, period.ID)

	loaded, err := repo.GetBySource(ctx, period.Source)
	require.NoError(t, err)
	require.Equal(t, period.ID, loaded.ID)
	require.Equal(t, period.SubscriptionID, loaded.SubscriptionID)
	require.Equal(t, *period.DailyLimitUSD, *loaded.DailyLimitUSD)

	duplicate := *period
	duplicate.ID = 0
	err = repo.Create(ctx, &duplicate)
	require.ErrorIs(t, err, service.ErrSubscriptionEntitlementPeriodSourceExists)
}

func TestSubscriptionEntitlementPeriodRepository_RevokeOnlyUnexpiredActivePeriods(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := NewSubscriptionEntitlementPeriodRepository(client)
	user, group, subscription := createSubscriptionEntitlementPeriodFixture(t, ctx, client)
	now := time.Now().UTC()

	active := newSubscriptionEntitlementPeriodFixture(user.ID, group.ID, subscription.ID, service.SubscriptionEntitlementSource{
		Type: "payment_order",
		ID:   fmt.Sprintf("active-%d", time.Now().UnixNano()),
	})
	active.StartsAt = now.Add(-time.Hour)
	active.ExpiresAt = now.Add(24 * time.Hour)
	require.NoError(t, repo.Create(ctx, active))

	expired := newSubscriptionEntitlementPeriodFixture(user.ID, group.ID, subscription.ID, service.SubscriptionEntitlementSource{
		Type: "payment_order",
		ID:   fmt.Sprintf("expired-%d", time.Now().UnixNano()),
	})
	expired.StartsAt = now.Add(-48 * time.Hour)
	expired.ExpiresAt = now.Add(-time.Hour)
	require.NoError(t, repo.Create(ctx, expired))

	require.NoError(t, repo.RevokeUnexpiredBySubscription(ctx, subscription.ID, now, "refund"))

	activeEntity, err := client.SubscriptionEntitlementPeriod.Get(ctx, active.ID)
	require.NoError(t, err)
	require.Equal(t, "revoked", activeEntity.Status)
	require.NotNil(t, activeEntity.RevokedAt)
	require.Equal(t, "refund", activeEntity.RevokedReason)

	expiredEntity, err := client.SubscriptionEntitlementPeriod.Get(ctx, expired.ID)
	require.NoError(t, err)
	require.Equal(t, "active", expiredEntity.Status)
	require.Nil(t, expiredEntity.RevokedAt)
}

func TestSubscriptionEntitlementPeriodRepository_RevokeBySourceOnlyRevokesMatchingUnexpiredActivePeriod(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := NewSubscriptionEntitlementPeriodRepository(client)
	user, group, subscription := createSubscriptionEntitlementPeriodFixture(t, ctx, client)
	now := time.Now().UTC()

	activeSource := service.SubscriptionEntitlementSource{
		Type: "redeem_code",
		ID:   fmt.Sprintf("active-source-%d", time.Now().UnixNano()),
	}
	active := newSubscriptionEntitlementPeriodFixture(user.ID, group.ID, subscription.ID, activeSource)
	active.StartsAt = now.Add(-time.Hour)
	active.ExpiresAt = now.Add(24 * time.Hour)
	require.NoError(t, repo.Create(ctx, active))

	expired := newSubscriptionEntitlementPeriodFixture(user.ID, group.ID, subscription.ID, service.SubscriptionEntitlementSource{
		Type: "redeem_code",
		ID:   fmt.Sprintf("expired-source-%d", time.Now().UnixNano()),
	})
	expired.StartsAt = now.Add(-48 * time.Hour)
	expired.ExpiresAt = now.Add(-time.Hour)
	require.NoError(t, repo.Create(ctx, expired))

	require.NoError(t, repo.RevokeBySource(ctx, activeSource, now, "manual_revoke"))

	activeEntity, err := client.SubscriptionEntitlementPeriod.Get(ctx, active.ID)
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionEntitlementPeriodStatusRevoked, activeEntity.Status)
	require.NotNil(t, activeEntity.RevokedAt)
	require.Equal(t, "manual_revoke", activeEntity.RevokedReason)

	expiredEntity, err := client.SubscriptionEntitlementPeriod.Get(ctx, expired.ID)
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionEntitlementPeriodStatusActive, expiredEntity.Status)
	require.Nil(t, expiredEntity.RevokedAt)
}

func TestSubscriptionEntitlementPeriodRepository_UsesOuterTransactionAndRollsBack(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err)
	txCtx := dbent.NewTxContext(ctx, tx)

	client := tx.Client()
	user, group, subscription := createSubscriptionEntitlementPeriodFixture(t, txCtx, client)
	source := service.SubscriptionEntitlementSource{
		Type: "payment_order",
		ID:   fmt.Sprintf("rollback-%d", time.Now().UnixNano()),
	}
	period := newSubscriptionEntitlementPeriodFixture(user.ID, group.ID, subscription.ID, source)

	// 默认 client 故意传入非事务 client，验证 repository 必须从 context 取得外层 transaction。
	repo := NewSubscriptionEntitlementPeriodRepository(integrationEntClient)
	require.NoError(t, repo.Create(txCtx, period))
	require.NoError(t, tx.Rollback())

	_, err = integrationEntClient.SubscriptionEntitlementPeriod.Query().
		Where(
			subscriptionentitlementperiod.SourceTypeEQ(source.Type),
			subscriptionentitlementperiod.SourceIDEQ(source.ID),
		).
		Only(ctx)
	require.True(t, dbent.IsNotFound(err), "outer transaction rollback must remove the period")
}

func TestSubscriptionService_GrantSnapshotsDailyLimitFromOuterTransaction(t *testing.T) {
	ctx := context.Background()
	user, group, _ := createSubscriptionEntitlementPeriodFixture(t, ctx, integrationEntClient)
	initialLimit := 19.0
	_, err := integrationEntClient.Group.UpdateOneID(group.ID).
		SetDailyLimitUsd(initialLimit).
		Save(ctx)
	require.NoError(t, err)

	tx, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err)
	txCtx := dbent.NewTxContext(ctx, tx)
	defer func() { _ = tx.Rollback() }()

	transactionLimit := 23.0
	_, err = tx.Client().Group.UpdateOneID(group.ID).
		SetDailyLimitUsd(transactionLimit).
		Save(txCtx)
	require.NoError(t, err)

	svc := service.ProvideSubscriptionService(
		NewGroupRepository(integrationEntClient, integrationDB),
		NewUserSubscriptionRepository(integrationEntClient),
		nil,
		integrationEntClient,
		nil,
		NewSubscriptionEntitlementPeriodRepository(integrationEntClient),
	)
	result, err := svc.GrantSubscriptionEntitlement(txCtx, &service.AssignSubscriptionInput{
		UserID:       user.ID,
		GroupID:      group.ID,
		ValidityDays: 30,
		EntitlementSource: service.SubscriptionEntitlementSource{
			Type: "payment_order",
			ID:   fmt.Sprintf("snapshot-%d", time.Now().UnixNano()),
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result.Period.DailyLimitUSD)
	require.InDelta(t, transactionLimit, *result.Period.DailyLimitUSD, 0.0000001)
}

func TestSubscriptionService_GrantRollsBackSubscriptionWhenPeriodCreateFails(t *testing.T) {
	ctx := context.Background()
	user, group, existingSubscription := createSubscriptionEntitlementPeriodFixture(t, ctx, integrationEntClient)
	originalExpiresAt := existingSubscription.ExpiresAt
	svc := service.ProvideSubscriptionService(
		NewGroupRepository(integrationEntClient, integrationDB),
		NewUserSubscriptionRepository(integrationEntClient),
		nil,
		integrationEntClient,
		nil,
		failingSubscriptionEntitlementPeriodRepository{err: errors.New("period persistence failed")},
	)
	source := service.SubscriptionEntitlementSource{
		Type: "payment_order",
		ID:   fmt.Sprintf("rollback-service-%d", time.Now().UnixNano()),
	}

	_, err := svc.GrantSubscriptionEntitlement(ctx, &service.AssignSubscriptionInput{
		UserID:            user.ID,
		GroupID:           group.ID,
		ValidityDays:      30,
		EntitlementSource: source,
	})

	require.Error(t, err)
	persistedSubscription, err := integrationEntClient.UserSubscription.Get(ctx, existingSubscription.ID)
	require.NoError(t, err)
	require.Equal(t, originalExpiresAt, persistedSubscription.ExpiresAt)
	periodCount, err := integrationEntClient.SubscriptionEntitlementPeriod.Query().
		Where(
			subscriptionentitlementperiod.SourceTypeEQ(source.Type),
			subscriptionentitlementperiod.SourceIDEQ(source.ID),
			subscriptionentitlementperiod.GroupIDEQ(group.ID),
		).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, periodCount)
}

func TestSubscriptionService_GrantRollsBackNewSubscriptionWhenPeriodCreateFails(t *testing.T) {
	ctx := context.Background()
	user, group := createSubscriptionEntitlementUserAndGroupFixture(t, ctx, integrationEntClient)
	svc := service.ProvideSubscriptionService(
		NewGroupRepository(integrationEntClient, integrationDB),
		NewUserSubscriptionRepository(integrationEntClient),
		nil,
		integrationEntClient,
		nil,
		failingSubscriptionEntitlementPeriodRepository{err: errors.New("period persistence failed")},
	)
	source := service.SubscriptionEntitlementSource{
		Type: "payment_order",
		ID:   fmt.Sprintf("rollback-new-service-%d", time.Now().UnixNano()),
	}

	_, err := svc.GrantSubscriptionEntitlement(ctx, &service.AssignSubscriptionInput{
		UserID:            user.ID,
		GroupID:           group.ID,
		ValidityDays:      30,
		EntitlementSource: source,
	})

	require.Error(t, err)
	subscriptionCount, err := integrationEntClient.UserSubscription.Query().
		Where(
			usersubscription.UserIDEQ(user.ID),
			usersubscription.GroupIDEQ(group.ID),
		).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, subscriptionCount)
}

func createSubscriptionEntitlementPeriodFixture(
	t *testing.T,
	ctx context.Context,
	client *dbent.Client,
) (*dbent.User, *dbent.Group, *dbent.UserSubscription) {
	t.Helper()
	user, group := createSubscriptionEntitlementUserAndGroupFixture(t, ctx, client)
	now := time.Now().UTC()
	subscription, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(group.ID).
		SetStartsAt(now).
		SetExpiresAt(now.AddDate(0, 0, 30)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(now).
		SetNotes("").
		Save(ctx)
	require.NoError(t, err)
	return user, group, subscription
}

func createSubscriptionEntitlementUserAndGroupFixture(
	t *testing.T,
	ctx context.Context,
	client *dbent.Client,
) (*dbent.User, *dbent.Group) {
	t.Helper()
	unique := time.Now().UnixNano()
	user, err := client.User.Create().
		SetEmail(fmt.Sprintf("entitlement-%d@example.com", unique)).
		SetPasswordHash("test-password-hash").
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	group, err := client.Group.Create().
		SetName(fmt.Sprintf("entitlement-group-%d", unique)).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	return user, group
}

func newSubscriptionEntitlementPeriodFixture(
	userID, groupID, subscriptionID int64,
	source service.SubscriptionEntitlementSource,
) *service.SubscriptionEntitlementPeriod {
	limit := 19.0
	now := time.Now().UTC()
	return &service.SubscriptionEntitlementPeriod{
		UserID:         userID,
		SubscriptionID: subscriptionID,
		GroupID:        groupID,
		Source:         source,
		StartsAt:       now,
		ExpiresAt:      now.AddDate(0, 0, 30),
		PeriodDays:     30,
		DailyLimitUSD:  &limit,
		Status:         "active",
	}
}

var _ = errors.Is

type failingSubscriptionEntitlementPeriodRepository struct {
	err error
}

func (r failingSubscriptionEntitlementPeriodRepository) GetBySource(context.Context, service.SubscriptionEntitlementSource) (*service.SubscriptionEntitlementPeriod, error) {
	return nil, service.ErrSubscriptionEntitlementPeriodNotFound
}

func (r failingSubscriptionEntitlementPeriodRepository) Create(context.Context, *service.SubscriptionEntitlementPeriod) error {
	return r.err
}

func (failingSubscriptionEntitlementPeriodRepository) RevokeUnexpiredBySubscription(context.Context, int64, time.Time, string) error {
	return nil
}

func (failingSubscriptionEntitlementPeriodRepository) RevokeBySource(context.Context, service.SubscriptionEntitlementSource, time.Time, string) error {
	return nil
}
