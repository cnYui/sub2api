package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func newGatewayRecordUsageServiceForTest(usageRepo UsageLogRepository, userRepo UserRepository, subRepo UserSubscriptionRepository) *GatewayService {
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1.1
	return NewGatewayService(
		nil,
		nil,
		usageRepo,
		userRepo,
		subRepo,
		nil,
		nil,
		cfg,
		nil,
		nil,
		NewBillingService(cfg, nil),
		nil,
		&BillingCacheService{},
		nil,
		nil,
		&DeferredService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil, // userPlatformQuotaRepo
		&openAIRecordUsageFactRepoStub{},
	)
}

func decodeUsageFactPayloadForTest(t *testing.T, fact *UsageFact) UsageFactPayload {
	t.Helper()
	require.NotNil(t, fact)
	payload, err := DecodeUsageFactPayload(fact.PayloadVersion, fact.Payload)
	require.NoError(t, err)
	return payload
}

func buildGatewayUsageFactPayloadForTest(t *testing.T, svc *GatewayService, ctx context.Context, input *RecordUsageInput) UsageFactPayload {
	t.Helper()
	fact, err := svc.BuildUsageFact(ctx, input)
	require.NoError(t, err)
	return decodeUsageFactPayloadForTest(t, fact)
}

func persistGatewayUsageFactPayloadForTest(t *testing.T, svc *GatewayService, ctx context.Context, input *RecordUsageInput) UsageFactPayload {
	t.Helper()
	fact, err := svc.PersistUsageFact(ctx, input)
	require.NoError(t, err)
	return decodeUsageFactPayloadForTest(t, fact)
}

func TestGatewayServicePersistUsageFact_DoesNotWriteUsageLogOrBillInline(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: false, err: context.DeadlineExceeded}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	quotaSvc := &openAIRecordUsageAPIKeyQuotaStub{}
	svc := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, subRepo)

	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()

	payload := persistGatewayUsageFactPayloadForTest(t, svc, reqCtx, &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "gateway_detached_ctx",
			Usage: ClaudeUsage{
				InputTokens:  10,
				OutputTokens: 6,
			},
			Model:    "claude-sonnet-4",
			Duration: time.Second,
		},
		APIKey: &APIKey{
			ID:    501,
			Quota: 100,
		},
		User:          &User{ID: 601},
		Account:       &Account{ID: 701},
		APIKeyService: quotaSvc,
	})

	repo, ok := svc.usageFactRepo.(*openAIRecordUsageFactRepoStub)
	require.True(t, ok)
	require.Equal(t, 1, repo.createCalls)
	require.Equal(t, 0, usageRepo.calls)
	require.Equal(t, 0, userRepo.deductCalls)
	require.Equal(t, 0, quotaSvc.quotaCalls)
	require.Equal(t, "gateway_detached_ctx", payload.UsageLog.RequestID)
	require.Greater(t, payload.BillingCommand.APIKeyQuotaCost, 0.0)
}

func TestGatewayServiceBuildUsageFact_BillingFingerprintIncludesRequestPayloadHash(t *testing.T) {
	svc := newGatewayRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})

	payloadHash := HashUsageRequestPayload([]byte(`{"messages":[{"role":"user","content":"hello"}]}`))
	payload := buildGatewayUsageFactPayloadForTest(t, svc, context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "gateway_payload_hash",
			Usage: ClaudeUsage{
				InputTokens:  10,
				OutputTokens: 6,
			},
			Model:    "claude-sonnet-4",
			Duration: time.Second,
		},
		APIKey:             &APIKey{ID: 501, Quota: 100},
		User:               &User{ID: 601},
		Account:            &Account{ID: 701},
		RequestPayloadHash: payloadHash,
	})
	require.Equal(t, payloadHash, payload.BillingCommand.RequestPayloadHash)
}

