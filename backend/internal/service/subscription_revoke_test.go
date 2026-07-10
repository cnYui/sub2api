package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type subscriptionRevokeRepoStub struct {
	userSubRepoNoop

	sub       *UserSubscription
	callOrder []string
	statusID  int64
	status    string
	deleteID  int64
}

func (s *subscriptionRevokeRepoStub) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	s.callOrder = append(s.callOrder, "get")
	if s.sub == nil || s.sub.ID != id {
		return nil, ErrSubscriptionNotFound
	}
	cp := *s.sub
	return &cp, nil
}

func (s *subscriptionRevokeRepoStub) UpdateStatus(_ context.Context, id int64, status string) error {
	s.callOrder = append(s.callOrder, "status")
	s.statusID = id
	s.status = status
	return nil
}

func (s *subscriptionRevokeRepoStub) Delete(_ context.Context, id int64) error {
	s.callOrder = append(s.callOrder, "delete")
	s.deleteID = id
	return nil
}

func TestRevokeSubscriptionMarksExpiredBeforeSoftDelete(t *testing.T) {
	repo := &subscriptionRevokeRepoStub{
		sub: &UserSubscription{
			ID:        88,
			UserID:    69,
			GroupID:   8,
			Status:    SubscriptionStatusActive,
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)

	err := svc.RevokeSubscription(context.Background(), 88)

	require.NoError(t, err)
	require.Equal(t, []string{"get", "status", "delete"}, repo.callOrder)
	require.Equal(t, int64(88), repo.statusID)
	require.Equal(t, SubscriptionStatusExpired, repo.status)
	require.Equal(t, int64(88), repo.deleteID)
}
