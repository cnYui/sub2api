package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInternalUsageEventNoAccount = errors.New("internal usage event account not found")

type CLIProxyUsageEvent struct {
	Version             int       `json:"version"`
	RequestID           string    `json:"request_id"`
	APIKeyHash          string    `json:"api_key_hash"`
	APIKeyPreview       string    `json:"api_key_preview"`
	Provider            string    `json:"provider"`
	Model               string    `json:"model"`
	Endpoint            string    `json:"endpoint"`
	Source              string    `json:"source"`
	AuthIndex           string    `json:"auth_index"`
	Success             bool      `json:"success"`
	Failed              bool      `json:"failed"`
	InputTokens         int       `json:"input_tokens"`
	OutputTokens        int       `json:"output_tokens"`
	ReasoningTokens     int       `json:"reasoning_tokens"`
	CachedTokens        int       `json:"cached_tokens"`
	CacheHitInputTokens int       `json:"cache_hit_input_tokens"`
	CacheMissTokens     int       `json:"cache_miss_input_tokens"`
	TotalTokens         int       `json:"total_tokens"`
	LatencyMS           int       `json:"latency_ms"`
	RequestedAt         time.Time `json:"requested_at"`
}

type InternalUsageEventResult struct {
	RequestID string
	Created   bool
	FactID    int64
	Skipped   bool
}

type InternalUsageEventService struct {
	apiKeyRepo     APIKeyRepository
	accountRepo    AccountRepository
	usageFactRepo  UsageFactRepository
	billingService *BillingService
}

func NewInternalUsageEventService(apiKeyRepo APIKeyRepository, accountRepo AccountRepository, usageFactRepo UsageFactRepository, billingService *BillingService) *InternalUsageEventService {
	return &InternalUsageEventService{
		apiKeyRepo:     apiKeyRepo,
		accountRepo:    accountRepo,
		usageFactRepo:  usageFactRepo,
		billingService: billingService,
	}
}

