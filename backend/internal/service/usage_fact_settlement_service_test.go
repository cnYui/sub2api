//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type usageFactSettlementFactRepoStub struct {
	UsageFactRepository
	markSettledID int64
	markDebtID    int64
	markDebtError string
}

func (s *usageFactSettlementFactRepoStub) MarkSettled(ctx context.Context, id int64, settledAt time.Time) error {
	s.markSettledID = id
	return nil
}

func (s *usageFactSettlementFactRepoStub) MarkDebt(ctx context.Context, id int64, reason string, settledAt time.Time) error {
	s.markDebtID = id
	s.markDebtError = reason
	return nil
}

type usageFactSettlementBillingRepoStub struct {
	result *UsageBillingApplyResult
	err    error
	calls  int
}

func (s *usageFactSettlementBillingRepoStub) Apply(ctx context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	s.calls++
	return s.result, s.err
}

type usageFactSettlementLogRepoStub struct {
	UsageLogRepository
	created *UsageLog
	err     error
}

func (s *usageFactSettlementLogRepoStub) Create(ctx context.Context, log *UsageLog) (bool, error) {
	s.created = log
	return s.err == nil, s.err
}

type usageFactSettlementEffectsStub struct {
	calls   int
	payload UsageSettlementEffectsPayload
	result  *UsageBillingApplyResult
}

func (s *usageFactSettlementEffectsStub) Apply(ctx context.Context, payload UsageSettlementEffectsPayload, result *UsageBillingApplyResult) {
	s.calls++
	s.payload = payload
	s.result = result
}

func TestUsageFactSettlementService_AppliesBillingWritesLogMarksSettledAndRunsEffects(t *testing.T) {
	factRepo := &usageFactSettlementFactRepoStub{}
	result := &UsageBillingApplyResult{Applied: true}
	billingRepo := &usageFactSettlementBillingRepoStub{result: result}
	logRepo := &usageFactSettlementLogRepoStub{}
	effects := &usageFactSettlementEffectsStub{}
	svc := NewUsageFactSettlementService(factRepo, billingRepo, logRepo, effects)

	err := svc.Settle(context.Background(), usageFactSettlementTestFact(t, 0.25))

	require.NoError(t, err)
	require.Equal(t, 1, billingRepo.calls)
	require.NotNil(t, logRepo.created)
	require.Equal(t, "req-1", logRepo.created.RequestID)
	require.Equal(t, int64(1), factRepo.markSettledID)
	require.Zero(t, factRepo.markDebtID)
	require.Equal(t, 1, effects.calls)
	require.Equal(t, int64(7), effects.payload.UserID)
	require.Same(t, result, effects.result)
}

func TestUsageFactSettlementService_MarksDebtAndWritesUsageLogOnInsufficientBalance(t *testing.T) {
	factRepo := &usageFactSettlementFactRepoStub{}
	billingRepo := &usageFactSettlementBillingRepoStub{err: ErrInsufficientBalance}
	logRepo := &usageFactSettlementLogRepoStub{}
	svc := NewUsageFactSettlementService(factRepo, billingRepo, logRepo, nil)

	err := svc.Settle(context.Background(), usageFactSettlementTestFact(t, 0.25))

	require.NoError(t, err)
	require.Equal(t, int64(1), factRepo.markDebtID)
	require.ErrorContains(t, errors.New(factRepo.markDebtError), ErrInsufficientBalance.Error())
	require.NotNil(t, logRepo.created)
	require.Equal(t, "req-1", logRepo.created.RequestID)
	require.Zero(t, factRepo.markSettledID)
}

func TestUsageFactSettlementService_MarksDebtWhenReservationSettlementReportsDebt(t *testing.T) {
	factRepo := &usageFactSettlementFactRepoStub{}
	billingRepo := &usageFactSettlementBillingRepoStub{result: &UsageBillingApplyResult{Applied: true, TrafficCreditDebtUSD: 0.125}}
	logRepo := &usageFactSettlementLogRepoStub{}
	svc := NewUsageFactSettlementService(factRepo, billingRepo, logRepo, nil)

	err := svc.Settle(context.Background(), usageFactSettlementTestFact(t, 0.25))

	require.NoError(t, err)
	require.Equal(t, int64(1), factRepo.markDebtID)
	require.Contains(t, factRepo.markDebtError, "traffic credit debt")
	require.NotNil(t, logRepo.created)
	require.Zero(t, factRepo.markSettledID)
}

func TestUsageFactSettlementService_RetriesTransientBillingFailure(t *testing.T) {
	transient := errors.New("database unavailable")
	factRepo := &usageFactSettlementFactRepoStub{}
	billingRepo := &usageFactSettlementBillingRepoStub{err: transient}
	logRepo := &usageFactSettlementLogRepoStub{}
	svc := NewUsageFactSettlementService(factRepo, billingRepo, logRepo, nil)

	err := svc.Settle(context.Background(), usageFactSettlementTestFact(t, 0))

	require.ErrorIs(t, err, transient)
	require.Nil(t, logRepo.created)
	require.Zero(t, factRepo.markDebtID)
	require.Zero(t, factRepo.markSettledID)
}

func usageFactSettlementTestFact(t *testing.T, trafficPackCost float64) UsageFact {
	t.Helper()
	payload := UsageFactPayload{
		BillingCommand: UsageBillingCommand{
			RequestID:       "req-1",
			APIKeyID:        9,
			UserID:          7,
			AccountID:       5,
			TrafficPackCost: trafficPackCost,
		},
		UsageLog: UsageLog{
			RequestID:  "req-1",
			APIKeyID:   9,
			UserID:     7,
			AccountID:  5,
			ActualCost: trafficPackCost,
		},
		Effects: UsageSettlementEffectsPayload{
			UserID:     7,
			APIKeyID:   9,
			AccountID:  5,
			ActualCost: trafficPackCost,
		},
	}
	raw, err := EncodeUsageFactPayload(payload)
	require.NoError(t, err)
	return UsageFact{
		ID:             1,
		PayloadVersion: UsageFactPayloadVersion1,
		Payload:        raw,
	}
}
