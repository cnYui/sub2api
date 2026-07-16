package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type UsageSettlementEffects interface {
	Apply(ctx context.Context, payload UsageSettlementEffectsPayload, result *UsageBillingApplyResult)
}

type UsageFactSettlementService struct {
	factRepo     UsageFactRepository
	billingRepo  UsageBillingRepository
	usageLogRepo UsageLogRepository
	effects      UsageSettlementEffects
}

func NewUsageFactSettlementService(
	factRepo UsageFactRepository,
	billingRepo UsageBillingRepository,
	usageLogRepo UsageLogRepository,
	effects UsageSettlementEffects,
) *UsageFactSettlementService {
	return &UsageFactSettlementService{
		factRepo:     factRepo,
		billingRepo:  billingRepo,
		usageLogRepo: usageLogRepo,
		effects:      effects,
	}
}

func (s *UsageFactSettlementService) Settle(ctx context.Context, fact UsageFact) error {
	if s == nil || s.factRepo == nil || s.billingRepo == nil || s.usageLogRepo == nil {
		return errors.New("usage fact settlement dependencies are incomplete")
	}
	payload, err := DecodeUsageFactPayload(fact.PayloadVersion, fact.Payload)
	if err != nil {
		return s.factRepo.MarkDebt(ctx, fact.ID, "invalid payload: "+err.Error(), time.Now())
	}

	result, billingErr := s.billingRepo.Apply(ctx, &payload.BillingCommand)
	if billingErr != nil && !errors.Is(billingErr, ErrInsufficientBalance) {
		return billingErr
	}
	if _, err := s.usageLogRepo.Create(ctx, &payload.UsageLog); err != nil {
		return err
	}

	now := time.Now()
	if errors.Is(billingErr, ErrInsufficientBalance) {
		return s.factRepo.MarkDebt(ctx, fact.ID, fmt.Sprintf("%v", billingErr), now)
	}
	if err := s.factRepo.MarkSettled(ctx, fact.ID, now); err != nil {
		return err
	}
	if s.effects != nil {
		s.effects.Apply(ctx, payload.Effects, result)
	}
	return nil
}