func (s *InternalUsageEventService) RecordCLIProxyUsageEvent(ctx context.Context, event CLIProxyUsageEvent, rawBody []byte) (*InternalUsageEventResult, error) {
	if s == nil || s.apiKeyRepo == nil || s.accountRepo == nil || s.usageFactRepo == nil || s.billingService == nil {
		return nil, errors.New("internal usage event service dependencies are incomplete")
	}
	event.RequestID = strings.TrimSpace(event.RequestID)
	event.APIKeyHash = strings.ToLower(strings.TrimSpace(event.APIKeyHash))
	event.Model = strings.TrimSpace(event.Model)
	event.Endpoint = strings.TrimSpace(event.Endpoint)
	if event.RequestID == "" {
		return nil, ErrUsageBillingRequestIDRequired
	}
	if event.APIKeyHash == "" || event.Model == "" {
		return nil, ErrInvalidInput
	}
	if event.Failed || !event.Success {
		return &InternalUsageEventResult{RequestID: event.RequestID, Skipped: true}, nil
	}
	requestID := "cliproxy:" + event.RequestID
	if existing, err := s.usageFactRepo.FindByRequestID(ctx, requestID); err == nil && len(existing) > 0 {
		return &InternalUsageEventResult{RequestID: requestID, FactID: existing[0].ID, Created: false}, nil
	} else if err != nil {
		return nil, err
	}

	apiKey, err := s.apiKeyRepo.GetActiveBySHA256Hash(ctx, event.APIKeyHash)
	if err != nil {
		return nil, err
	}
	account, err := s.resolveCLIProxyAccount(ctx)
	if err != nil {
		return nil, err
	}

	cacheReadTokens := event.CacheHitInputTokens
	if cacheReadTokens <= 0 {
		cacheReadTokens = event.CachedTokens
	}
	tokens := UsageTokens{
		InputTokens:     event.InputTokens,
		OutputTokens:    event.OutputTokens,
		CacheReadTokens: cacheReadTokens,
	}
	rateMultiplier := 1.0
	if apiKey.Group != nil && apiKey.Group.RateMultiplier > 0 {
		rateMultiplier = apiKey.Group.RateMultiplier
	}
	cost, err := s.billingService.CalculateCost(event.Model, tokens, rateMultiplier)
	if err != nil {
		return nil, err
	}
	completedAt := event.RequestedAt
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	if event.LatencyMS > 0 {
		completedAt = completedAt.Add(time.Duration(event.LatencyMS) * time.Millisecond)
	}
	payloadHash := HashUsageRequestPayload(rawBody)
	accountRateMultiplier := account.BillingRateMultiplier()
	billingMode := string(BillingModeToken)
	upstreamEndpoint := optionalString(event.Endpoint)
	usageLog := UsageLog{
		UserID:                apiKey.UserID,
		APIKeyID:              apiKey.ID,
		AccountID:             account.ID,
		RequestID:             requestID,
		Model:                 event.Model,
		RequestedModel:        event.Model,
		UpstreamModel:         optionalString(event.Model),
		UpstreamEndpoint:      upstreamEndpoint,
		GroupID:               apiKey.GroupID,
		InputTokens:           event.InputTokens,
		OutputTokens:          event.OutputTokens,
		CacheReadTokens:       cacheReadTokens,
		InputCost:             cost.InputCost,
		OutputCost:            cost.OutputCost,
		CacheReadCost:         cost.CacheReadCost,
		TotalCost:             cost.TotalCost,
		ActualCost:            cost.ActualCost,
		RateMultiplier:        rateMultiplier,
		AccountRateMultiplier: &accountRateMultiplier,
		BillingType:           BillingTypeBalance,
		RequestType:           RequestTypeSync,
		DurationMs:            optionalInt(event.LatencyMS),
		BillingMode:           &billingMode,
		CreatedAt:             completedAt,
	}
	cmd := UsageBillingCommand{
		RequestID:           requestID,
		APIKeyID:            apiKey.ID,
		RequestPayloadHash:  payloadHash,
		CompletedAt:         completedAt,
		UserID:              apiKey.UserID,
		AccountID:           account.ID,
		AccountType:         account.Type,
		Model:               event.Model,
		BillingType:         BillingTypeBalance,
		InputTokens:         event.InputTokens,
		OutputTokens:        event.OutputTokens,
		CacheReadTokens:     cacheReadTokens,
		BalanceCost:         cost.ActualCost,
		APIKeyQuotaCost:     cost.ActualCost,
		APIKeyRateLimitCost: cost.ActualCost,
		AccountQuotaCost:    cost.TotalCost * accountRateMultiplier,
	}
	fact, err := NewUsageFact(UsageFactPayload{
		BillingCommand: cmd,
		UsageLog:       usageLog,
		Effects: UsageSettlementEffectsPayload{
			UserID:                apiKey.UserID,
			APIKeyID:              apiKey.ID,
			AccountID:             account.ID,
			GroupID:               apiKey.GroupID,
			Platform:              PlatformOpenAI,
			ActualCost:            cost.ActualCost,
			TotalCost:             cost.TotalCost,
			AccountRateMultiplier: accountRateMultiplier,
		},
		OpenAIBilling: &OpenAIUsageBillingSnapshot{
			BillingIncomplete: false,
		},
	})
	if err != nil {
		return nil, err
	}
	createdFact, created, err := s.usageFactRepo.CreatePending(ctx, fact)
	if err != nil {
		return nil, err
	}
	return &InternalUsageEventResult{RequestID: requestID, Created: created, FactID: createdFact.ID}, nil
}

func (s *InternalUsageEventService) resolveCLIProxyAccount(ctx context.Context) (*Account, error) {
	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		account := accounts[i]
		if !account.IsActive() {
			continue
		}
		baseURL := strings.ToLower(strings.TrimSpace(fmt.Sprint(account.Credentials["base_url"])))
		if strings.Contains(baseURL, "host.docker.internal:8317") || strings.Contains(baseURL, "127.0.0.1:8317") || strings.Contains(strings.ToLower(account.Name), "cliproxy") {
			return &account, nil
		}
	}
	for i := range accounts {
		account := accounts[i]
		if account.IsActive() && account.Type == AccountTypeAPIKey {
			return &account, nil
		}
	}
	return nil, ErrInternalUsageEventNoAccount
}

func HashAPIKeySHA256(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func optionalInt(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}