func TestGatewayServiceBuildUsageFact_BillingFingerprintFallsBackToContextRequestID(t *testing.T) {
	svc := newGatewayRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})

	ctx := context.WithValue(context.Background(), ctxkey.RequestID, "req-local-123")
	payload := buildGatewayUsageFactPayloadForTest(t, svc, ctx, &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "gateway_payload_fallback",
			Usage: ClaudeUsage{
				InputTokens:  10,
				OutputTokens: 6,
			},
			Model:    "claude-sonnet-4",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 501, Quota: 100},
		User:    &User{ID: 601},
		Account: &Account{ID: 701},
	})
	require.Equal(t, "local:req-local-123", payload.BillingCommand.RequestPayloadHash)
}

func TestGatewayServiceBuildUsageFact_PreservesRequestedAndUpstreamModels(t *testing.T) {
	svc := newGatewayRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	mappedModel := "claude-sonnet-4-20250514"

	payload := buildGatewayUsageFactPayloadForTest(t, svc, context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID:     "gateway_models_split",
			Usage:         ClaudeUsage{InputTokens: 10, OutputTokens: 6},
			Model:         "claude-sonnet-4",
			UpstreamModel: mappedModel,
			Duration:      time.Second,
		},
		APIKey:  &APIKey{ID: 501, Quota: 100},
		User:    &User{ID: 601},
		Account: &Account{ID: 701},
	})

	require.Equal(t, "claude-sonnet-4", payload.UsageLog.Model)
	require.Equal(t, "claude-sonnet-4", payload.UsageLog.RequestedModel)
	require.NotNil(t, payload.UsageLog.UpstreamModel)
	require.Equal(t, mappedModel, *payload.UsageLog.UpstreamModel)
}

func TestGatewayServiceBuildUsageFact_EmptyImageSizeDefaultsBeforeBillingAndPersistence(t *testing.T) {
	groupID := int64(901)
	svc := newGatewayRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	svc.resolver = newOpenAIImageChannelPricingResolverForTest(t, groupID, "gemini-image", 0.19)

	payload := buildGatewayUsageFactPayloadForTest(t, svc, context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID:      "gateway_image_default_size",
			Model:          "gemini-image",
			ImageCount:     1,
			ImageInputSize: "auto",
			Duration:       time.Second,
		},
		APIKey: &APIKey{
			ID:      801,
			GroupID: i64p(groupID),
			Group: &Group{
				ID:             groupID,
				RateMultiplier: 1.0,
			},
		},
		User:    &User{ID: 601},
		Account: &Account{ID: 701},
	})

	require.Equal(t, 1, payload.UsageLog.ImageCount)
	require.NotNil(t, payload.UsageLog.ImageSize)
	require.Equal(t, ImageBillingSize2K, *payload.UsageLog.ImageSize)
	require.NotNil(t, payload.UsageLog.ImageInputSize)
	require.Equal(t, "auto", *payload.UsageLog.ImageInputSize)
	require.NotNil(t, payload.UsageLog.ImageSizeSource)
	require.Equal(t, ImageSizeSourceDefault, *payload.UsageLog.ImageSizeSource)
	require.InDelta(t, 0.19, payload.UsageLog.TotalCost, 1e-12)
	require.InDelta(t, 0.19, payload.UsageLog.ActualCost, 1e-12)
}

func TestGatewayServiceBuildUsageFact_LongContextFieldsBuildFact(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: false, err: context.DeadlineExceeded}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	quotaSvc := &openAIRecordUsageAPIKeyQuotaStub{}
	svc := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, subRepo)

	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()

	payload := buildGatewayUsageFactPayloadForTest(t, svc, reqCtx, &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "gateway_long_context_detached_ctx",
			Usage: ClaudeUsage{
				InputTokens:  12,
				OutputTokens: 8,
			},
			Model:    "claude-sonnet-4",
			Duration: time.Second,
		},
		APIKey: &APIKey{
			ID:    502,
			Quota: 100,
		},
		User:                  &User{ID: 602},
		Account:               &Account{ID: 702},
		LongContextThreshold:  200000,
		LongContextMultiplier: 2,
		APIKeyService:         quotaSvc,
	})

	require.Equal(t, 0, usageRepo.calls)
	require.Equal(t, 0, userRepo.deductCalls)
	require.Equal(t, 0, quotaSvc.quotaCalls)
	require.Equal(t, "gateway_long_context_detached_ctx", payload.UsageLog.RequestID)
}

