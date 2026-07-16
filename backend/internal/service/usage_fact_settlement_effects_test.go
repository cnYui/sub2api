//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type usageSettlementEffectsUserRepoStub struct {
	UserRepository
	user *User
}

func (s *usageSettlementEffectsUserRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	return s.user, nil
}

type usageSettlementEffectsAPIKeyRepoStub struct {
	APIKeyRepository
	apiKey *APIKey
}

func (s *usageSettlementEffectsAPIKeyRepoStub) GetByID(ctx context.Context, id int64) (*APIKey, error) {
	return s.apiKey, nil
}

type usageSettlementEffectsAccountRepoStub struct {
	AccountRepository
	account *Account
}

func (s *usageSettlementEffectsAccountRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	return s.account, nil
}

type usageSettlementEffectsInvalidatorStub struct {
	key string
}

func (s *usageSettlementEffectsInvalidatorStub) InvalidateAuthCacheByKey(ctx context.Context, key string) {
	s.key = key
}

func (s *usageSettlementEffectsInvalidatorStub) InvalidateAuthCacheByUserID(ctx context.Context, userID int64) {
}

func (s *usageSettlementEffectsInvalidatorStub) InvalidateAuthCacheByGroupID(ctx context.Context, groupID int64) {
}

func TestOpenAIUsageSettlementEffects_RebuildsPostBillingContext(t *testing.T) {
	groupID := int64(11)
	user := &User{ID: 7}
	apiKey := &APIKey{ID: 9, Key: "sk-test", GroupID: &groupID}
	account := &Account{ID: 5, Type: AccountTypeAPIKey}
	invalidator := &usageSettlementEffectsInvalidatorStub{}
	effects := &OpenAIUsageSettlementEffects{
		userRepo:        &usageSettlementEffectsUserRepoStub{user: user},
		apiKeyRepo:      &usageSettlementEffectsAPIKeyRepoStub{apiKey: apiKey},
		accountRepo:     &usageSettlementEffectsAccountRepoStub{account: account},
		authInvalidator: invalidator,
		deps:            &billingDeps{},
	}
	var gotParams *postUsageBillingParams
	effects.finalize = func(ctx context.Context, p *postUsageBillingParams, deps *billingDeps, result *UsageBillingApplyResult) {
		gotParams = p
	}

	effects.Apply(context.Background(), UsageSettlementEffectsPayload{
		UserID:                user.ID,
		APIKeyID:              apiKey.ID,
		AccountID:             account.ID,
		GroupID:               &groupID,
		Platform:              PlatformOpenAI,
		ActualCost:            0.25,
		TotalCost:             0.5,
		AccountRateMultiplier: 1.2,
	}, &UsageBillingApplyResult{Applied: true, APIKeyQuotaExhausted: true})

	require.NotNil(t, gotParams)
	require.Same(t, user, gotParams.User)
	require.Same(t, apiKey, gotParams.APIKey)
	require.Same(t, account, gotParams.Account)
	require.InDelta(t, 0.25, gotParams.Cost.ActualCost, 1e-12)
	require.InDelta(t, 0.5, gotParams.Cost.TotalCost, 1e-12)
	require.InDelta(t, 1.2, gotParams.AccountRateMultiplier, 1e-12)
	require.Equal(t, "sk-test", invalidator.key)
}
