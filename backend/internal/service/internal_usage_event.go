package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

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
	_ = s
	_ = ctx
	_ = rawBody
	event.RequestID = strings.TrimSpace(event.RequestID)
	if event.RequestID == "" {
		return nil, ErrUsageBillingRequestIDRequired
	}
	// CLIProxy 未提供与 Sub2API 预授权一致的关联键，不能独立创建计费事实。
	return &InternalUsageEventResult{RequestID: event.RequestID, Skipped: true}, nil
}

func HashAPIKeySHA256(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