func TestGatewayServiceBuildUsageFact_UsesFallbackRequestIDForUsageLog(t *testing.T) {
	svc := newGatewayRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})

	ctx := context.WithValue(context.Background(), ctxkey.RequestID, "gateway-local-fallback")
	payload := buildGatewayUsageFactPayloadForTest(t, svc, ctx, &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "",
			Usage: ClaudeUsage{
				InputTokens:  10,
				OutputTokens: 6,
			},
			Model:    "claude-sonnet-4",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 504},
		User:    &User{ID: 604},
		Account: &Account{ID: 704},
	})

	require.Equal(t, "local:gateway-local-fallback", payload.UsageLog.RequestID)
}

func TestGatewayServiceBuildUsageFact_PrefersClientRequestIDOverUpstreamRequestID(t *testing.T) {
	svc := newGatewayRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})

	ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "client-stable-123")
	ctx = context.WithValue(ctx, ctxkey.RequestID, "req-local-ignored")
	payload := buildGatewayUsageFactPayloadForTest(t, svc, ctx, &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "upstream-volatile-456",
			Usage: ClaudeUsage{
				InputTokens:  10,
				OutputTokens: 6,
			},
			Model:    "claude-sonnet-4",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 506},
		User:    &User{ID: 606},
		Account: &Account{ID: 706},
	})

	require.Equal(t, "client:client-stable-123", payload.BillingCommand.RequestID)
	require.Equal(t, "client:client-stable-123", payload.UsageLog.RequestID)
}

func TestGatewayServiceBuildUsageFact_GeneratesRequestIDWhenAllSourcesMissing(t *testing.T) {
	svc := newGatewayRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})

	payload := buildGatewayUsageFactPayloadForTest(t, svc, context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "",
			Usage: ClaudeUsage{
				InputTokens:  10,
				OutputTokens: 6,
			},
			Model:    "claude-sonnet-4",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 507},
		User:    &User{ID: 607},
		Account: &Account{ID: 707},
	})

	require.True(t, strings.HasPrefix(payload.BillingCommand.RequestID, "generated:"))
	require.Equal(t, payload.BillingCommand.RequestID, payload.UsageLog.RequestID)
}

func TestGatewayServiceBuildUsageFact_ReasoningEffortPersisted(t *testing.T) {
	svc := newGatewayRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})

	effort := "max"
	payload := buildGatewayUsageFactPayloadForTest(t, svc, context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "effort_test",
			Usage: ClaudeUsage{
				InputTokens:  10,
				OutputTokens: 5,
			},
			Model:           "claude-opus-4-6",
			Duration:        time.Second,
			ReasoningEffort: &effort,
		},
		APIKey:  &APIKey{ID: 1},
		User:    &User{ID: 1},
		Account: &Account{ID: 1},
	})

	require.NotNil(t, payload.UsageLog.ReasoningEffort)
	require.Equal(t, "max", *payload.UsageLog.ReasoningEffort)
}

func TestGatewayServiceBuildUsageFact_ReasoningEffortNil(t *testing.T) {
	svc := newGatewayRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})

	payload := buildGatewayUsageFactPayloadForTest(t, svc, context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "no_effort_test",
			Usage: ClaudeUsage{
				InputTokens:  10,
				OutputTokens: 5,
			},
			Model:    "claude-sonnet-4",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 1},
		User:    &User{ID: 1},
		Account: &Account{ID: 1},
	})

	require.Nil(t, payload.UsageLog.ReasoningEffort)
}
