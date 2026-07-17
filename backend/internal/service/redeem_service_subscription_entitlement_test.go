//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type redeemSubscriptionRepoStub struct {
	redeemRepoStub

	code *RedeemCode
}

func (s *redeemSubscriptionRepoStub) GetByCode(_ context.Context, code string) (*RedeemCode, error) {
	if s.code == nil || s.code.Code != code {
		return nil, ErrRedeemCodeNotFound
	}
	cp := *s.code
	return &cp, nil
}

func (s *redeemSubscriptionRepoStub) Use(_ context.Context, id, userID int64) error {
	if s.code == nil || s.code.ID != id || !s.code.CanUse() {
		return ErrRedeemCodeUsed
	}
	s.code.Status = StatusUsed
	s.code.UsedBy = &userID
	return nil
}

func (s *redeemSubscriptionRepoStub) GetByID(_ context.Context, id int64) (*RedeemCode, error) {
	if s.code == nil || s.code.ID != id {
		return nil, ErrRedeemCodeNotFound
	}
	cp := *s.code
	return &cp, nil
}

func TestRedeemSubscriptionCreatesEntitlementPeriodWithRedeemCodeSource(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, err := client.User.Create().
		SetEmail("redeem-subscription@example.com").
		SetPasswordHash("hash").
		SetStatus(StatusActive).
		Save(ctx)
	require.NoError(t, err)
	groupID := int64(7)
	code := &RedeemCode{
		ID:           88,
		Code:         "SUB-88",
		Type:         RedeemTypeSubscription,
		Status:       StatusUnused,
		ValidityDays: 30,
		GroupID:      &groupID,
	}
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	subscriptionSvc := NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: groupID, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, newSubscriptionUserSubRepoStub(), nil, nil, nil)
	subscriptionSvc.entitlementPeriodRepo = entitlementRepo
	svc := NewRedeemService(
		&redeemSubscriptionRepoStub{code: code},
		&userRepoStub{user: &User{ID: user.ID, Status: StatusActive}},
		subscriptionSvc,
		nil,
		nil,
		client,
		nil,
		nil,
	)

	_, err = svc.Redeem(ctx, user.ID, "SUB-88")

	require.NoError(t, err)
	period, err := entitlementRepo.GetBySource(ctx, SubscriptionEntitlementSource{
		Type: "redeem_code",
		ID:   strconv.FormatInt(code.ID, 10),
	})
	require.NoError(t, err)
	require.Equal(t, user.ID, period.UserID)
	require.Equal(t, groupID, period.GroupID)
}

func TestRedeemNegativeSubscriptionRevokesUnexpiredEntitlementPeriods(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2030, 7, 16, 9, 0, 0, 0, time.UTC)
	client := newPaymentConfigServiceTestClient(t)
	user, err := client.User.Create().
		SetEmail("redeem-negative-subscription@example.com").
		SetPasswordHash("hash").
		SetStatus(StatusActive).
		Save(ctx)
	require.NoError(t, err)
	groupID := int64(7)
	code := &RedeemCode{
		ID:           89,
		Code:         "SUB-NEG-89",
		Type:         RedeemTypeSubscription,
		Status:       StatusUnused,
		ValidityDays: -10,
		GroupID:      &groupID,
	}
	userSubRepo := newSubscriptionUserSubRepoStub()
	userSubRepo.seed(&UserSubscription{
		ID:        99,
		UserID:    user.ID,
		GroupID:   groupID,
		Status:    SubscriptionStatusActive,
		StartsAt:  now.AddDate(0, 0, -10),
		ExpiresAt: now.AddDate(0, 0, 20),
	})
	entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
	source := SubscriptionEntitlementSource{Type: "payment_order", ID: "order-99"}
	entitlementRepo.periods[subscriptionEntitlementSourceKey(source)] = &SubscriptionEntitlementPeriod{
		ID:             1,
		UserID:         user.ID,
		SubscriptionID: 99,
		GroupID:        groupID,
		Source:         source,
		StartsAt:       now.AddDate(0, 0, -10),
		ExpiresAt:      now.AddDate(0, 0, 20),
		PeriodDays:     30,
		Status:         SubscriptionEntitlementPeriodStatusActive,
	}
	subscriptionSvc := NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: groupID, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, userSubRepo, nil, nil, nil)
	subscriptionSvc.entitlementPeriodRepo = entitlementRepo
	subscriptionSvc.now = func() time.Time { return now }
	svc := NewRedeemService(
		&redeemSubscriptionRepoStub{code: code},
		&userRepoStub{user: &User{ID: user.ID, Status: StatusActive}},
		subscriptionSvc,
		nil,
		nil,
		client,
		nil,
		nil,
	)

	_, err = svc.Redeem(ctx, user.ID, "SUB-NEG-89")

	require.NoError(t, err)
	require.Equal(t, []int64{99}, entitlementRepo.revokeSubscriptionCalls)
	require.Equal(t, "revoked", entitlementRepo.periods[subscriptionEntitlementSourceKey(source)].Status)
	require.Equal(t, "redeem_negative_adjustment", entitlementRepo.periods[subscriptionEntitlementSourceKey(source)].RevokedReason)
}
