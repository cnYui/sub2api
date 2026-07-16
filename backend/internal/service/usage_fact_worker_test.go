//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type usageFactWorkerRepoStub struct {
	UsageFactRepository
	claimed         []UsageFact
	claimLeaseUntil time.Time
	retryID         int64
	retryReason     string
	retryAt         time.Time
}

func (s *usageFactWorkerRepoStub) ClaimPending(ctx context.Context, limit int, now, leaseUntil time.Time) ([]UsageFact, error) {
	s.claimLeaseUntil = leaseUntil
	return s.claimed, nil
}

func (s *usageFactWorkerRepoStub) MarkRetry(ctx context.Context, id int64, reason string, nextAttemptAt time.Time) error {
	s.retryID = id
	s.retryReason = reason
	s.retryAt = nextAttemptAt
	return nil
}

type usageFactWorkerSettlementStub struct {
	err error
}

func (s *usageFactWorkerSettlementStub) Settle(ctx context.Context, fact UsageFact) error {
	return s.err
}

func TestUsageFactWorker_RetriesWithBackoff(t *testing.T) {
	repo := &usageFactWorkerRepoStub{claimed: []UsageFact{{ID: 1, AttemptCount: 1}}}
	settlement := &usageFactWorkerSettlementStub{err: errors.New("temporary")}
	worker := NewUsageFactWorker(repo, settlement, UsageFactWorkerConfig{
		BatchSize:    10,
		PollInterval: time.Millisecond,
		TaskTimeout:  time.Second,
	})
	startedAt := time.Now()

	worker.runOnce(context.Background())

	require.Equal(t, int64(1), repo.retryID)
	require.Contains(t, repo.retryReason, "temporary")
	require.True(t, repo.retryAt.After(startedAt))
	require.True(t, repo.claimLeaseUntil.After(startedAt.Add(time.Second)))
}
