package service

import (
	"context"
	"log/slog"
)

type usageSettlementFinalizeFunc func(
	ctx context.Context,
	p *usageSettlementParams,
	deps *billingDeps,
	result *UsageBillingApplyResult,
)

type UsageSettlementEffectsHandler struct {
	userRepo        UserRepository
	apiKeyRepo      APIKeyRepository
	accountRepo     AccountRepository
	authInvalidator APIKeyAuthCacheInvalidator
	deps            *billingDeps
	finalize        usageSettlementFinalizeFunc
}

func (s *UsageSettlementEffectsHandler) Apply(
	ctx context.Context,
	payload UsageSettlementEffectsPayload,
	result *UsageBillingApplyResult,
) {
	if s == nil {
		return
	}
	if result == nil || !result.Applied {
		if s.deps != nil && s.deps.deferredService != nil && payload.AccountID > 0 {
			s.deps.deferredService.ScheduleLastUsedUpdate(payload.AccountID)
		}
		return
	}
	user, err := s.userRepo.GetByID(ctx, payload.UserID)
	if err != nil {
		slog.Error("load usage settlement user failed", "user_id", payload.UserID, "error", err)
		return
	}
	apiKey, err := s.apiKeyRepo.GetByID(ctx, payload.APIKeyID)
	if err != nil {
		slog.Error("load usage settlement api key failed", "api_key_id", payload.APIKeyID, "error", err)
		return
	}
	account, err := s.accountRepo.GetByID(ctx, payload.AccountID)
	if err != nil {
		slog.Error("load usage settlement account failed", "account_id", payload.AccountID, "error", err)
		return
	}
	if result.APIKeyQuotaExhausted && s.authInvalidator != nil && apiKey.Key != "" {
		s.authInvalidator.InvalidateAuthCacheByKey(ctx, apiKey.Key)
	}
	finalize := s.finalize
	if finalize == nil {
		finalize = applyUsageSettlementEffects
	}
	finalize(ctx, &usageSettlementParams{
		Cost: &CostBreakdown{
			ActualCost: payload.ActualCost,
			TotalCost:  payload.TotalCost,
		},
		User:                  user,
		APIKey:                apiKey,
		Account:               account,
		IsSubscriptionBill:    payload.IsSubscription,
		AccountRateMultiplier: payload.AccountRateMultiplier,
		Platform:              payload.Platform,
		UseTrafficPack:        payload.IsTrafficCredit,
	}, s.deps, result)
}
