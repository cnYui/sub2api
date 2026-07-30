//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUsageFactRepository_CreatePendingIsIdempotent(t *testing.T) {
	resetUsageFacts(t)
	repo := NewUsageFactRepository(integrationDB)
	fact := newUsageFactRepositoryTestFact("same-fingerprint")

	created, inserted, err := repo.CreatePending(context.Background(), fact)
	require.NoError(t, err)
	require.True(t, inserted)
	require.NotZero(t, created.ID)

	again, inserted, err := repo.CreatePending(context.Background(), fact)
	require.NoError(t, err)
	require.False(t, inserted)
	require.Equal(t, created.ID, again.ID)
	require.Equal(t, created.RequestFingerprint, again.RequestFingerprint)
}

func TestUsageFactRepository_PersistsAndClaimsAuthorizationID(t *testing.T) {
	resetUsageFacts(t)
	ctx := context.Background()
	repo := NewUsageFactRepository(integrationDB)
	fact := newUsageFactRepositoryTestFact("authorization-id")
	authorizationID := int64(71)
	fact.AuthorizationID = &authorizationID

	created, inserted, err := repo.CreatePending(ctx, fact)
	require.NoError(t, err)
	require.True(t, inserted)
	require.Equal(t, &authorizationID, created.AuthorizationID)

	claimAt := time.Now().Add(time.Minute)
	claimed, err := repo.ClaimPending(ctx, 1, claimAt, claimAt.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, &authorizationID, claimed[0].AuthorizationID)

	found, err := repo.FindByRequestID(ctx, created.RequestID)
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, &authorizationID, found[0].AuthorizationID)
}

func TestUsageFactRepository_CreatePendingRejectsFingerprintConflict(t *testing.T) {
	resetUsageFacts(t)
	repo := NewUsageFactRepository(integrationDB)
	fact := newUsageFactRepositoryTestFact("fingerprint-a")
	_, _, err := repo.CreatePending(context.Background(), fact)
	require.NoError(t, err)

	conflict := *fact
	conflict.RequestFingerprint = "fingerprint-b"
	_, _, err = repo.CreatePending(context.Background(), &conflict)
	require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
}

func TestUsageFactRepository_ClaimPendingSkipsLockedRows(t *testing.T) {
	resetUsageFacts(t)
	ctx := context.Background()
	repo := NewUsageFactRepository(integrationDB)
	first, _, err := repo.CreatePending(ctx, newUsageFactRepositoryTestFact("first"))
	require.NoError(t, err)
	second, _, err := repo.CreatePending(ctx, newUsageFactRepositoryTestFact("second"))
	require.NoError(t, err)

	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	var lockedID int64
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT id FROM usage_facts WHERE id = $1 FOR UPDATE", first.ID).Scan(&lockedID))

	claimAt := time.Now().Add(time.Minute)
	claimed, err := repo.ClaimPending(ctx, 1, claimAt, claimAt.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, second.ID, claimed[0].ID)
	require.Equal(t, service.UsageFactStatusSettling, claimed[0].BillingStatus)
	require.Equal(t, 1, claimed[0].AttemptCount)
}

func TestUsageFactRepository_ClaimPendingLeasesSettlingRows(t *testing.T) {
	resetUsageFacts(t)
	ctx := context.Background()
	repo := NewUsageFactRepository(integrationDB)
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, _, err := repo.CreatePending(ctx, newUsageFactRepositoryTestFact("lease"))
	require.NoError(t, err)

	claimAt := now.Add(time.Minute)
	leaseUntil := claimAt.Add(30 * time.Second)
	claimed, err := repo.ClaimPending(ctx, 1, claimAt, leaseUntil)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	secondClaimAt := now.Add(time.Minute + time.Second)
	claimedAgain, err := repo.ClaimPending(ctx, 1, secondClaimAt, secondClaimAt.Add(30*time.Second))
	require.NoError(t, err)
	require.Empty(t, claimedAgain)

	reclaimAt := now.Add(2 * time.Minute)
	reclaimed, err := repo.ClaimPending(ctx, 1, reclaimAt, reclaimAt.Add(30*time.Second))
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	require.Equal(t, claimed[0].ID, reclaimed[0].ID)
	require.Equal(t, 2, reclaimed[0].AttemptCount)
}

func TestUsageFactRepository_MarkDebtPreservesPayload(t *testing.T) {
	resetUsageFacts(t)
	ctx := context.Background()
	repo := NewUsageFactRepository(integrationDB)
	fact, _, err := repo.CreatePending(ctx, newUsageFactRepositoryTestFact("debt"))
	require.NoError(t, err)
	originalPayload := append(json.RawMessage(nil), fact.Payload...)

	settledAt := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, repo.MarkDebt(ctx, fact.ID, "insufficient balance", settledAt))

	var payload []byte
	var status string
	var lastError string
	var gotSettledAt time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT payload, billing_status, last_error, settled_at
		FROM usage_facts
		WHERE id = $1
	`, fact.ID).Scan(&payload, &status, &lastError, &gotSettledAt))
	require.JSONEq(t, string(originalPayload), string(payload))
	require.Equal(t, service.UsageFactStatusDebt, status)
	require.Equal(t, "insufficient balance", lastError)
	require.WithinDuration(t, settledAt, gotSettledAt, time.Microsecond)
}

func newUsageFactRepositoryTestFact(fingerprint string) *service.UsageFact {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &service.UsageFact{
		RequestID:          uuid.NewString(),
		APIKeyID:           9,
		UserID:             7,
		AccountID:          5,
		RequestFingerprint: fingerprint,
		PayloadVersion:     service.UsageFactPayloadVersion1,
		Payload:            json.RawMessage(`{"marker":"` + fingerprint + `"}`),
		BillingStatus:      service.UsageFactStatusPending,
		NextAttemptAt:      now,
		CompletedAt:        now,
	}
}

func resetUsageFacts(t *testing.T) {
	t.Helper()
	_, err := integrationDB.ExecContext(context.Background(), "TRUNCATE usage_facts RESTART IDENTITY")
	require.NoError(t, err)
}
