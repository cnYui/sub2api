package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type openAIRecordUsageLogRepoStub struct {
	UsageLogRepository

	inserted   bool
	err        error
	calls      int
	lastLog    *UsageLog
	lastCtxErr error
}

func (s *openAIRecordUsageLogRepoStub) Create(ctx context.Context, log *UsageLog) (bool, error) {
	s.calls++
	s.lastLog = log
	s.lastCtxErr = ctx.Err()
	return s.inserted, s.err
}

type openAIRecordUsageBillingRepoStub struct {
	UsageBillingRepository

	result     *UsageBillingApplyResult
	err        error
	calls      int
	lastCmd    *UsageBillingCommand
	lastCtxErr error
}

func (s *openAIRecordUsageBillingRepoStub) Apply(ctx context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	s.calls++
	s.lastCmd = cmd
	s.lastCtxErr = ctx.Err()
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &UsageBillingApplyResult{Applied: true}, nil
}

type openAIRecordUsageFactRepoStub struct {
	UsageFactRepository
	createCalls int
	created     *UsageFact
	err         error
}

func (s *openAIRecordUsageFactRepoStub) CreatePending(ctx context.Context, fact *UsageFact) (*UsageFact, bool, error) {
	s.createCalls++
	s.created = fact
	if s.err != nil {
		return nil, false, s.err
	}
	persisted := *fact
	persisted.ID = 1
	return &persisted, true, nil
}

type openAIRecordUsageUserRepoStub struct {
	UserRepository

	deductCalls int
	deductErr   error
	lastAmount  float64
	lastCtxErr  error
}

func (s *openAIRecordUsageUserRepoStub) DeductBalance(ctx context.Context, id int64, amount float64) error {
	s.deductCalls++
	s.lastAmount = amount
	s.lastCtxErr = ctx.Err()
	return s.deductErr
}

type openAIRecordUsageSubRepoStub struct {
	UserSubscriptionRepository

	incrementCalls int
	incrementErr   error
	lastCtxErr     error
}

func (s *openAIRecordUsageSubRepoStub) IncrementUsage(ctx context.Context, id int64, costUSD float64) error {
	s.incrementCalls++
	s.lastCtxErr = ctx.Err()
	return s.incrementErr
}

type openAIRecordUsageAPIKeyQuotaStub struct {
	quotaCalls          int
	rateLimitCalls      int
	err                 error
	lastAmount          float64
	lastQuotaCtxErr     error
	lastRateLimitCtxErr error
}

func (s *openAIRecordUsageAPIKeyQuotaStub) UpdateQuotaUsed(ctx context.Context, apiKeyID int64, cost float64) error {
	s.quotaCalls++
	s.lastAmount = cost
	s.lastQuotaCtxErr = ctx.Err()
	return s.err
}

func (s *openAIRecordUsageAPIKeyQuotaStub) UpdateRateLimitUsage(ctx context.Context, apiKeyID int64, cost float64) error {
	s.rateLimitCalls++
	s.lastAmount = cost
	s.lastRateLimitCtxErr = ctx.Err()
	return s.err
}

type openAIUserGroupRateRepoStub struct {
	UserGroupRateRepository

	rate  *float64
	err   error
	calls int
}

func (s *openAIUserGroupRateRepoStub) GetByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.rate, nil
}

func i64p(v int64) *int64 {
	return &v
}

type openAIRecordUsageTestService struct {
	*OpenAIGatewayService
}

func newOpenAIRecordUsageServiceForTest(usageRepo UsageLogRepository, userRepo UserRepository, subRepo UserSubscriptionRepository, rateRepo UserGroupRateRepository) *openAIRecordUsageTestService {
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1.1
	svc := NewOpenAIGatewayService(
		nil,
		usageRepo,
		nil,
		userRepo,
		subRepo,
		rateRepo,
		nil,
		cfg,
		nil,
		nil,
		NewBillingService(cfg, nil),
		nil,
		&BillingCacheService{},
		nil,
		&DeferredService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil, // userPlatformQuotaRepo
		nil, // usageFactRepo
		nil, // billingAuthorizationService
	)
	svc.userGroupRateResolver = newUserGroupRateResolver(
		rateRepo,
		nil,
		resolveUserGroupRateCacheTTL(cfg),
		nil,
		"service.openai_gateway.test",
	)
	return &openAIRecordUsageTestService{OpenAIGatewayService: svc}
}

func newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo UsageLogRepository, billingRepo UsageBillingRepository, userRepo UserRepository, subRepo UserSubscriptionRepository, rateRepo UserGroupRateRepository) *openAIRecordUsageTestService {
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, subRepo, rateRepo)
	svc.usageBillingRepo = billingRepo
	return svc
}

func expectedOpenAICost(t *testing.T, svc *openAIRecordUsageTestService, model string, usage OpenAIUsage, multiplier float64) *CostBreakdown {
	t.Helper()

	cost, err := svc.billingService.CalculateCost(model, UsageTokens{
		InputTokens:         max(usage.InputTokens-usage.CacheReadInputTokens, 0),
		OutputTokens:        usage.OutputTokens,
		CacheCreationTokens: usage.CacheCreationInputTokens,
		CacheReadTokens:     usage.CacheReadInputTokens,
	}, multiplier)
	require.NoError(t, err)
	return cost
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func buildOpenAIUsageFactPayloadForTest(t *testing.T, svc *openAIRecordUsageTestService, ctx context.Context, input *OpenAIRecordUsageInput) UsageFactPayload {
	t.Helper()
	fact, err := svc.BuildUsageFact(ctx, input)
	require.NoError(t, err)
	return decodeUsageFactPayloadForTest(t, fact)
}

func persistOpenAIUsageFactForTest(t *testing.T, svc *openAIRecordUsageTestService, ctx context.Context, input *OpenAIRecordUsageInput) (*UsageFact, UsageFactPayload) {
	t.Helper()
	fact, err := svc.PersistUsageFact(ctx, input)
	require.NoError(t, err)
	return fact, decodeUsageFactPayloadForTest(t, fact)
}

func TestOpenAIGatewayServiceBuildUsageFact_RejectsNilInput(t *testing.T) {
	svc := &OpenAIGatewayService{}
	_, err := svc.BuildUsageFact(context.Background(), nil)
	require.Error(t, err)
	_, err = svc.BuildUsageFact(context.Background(), &OpenAIRecordUsageInput{})
	require.Error(t, err)
}

func TestOpenAIGatewayServicePersistUsageFact_DoesNotApplyBillingInline(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{}
	billingRepo := &openAIRecordUsageBillingRepoStub{}
	factRepo := &openAIRecordUsageFactRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(
		usageRepo,
		billingRepo,
		&openAIRecordUsageUserRepoStub{},
		&openAIRecordUsageSubRepoStub{},
		nil,
	)
	svc.usageFactRepo = factRepo

	_, payload := persistOpenAIUsageFactForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "req-1",
			Usage: OpenAIUsage{
				InputTokens:  10,
				OutputTokens: 2,
			},
			Model:    "gpt-5.1",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 9, Group: &Group{RateMultiplier: 1}},
		User:    &User{ID: 7},
		Account: &Account{ID: 5},
	})

	require.Equal(t, 1, factRepo.createCalls)
	require.Zero(t, billingRepo.calls)
	require.Zero(t, usageRepo.calls)
	require.Equal(t, "req-1", payload.BillingCommand.RequestID)
	require.Equal(t, "req-1", payload.UsageLog.RequestID)
}

func TestOpenAIGatewayServiceBuildUsageFact_UsesRequestBillingAuthorization(t *testing.T) {
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(
		&openAIRecordUsageLogRepoStub{},
		&openAIRecordUsageBillingRepoStub{},
		&openAIRecordUsageUserRepoStub{},
		&openAIRecordUsageSubRepoStub{},
		nil,
	)
	reservationID := int64(451)

	fact, err := svc.BuildUsageFact(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "req-reservation-source",
			Usage: OpenAIUsage{
				InputTokens:  1000,
				OutputTokens: 200,
			},
			Model:    "gpt-5.1",
			Duration: time.Second,
			BillingAuthorization: &OpenAIBillingAuthorization{
				Source:             BillingSourceTrafficCredit,
				ReservationID:      &reservationID,
				RequestFingerprint: "reservation-fingerprint",
			},
		},
		APIKey: &APIKey{
			ID:      91,
			GroupID: i64p(88),
			Group:   &Group{ID: 88, SubscriptionType: SubscriptionTypeSubscription, RateMultiplier: 1},
		},
		User:         &User{ID: 7, Balance: 100},
		Account:      &Account{ID: 5},
		Subscription: &UserSubscription{ID: 77},
	})

	require.NoError(t, err)
	require.NotNil(t, fact.ReservationID)
	require.Equal(t, reservationID, *fact.ReservationID)
	payload := decodeUsageFactPayloadForTest(t, fact)
	require.NotNil(t, payload.BillingCommand.TrafficCreditReservationID)
	require.Equal(t, reservationID, *payload.BillingCommand.TrafficCreditReservationID)
	require.Equal(t, "reservation-fingerprint", payload.BillingCommand.RequestFingerprint)
	require.Greater(t, payload.BillingCommand.TrafficPackCost, 0.0)
	require.Zero(t, payload.BillingCommand.BalanceCost)
	require.Zero(t, payload.BillingCommand.SubscriptionCost)
	require.Equal(t, int8(2), payload.UsageLog.BillingType)
	require.True(t, payload.Effects.IsTrafficCredit)
	require.False(t, payload.Effects.IsSubscription)
}

func TestOpenAIGatewayServiceBuildUsageFact_ShadowAuthorizationDoesNotCreateCharge(t *testing.T) {
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(
		&openAIRecordUsageLogRepoStub{},
		&openAIRecordUsageBillingRepoStub{},
		&openAIRecordUsageUserRepoStub{},
		&openAIRecordUsageSubRepoStub{},
		nil,
	)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "req-shadow-source",
			Usage: OpenAIUsage{
				InputTokens:  1000,
				OutputTokens: 200,
			},
			Model:    "gpt-5.1",
			Duration: time.Second,
			BillingAuthorization: &OpenAIBillingAuthorization{
				Source:             BillingSourceShadow,
				RequestFingerprint: "shadow-fingerprint",
			},
		},
		APIKey:  &APIKey{ID: 91, Group: &Group{RateMultiplier: 1}},
		User:    &User{ID: 7, Balance: 100},
		Account: &Account{ID: 5},
	})

	require.Zero(t, payload.BillingCommand.BalanceCost)
	require.Zero(t, payload.BillingCommand.SubscriptionCost)
	require.Zero(t, payload.BillingCommand.TrafficPackCost)
	require.Equal(t, int8(3), payload.UsageLog.BillingType)
}

func TestPersistCyberPolicyUsageFact_StoresRealUpstreamTokens(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, subRepo, nil)
	factRepo := &openAIRecordUsageFactRepoStub{}
	svc.usageFactRepo = factRepo
	usage := OpenAIUsage{InputTokens: 1200, OutputTokens: 300}

	// 流式 cyber 拒绝有真实 token，必须按真实 token 形成 durable fact，避免免费用量行漏计费。
	err := svc.PersistCyberPolicyUsageFact(context.Background(), CyberPolicyUsageInput{
		APIKey:       &APIKey{ID: 2, User: &User{ID: 1}},
		Account:      &Account{ID: 3},
		RequestID:    "rid-cyber-stream",
		Model:        "gpt-5.1",
		Stream:       true,
		InputTokens:  1200,
		OutputTokens: 300,
	})

	require.NoError(t, err)
	require.Equal(t, 0, usageRepo.calls)
	require.Equal(t, 0, userRepo.deductCalls)
	require.NotNil(t, factRepo.created)
	payload := decodeUsageFactPayloadForTest(t, factRepo.created)
	require.Equal(t, "gpt-5.1", payload.UsageLog.Model)
	require.Equal(t, 1200, payload.UsageLog.InputTokens)
	require.Equal(t, 300, payload.UsageLog.OutputTokens)
	require.Equal(t, RequestTypeCyberBlocked, payload.UsageLog.RequestType)
	require.True(t, payload.UsageLog.Stream)

	expected := expectedOpenAICost(t, svc, "gpt-5.1", usage, 1.1)
	require.Greater(t, payload.UsageLog.ActualCost, 0.0)
	require.InDelta(t, expected.ActualCost, payload.UsageLog.ActualCost, 1e-12)
	require.InDelta(t, expected.ActualCost, payload.BillingCommand.BalanceCost, 1e-12)
}

func TestPersistCyberPolicyUsageFact_NonStreamZeroTokensZeroCost(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, subRepo, nil)
	factRepo := &openAIRecordUsageFactRepoStub{}
	svc.usageFactRepo = factRepo

	// 非流式直接拒未返回 token 时保留可见 fact，但自然为 0 cost。
	err := svc.PersistCyberPolicyUsageFact(context.Background(), CyberPolicyUsageInput{
		APIKey:    &APIKey{ID: 2, User: &User{ID: 1}},
		Account:   &Account{ID: 3},
		RequestID: "rid-cyber-400",
		Model:     "gpt-5.1",
		Stream:    false,
	})

	require.NoError(t, err)
	require.NotNil(t, factRepo.created)
	payload := decodeUsageFactPayloadForTest(t, factRepo.created)
	require.Zero(t, payload.UsageLog.InputTokens)
	require.Zero(t, payload.UsageLog.OutputTokens)
	require.Zero(t, payload.UsageLog.TotalCost)
	require.Equal(t, RequestTypeCyberBlocked, payload.UsageLog.RequestType)
}

func TestPersistCyberPolicyUsageFact_SkipsWhenIncomplete(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	factRepo := &openAIRecordUsageFactRepoStub{}
	svc.usageFactRepo = factRepo

	acct := &Account{ID: 3}
	require.NoError(t, svc.PersistCyberPolicyUsageFact(context.Background(), CyberPolicyUsageInput{Account: acct, Model: "gpt-5"}))
	require.NoError(t, svc.PersistCyberPolicyUsageFact(context.Background(), CyberPolicyUsageInput{APIKey: &APIKey{ID: 2}, Account: acct, Model: "gpt-5"}))
	require.NoError(t, svc.PersistCyberPolicyUsageFact(context.Background(), CyberPolicyUsageInput{APIKey: &APIKey{ID: 2, User: &User{ID: 1}}, Model: "gpt-5"}))
	require.NoError(t, svc.PersistCyberPolicyUsageFact(context.Background(), CyberPolicyUsageInput{APIKey: &APIKey{ID: 2, User: &User{ID: 1}}, Account: acct}))
	require.Equal(t, 0, factRepo.createCalls)
}

func TestOpenAIGatewayServiceBuildUsageFact_ZeroUsageBuildsZeroCostPayload(t *testing.T) {
	quotaSvc := &openAIRecordUsageAPIKeyQuotaStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageBillingRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_zero_usage",
			Usage:     OpenAIUsage{},
			Model:     "gpt-5.1",
			Duration:  time.Second,
		},
		APIKey:        &APIKey{ID: 1000, Quota: 100, Group: &Group{RateMultiplier: 1}},
		User:          &User{ID: 2000},
		Account:       &Account{ID: 3000, Type: AccountTypeAPIKey},
		APIKeyService: quotaSvc,
	})

	require.Equal(t, "resp_zero_usage", payload.UsageLog.RequestID)
	require.Zero(t, payload.UsageLog.InputTokens)
	require.Zero(t, payload.UsageLog.OutputTokens)
	require.Zero(t, payload.UsageLog.CacheCreationTokens)
	require.Zero(t, payload.UsageLog.CacheReadTokens)
	require.Zero(t, payload.UsageLog.ImageOutputTokens)
	require.Zero(t, payload.UsageLog.ImageCount)
	require.Zero(t, payload.UsageLog.InputCost)
	require.Zero(t, payload.UsageLog.OutputCost)
	require.Zero(t, payload.UsageLog.TotalCost)
	require.Zero(t, payload.UsageLog.ActualCost)
	require.Zero(t, payload.BillingCommand.BalanceCost)
	require.Zero(t, payload.BillingCommand.SubscriptionCost)
	require.Zero(t, payload.BillingCommand.APIKeyQuotaCost)
	require.Zero(t, payload.BillingCommand.APIKeyRateLimitCost)
	require.Zero(t, payload.BillingCommand.AccountQuotaCost)
	require.Equal(t, 0, quotaSvc.quotaCalls)
}

func TestOpenAIGatewayServiceBuildUsageFact_MissingPricingRecordsZeroCostPayload(t *testing.T) {
	quotaSvc := &openAIRecordUsageAPIKeyQuotaStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageBillingRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_missing_pricing",
			Usage: OpenAIUsage{
				InputTokens:  1200,
				OutputTokens: 300,
			},
			Model:    "pricing-missing-test-model",
			Duration: time.Second,
		},
		APIKey:        &APIKey{ID: 1002, Quota: 100, Group: &Group{RateMultiplier: 1}},
		User:          &User{ID: 2002},
		Account:       &Account{ID: 3002, Type: AccountTypeAPIKey},
		APIKeyService: quotaSvc,
	})

	require.Equal(t, "resp_missing_pricing", payload.UsageLog.RequestID)
	require.Equal(t, "pricing-missing-test-model", payload.UsageLog.Model)
	require.Equal(t, "pricing-missing-test-model", payload.UsageLog.RequestedModel)
	require.Equal(t, 1200, payload.UsageLog.InputTokens)
	require.Equal(t, 300, payload.UsageLog.OutputTokens)
	require.Zero(t, payload.UsageLog.TotalCost)
	require.Zero(t, payload.UsageLog.ActualCost)
	require.NotNil(t, payload.UsageLog.BillingMode)
	require.Equal(t, string(BillingModeToken), *payload.UsageLog.BillingMode)
	require.Zero(t, payload.BillingCommand.BalanceCost)
	require.Zero(t, payload.BillingCommand.SubscriptionCost)
	require.Zero(t, payload.BillingCommand.APIKeyQuotaCost)
	require.Zero(t, payload.BillingCommand.APIKeyRateLimitCost)
	require.Zero(t, payload.BillingCommand.AccountQuotaCost)
}

func TestOpenAIGatewayServiceBuildUsageFact_UsesUserSpecificGroupRate(t *testing.T) {
	groupID := int64(11)
	groupRate := 1.4
	userRate := 1.8
	usage := OpenAIUsage{InputTokens: 15, OutputTokens: 4, CacheReadInputTokens: 3}
	rateRepo := &openAIUserGroupRateRepoStub{rate: &userRate}
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, rateRepo)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_user_group_rate",
			Usage:     usage,
			Model:     "gpt-5.1",
			Duration:  time.Second,
		},
		APIKey: &APIKey{
			ID:      1001,
			GroupID: i64p(groupID),
			Group: &Group{
				ID:             groupID,
				RateMultiplier: groupRate,
			},
		},
		User:    &User{ID: 2001},
		Account: &Account{ID: 3001},
	})

	require.Equal(t, 1, rateRepo.calls)
	require.Equal(t, userRate, payload.UsageLog.RateMultiplier)
	require.Equal(t, 12, payload.UsageLog.InputTokens)
	require.Equal(t, 3, payload.UsageLog.CacheReadTokens)
	expected := expectedOpenAICost(t, svc, "gpt-5.1", usage, userRate)
	require.InDelta(t, expected.ActualCost, payload.UsageLog.ActualCost, 1e-12)
	require.InDelta(t, expected.ActualCost, payload.BillingCommand.BalanceCost, 1e-12)
}

func TestOpenAIGatewayServiceBuildUsageFact_IncludesEndpointMetadata(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, &openAIUserGroupRateRepoStub{})

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_endpoint_metadata",
			Usage: OpenAIUsage{
				InputTokens:  8,
				OutputTokens: 2,
			},
			Model:    "gpt-5.1",
			Duration: time.Second,
		},
		APIKey: &APIKey{
			ID:    1002,
			Group: &Group{RateMultiplier: 1},
		},
		User:             &User{ID: 2002},
		Account:          &Account{ID: 3002},
		InboundEndpoint:  " /v1/chat/completions ",
		UpstreamEndpoint: " /v1/responses ",
	})

	require.NotNil(t, payload.UsageLog.InboundEndpoint)
	require.Equal(t, "/v1/chat/completions", *payload.UsageLog.InboundEndpoint)
	require.NotNil(t, payload.UsageLog.UpstreamEndpoint)
	require.Equal(t, "/v1/responses", *payload.UsageLog.UpstreamEndpoint)
}

func TestOpenAIGatewayServiceBuildUsageFact_FallsBackToGroupDefaultRateOnResolverError(t *testing.T) {
	groupID := int64(12)
	groupRate := 1.6
	usage := OpenAIUsage{InputTokens: 10, OutputTokens: 5, CacheReadInputTokens: 2}
	rateRepo := &openAIUserGroupRateRepoStub{err: errors.New("db unavailable")}
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, rateRepo)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_group_default_on_error",
			Usage:     usage,
			Model:     "gpt-5.1",
			Duration:  time.Second,
		},
		APIKey: &APIKey{
			ID:      1002,
			GroupID: i64p(groupID),
			Group: &Group{
				ID:             groupID,
				RateMultiplier: groupRate,
			},
		},
		User:    &User{ID: 2002},
		Account: &Account{ID: 3002},
	})

	require.Equal(t, 1, rateRepo.calls)
	require.Equal(t, groupRate, payload.UsageLog.RateMultiplier)
	expected := expectedOpenAICost(t, svc, "gpt-5.1", usage, groupRate)
	require.InDelta(t, expected.ActualCost, payload.BillingCommand.BalanceCost, 1e-12)
}

func TestOpenAIGatewayServiceBuildUsageFact_FallsBackToGroupDefaultRateWhenResolverMissing(t *testing.T) {
	groupID := int64(13)
	groupRate := 1.25
	usage := OpenAIUsage{InputTokens: 9, OutputTokens: 4, CacheReadInputTokens: 1}
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.userGroupRateResolver = nil

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_group_default_nil_resolver",
			Usage:     usage,
			Model:     "gpt-5.1",
			Duration:  time.Second,
		},
		APIKey: &APIKey{
			ID:      1003,
			GroupID: i64p(groupID),
			Group: &Group{
				ID:             groupID,
				RateMultiplier: groupRate,
			},
		},
		User:    &User{ID: 2003},
		Account: &Account{ID: 3003},
	})

	require.Equal(t, groupRate, payload.UsageLog.RateMultiplier)
}

func TestOpenAIGatewayServiceBuildUsageFact_BillingFingerprintIncludesRequestPayloadHash(t *testing.T) {
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageBillingRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	payloadHash := HashUsageRequestPayload([]byte(`{"model":"gpt-5","input":"hello"}`))
	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "openai_payload_hash",
			Usage: OpenAIUsage{
				InputTokens:  10,
				OutputTokens: 6,
			},
			Model:    "gpt-5",
			Duration: time.Second,
		},
		APIKey:             &APIKey{ID: 501, Quota: 100},
		User:               &User{ID: 601},
		Account:            &Account{ID: 701},
		RequestPayloadHash: payloadHash,
	})
	require.Equal(t, payloadHash, payload.BillingCommand.RequestPayloadHash)
}

func TestOpenAIGatewayServiceBuildUsageFact_UsesFallbackRequestIDForBillingAndUsageLog(t *testing.T) {
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageBillingRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	ctx := context.WithValue(context.Background(), ctxkey.RequestID, "req-local-fallback")
	payload := buildOpenAIUsageFactPayloadForTest(t, svc, ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "",
			Usage: OpenAIUsage{
				InputTokens:  8,
				OutputTokens: 4,
			},
			Model:    "gpt-5.1",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 10047},
		User:    &User{ID: 20047},
		Account: &Account{ID: 30047},
	})

	require.Equal(t, "local:req-local-fallback", payload.BillingCommand.RequestID)
	require.Equal(t, "local:req-local-fallback", payload.UsageLog.RequestID)
}

func TestOpenAIGatewayServiceBuildUsageFact_PrefersClientRequestIDOverUpstreamRequestID(t *testing.T) {
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageBillingRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "openai-client-stable-123")
	payload := buildOpenAIUsageFactPayloadForTest(t, svc, ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "upstream-openai-volatile-456",
			Usage: OpenAIUsage{
				InputTokens:  8,
				OutputTokens: 4,
			},
			Model:    "gpt-5.1",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 10049},
		User:    &User{ID: 20049},
		Account: &Account{ID: 30049},
	})

	require.Equal(t, "client:openai-client-stable-123", payload.BillingCommand.RequestID)
	require.Equal(t, "client:openai-client-stable-123", payload.UsageLog.RequestID)
}

func TestOpenAIGatewayServiceBuildUsageFact_WSModePrefersUpstreamRequestIDOverClientRequestID(t *testing.T) {
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageBillingRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "openai-ws-connection-123")
	payload := buildOpenAIUsageFactPayloadForTest(t, svc, ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:    "resp_openai_ws_turn_456",
			OpenAIWSMode: true,
			Usage: OpenAIUsage{
				InputTokens:  8,
				OutputTokens: 4,
			},
			Model:    "gpt-5.1",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 10050},
		User:    &User{ID: 20050},
		Account: &Account{ID: 30050},
	})

	require.Equal(t, "resp_openai_ws_turn_456", payload.BillingCommand.RequestID)
	require.Equal(t, "resp_openai_ws_turn_456", payload.UsageLog.RequestID)
}

func TestOpenAIGatewayServiceBuildUsageFact_GeneratesRequestIDWhenAllSourcesMissing(t *testing.T) {
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageBillingRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "",
			Usage: OpenAIUsage{
				InputTokens:  8,
				OutputTokens: 4,
			},
			Model:    "gpt-5.1",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 10050},
		User:    &User{ID: 20050},
		Account: &Account{ID: 30050},
	})

	require.True(t, strings.HasPrefix(payload.BillingCommand.RequestID, "generated:"))
	require.Equal(t, payload.BillingCommand.RequestID, payload.UsageLog.RequestID)
}

func TestOpenAIGatewayServiceBuildUsageFact_UpdatesAPIKeyQuotaWhenConfigured(t *testing.T) {
	usage := OpenAIUsage{InputTokens: 10, OutputTokens: 6, CacheReadInputTokens: 2}
	quotaSvc := &openAIRecordUsageAPIKeyQuotaStub{}
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_quota_update",
			Usage:     usage,
			Model:     "gpt-5.1",
			Duration:  time.Second,
		},
		APIKey: &APIKey{
			ID:    1005,
			Quota: 100,
		},
		User:          &User{ID: 2005},
		Account:       &Account{ID: 3005},
		APIKeyService: quotaSvc,
	})

	require.Equal(t, 0, quotaSvc.quotaCalls)
	expected := expectedOpenAICost(t, svc, "gpt-5.1", usage, 1.1)
	require.InDelta(t, expected.ActualCost, payload.BillingCommand.APIKeyQuotaCost, 1e-12)
	require.Zero(t, payload.BillingCommand.APIKeyRateLimitCost)
}

func TestOpenAIGatewayServiceBuildUsageFact_ClampsActualInputTokensToZero(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_clamp_actual_input",
			Usage: OpenAIUsage{
				InputTokens:          2,
				OutputTokens:         1,
				CacheReadInputTokens: 5,
			},
			Model:    "gpt-5.1",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 1006},
		User:    &User{ID: 2006},
		Account: &Account{ID: 3006},
	})

	require.Equal(t, 0, payload.UsageLog.InputTokens)
}

func TestOpenAIGatewayServiceBuildUsageFact_Gpt54LongContextBillsWholeSession(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_gpt54_long_context",
			Usage: OpenAIUsage{
				InputTokens:  300000,
				OutputTokens: 2000,
			},
			Model:    "gpt-5.4-2026-03-05",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 1014},
		User:    &User{ID: 2014},
		Account: &Account{ID: 3014},
	})

	expectedInput := 300000 * 2.5e-6 * 2.0
	expectedOutput := 2000 * 15e-6 * 1.5
	require.InDelta(t, expectedInput, payload.UsageLog.InputCost, 1e-10)
	require.InDelta(t, expectedOutput, payload.UsageLog.OutputCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, payload.UsageLog.TotalCost, 1e-10)
	require.InDelta(t, (expectedInput+expectedOutput)*1.1, payload.UsageLog.ActualCost, 1e-10)
	require.InDelta(t, payload.UsageLog.ActualCost, payload.BillingCommand.BalanceCost, 1e-10)
}

func TestOpenAIGatewayServiceBuildUsageFact_ServiceTierPriorityUsesOfficialMultiplier(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	serviceTier := "priority"
	usage := OpenAIUsage{InputTokens: 100, OutputTokens: 50}

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:   "resp_service_tier_priority",
			ServiceTier: &serviceTier,
			Usage:       usage,
			Model:       "gpt-5.4",
			Duration:    time.Second,
		},
		APIKey:  &APIKey{ID: 1015},
		User:    &User{ID: 2015},
		Account: &Account{ID: 3015},
	})

	require.NotNil(t, payload.UsageLog.ServiceTier)
	require.Equal(t, serviceTier, *payload.UsageLog.ServiceTier)
	baseCost, calcErr := svc.billingService.CalculateCost("gpt-5.4", UsageTokens{InputTokens: 100, OutputTokens: 50}, 1.0)
	require.NoError(t, calcErr)
	require.InDelta(t, baseCost.TotalCost*2.0, payload.UsageLog.TotalCost, 1e-10)
}

func TestOpenAIGatewayServiceBuildUsageFact_ServiceTierFlexHalvesCost(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	serviceTier := "flex"
	usage := OpenAIUsage{InputTokens: 100, OutputTokens: 50, CacheReadInputTokens: 20}

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:   "resp_service_tier_flex",
			ServiceTier: &serviceTier,
			Usage:       usage,
			Model:       "gpt-5.4",
			Duration:    time.Second,
		},
		APIKey:  &APIKey{ID: 1016},
		User:    &User{ID: 2016},
		Account: &Account{ID: 3016},
	})

	baseCost, calcErr := svc.billingService.CalculateCost("gpt-5.4", UsageTokens{InputTokens: 80, OutputTokens: 50, CacheReadTokens: 20}, 1.0)
	require.NoError(t, calcErr)
	require.InDelta(t, baseCost.TotalCost*0.5, payload.UsageLog.TotalCost, 1e-10)
}

func TestNormalizeOpenAIServiceTier(t *testing.T) {
	t.Run("fast maps to priority", func(t *testing.T) {
		got := normalizeOpenAIServiceTier(" fast ")
		require.NotNil(t, got)
		require.Equal(t, "priority", *got)
	})

	t.Run("openai official tiers preserved", func(t *testing.T) {
		// OpenAI 官方合法 tier 必须透传，避免白名单过窄静默剥离客户端显式字段。
		for _, tier := range []string{"priority", "flex", "auto", "default", "scale"} {
			got := normalizeOpenAIServiceTier(tier)
			require.NotNil(t, got, "tier %q should not be normalized to nil", tier)
			require.Equal(t, tier, *got)
		}
	})

	t.Run("invalid ignored", func(t *testing.T) {
		require.Nil(t, normalizeOpenAIServiceTier("turbo"))
		require.Nil(t, normalizeOpenAIServiceTier("xxx"))
	})
}

func TestExtractOpenAIServiceTier(t *testing.T) {
	require.Equal(t, "priority", *extractOpenAIServiceTier(map[string]any{"service_tier": "fast"}))
	require.Equal(t, "flex", *extractOpenAIServiceTier(map[string]any{"service_tier": "flex"}))
	require.Equal(t, "auto", *extractOpenAIServiceTier(map[string]any{"service_tier": "auto"}))
	require.Equal(t, "default", *extractOpenAIServiceTier(map[string]any{"service_tier": "default"}))
	require.Equal(t, "scale", *extractOpenAIServiceTier(map[string]any{"service_tier": "scale"}))
	require.Nil(t, extractOpenAIServiceTier(map[string]any{"service_tier": 1}))
	require.Nil(t, extractOpenAIServiceTier(nil))
}

func TestExtractOpenAIServiceTierFromBody(t *testing.T) {
	require.Equal(t, "priority", *extractOpenAIServiceTierFromBody([]byte(`{"service_tier":"fast"}`)))
	require.Equal(t, "flex", *extractOpenAIServiceTierFromBody([]byte(`{"service_tier":"flex"}`)))
	require.Equal(t, "auto", *extractOpenAIServiceTierFromBody([]byte(`{"service_tier":"auto"}`)))
	require.Equal(t, "default", *extractOpenAIServiceTierFromBody([]byte(`{"service_tier":"default"}`)))
	require.Equal(t, "scale", *extractOpenAIServiceTierFromBody([]byte(`{"service_tier":"scale"}`)))
	require.Nil(t, extractOpenAIServiceTierFromBody([]byte(`{"service_tier":"turbo"}`)))
	require.Nil(t, extractOpenAIServiceTierFromBody(nil))
}

func TestOpenAIGatewayServiceBuildUsageFact_UsesRequestedModelAndUpstreamModelMetadataFields(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	serviceTier := "priority"
	reasoning := "high"

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:       "resp_billing_model_override",
			BillingModel:    "gpt-5.1-codex",
			Model:           "gpt-5.1",
			UpstreamModel:   "gpt-5.1-codex",
			ServiceTier:     &serviceTier,
			ReasoningEffort: &reasoning,
			Usage: OpenAIUsage{
				InputTokens:  20,
				OutputTokens: 10,
			},
			Duration:     2 * time.Second,
			FirstTokenMs: func() *int { v := 120; return &v }(),
		},
		APIKey:    &APIKey{ID: 10, GroupID: i64p(11), Group: &Group{ID: 11, RateMultiplier: 1.2}},
		User:      &User{ID: 20},
		Account:   &Account{ID: 30},
		UserAgent: "codex-cli/1.0",
		IPAddress: "127.0.0.1",
	})

	require.Equal(t, "gpt-5.1", payload.UsageLog.Model)
	require.Equal(t, "gpt-5.1", payload.UsageLog.RequestedModel)
	require.NotNil(t, payload.UsageLog.UpstreamModel)
	require.Equal(t, "gpt-5.1-codex", *payload.UsageLog.UpstreamModel)
	require.NotNil(t, payload.UsageLog.ServiceTier)
	require.Equal(t, serviceTier, *payload.UsageLog.ServiceTier)
	require.NotNil(t, payload.UsageLog.ReasoningEffort)
	require.Equal(t, reasoning, *payload.UsageLog.ReasoningEffort)
	require.NotNil(t, payload.UsageLog.UserAgent)
	require.Equal(t, "codex-cli/1.0", *payload.UsageLog.UserAgent)
	require.NotNil(t, payload.UsageLog.IPAddress)
	require.Equal(t, "127.0.0.1", *payload.UsageLog.IPAddress)
	require.NotNil(t, payload.UsageLog.GroupID)
	require.Equal(t, int64(11), *payload.UsageLog.GroupID)
	require.InDelta(t, payload.UsageLog.ActualCost, payload.BillingCommand.BalanceCost, 1e-12)
}

func TestOpenAIGatewayServiceBuildUsageFact_BillsMappedRequestsUsingRequestedModel(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	usage := OpenAIUsage{InputTokens: 20, OutputTokens: 10}

	expectedCost, err := svc.billingService.CalculateCost("gpt-5.1", UsageTokens{
		InputTokens:  20,
		OutputTokens: 10,
	}, 1.1)
	require.NoError(t, err)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:     "resp_upstream_model_billing_fallback",
			Model:         "gpt-5.1",
			UpstreamModel: "gpt-5.1-codex",
			Usage:         usage,
			Duration:      time.Second,
		},
		APIKey:  &APIKey{ID: 10},
		User:    &User{ID: 20},
		Account: &Account{ID: 30},
	})

	require.Equal(t, "gpt-5.1", payload.UsageLog.Model)
	require.Equal(t, expectedCost.ActualCost, payload.UsageLog.ActualCost)
	require.Equal(t, expectedCost.TotalCost, payload.UsageLog.TotalCost)
	require.Equal(t, expectedCost.ActualCost, payload.BillingCommand.BalanceCost)
}

func TestOpenAIGatewayServiceBuildUsageFact_ChannelMappedDoesNotOverrideBillingModelWhenUnmapped(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	usage := OpenAIUsage{InputTokens: 20, OutputTokens: 10}
	expectedCost, err := svc.billingService.CalculateCost("gpt-5.1", UsageTokens{
		InputTokens:  20,
		OutputTokens: 10,
	}, 1.1)
	require.NoError(t, err)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:     "resp_channel_unmapped_billing",
			Model:         "glm",
			BillingModel:  "gpt-5.1",
			UpstreamModel: "gpt-5.1",
			Usage:         usage,
			Duration:      time.Second,
		},
		APIKey:  &APIKey{ID: 10},
		User:    &User{ID: 20},
		Account: &Account{ID: 30},
		ChannelUsageFields: ChannelUsageFields{
			ChannelID:          1,
			OriginalModel:      "glm",
			ChannelMappedModel: "glm",
			BillingModelSource: BillingModelSourceChannelMapped,
		},
	})

	require.Equal(t, expectedCost.ActualCost, payload.UsageLog.ActualCost)
	require.True(t, payload.UsageLog.ActualCost > 0, "cost must not be zero")
}

func TestOpenAIGatewayServiceBuildUsageFact_ChannelMappedOverridesBillingModelWhenMapped(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	usage := OpenAIUsage{InputTokens: 20, OutputTokens: 10}
	expectedCost, err := svc.billingService.CalculateCost("gpt-5.1", UsageTokens{
		InputTokens:  20,
		OutputTokens: 10,
	}, 1.1)
	require.NoError(t, err)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:     "resp_channel_mapped_billing",
			Model:         "glm",
			BillingModel:  "gpt-5.1-codex",
			UpstreamModel: "gpt-5.1-codex",
			Usage:         usage,
			Duration:      time.Second,
		},
		APIKey:  &APIKey{ID: 10},
		User:    &User{ID: 20},
		Account: &Account{ID: 30},
		ChannelUsageFields: ChannelUsageFields{
			ChannelID:          1,
			OriginalModel:      "glm",
			ChannelMappedModel: "gpt-5.1",
			BillingModelSource: BillingModelSourceChannelMapped,
		},
	})

	require.Equal(t, expectedCost.ActualCost, payload.UsageLog.ActualCost)
	require.True(t, payload.UsageLog.ActualCost > 0, "cost must not be zero")
}

func TestOpenAIGatewayServiceBuildUsageFact_BillsCompactOpenAIModelAlias(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	usage := OpenAIUsage{InputTokens: 20, OutputTokens: 10}
	expectedCost, err := svc.billingService.CalculateCost("gpt-5.5", UsageTokens{
		InputTokens:  20,
		OutputTokens: 10,
	}, 1.1)
	require.NoError(t, err)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:     "resp_compact_openai_alias",
			Model:         "gpt5.5",
			UpstreamModel: "gpt-5.4",
			Usage:         usage,
			Duration:      time.Second,
		},
		APIKey:  &APIKey{ID: 10},
		User:    &User{ID: 20},
		Account: &Account{ID: 30},
	})

	require.Equal(t, "gpt5.5", payload.UsageLog.Model)
	require.NotNil(t, payload.UsageLog.UpstreamModel)
	require.Equal(t, "gpt-5.4", *payload.UsageLog.UpstreamModel)
	require.InDelta(t, expectedCost.ActualCost, payload.UsageLog.ActualCost, 1e-12)
	require.True(t, payload.UsageLog.ActualCost > 0, "cost must not be zero")
	require.InDelta(t, expectedCost.ActualCost, payload.BillingCommand.BalanceCost, 1e-12)
}

func TestOpenAIGatewayServiceBuildUsageFact_FallsBackToUpstreamModelWhenPrimaryUnpriceable(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	usage := OpenAIUsage{InputTokens: 20, OutputTokens: 10}
	expectedCost, err := svc.billingService.CalculateCost("gpt-5.4", UsageTokens{
		InputTokens:  20,
		OutputTokens: 10,
	}, 1.1)
	require.NoError(t, err)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:     "resp_unpriceable_primary_upstream_fallback",
			Model:         "not-priceable-alias",
			BillingModel:  "not-priceable-alias",
			UpstreamModel: "gpt-5.4",
			Usage:         usage,
			Duration:      time.Second,
		},
		APIKey:  &APIKey{ID: 10},
		User:    &User{ID: 20},
		Account: &Account{ID: 30},
	})

	require.InDelta(t, expectedCost.ActualCost, payload.UsageLog.ActualCost, 1e-12)
	require.True(t, payload.UsageLog.ActualCost > 0, "cost must not be zero")
	require.InDelta(t, expectedCost.ActualCost, payload.BillingCommand.BalanceCost, 1e-12)
}

func TestOpenAIGatewayServiceBuildUsageFact_UnpricedTokenModelFallsBackToZeroCostPayload(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_unpriceable_without_upstream",
			Model:     "not-priceable-alias",
			Usage:     OpenAIUsage{InputTokens: 20, OutputTokens: 10},
			Duration:  time.Second,
		},
		APIKey:  &APIKey{ID: 10},
		User:    &User{ID: 20},
		Account: &Account{ID: 30},
	})

	require.Equal(t, "not-priceable-alias", payload.UsageLog.Model)
	require.Equal(t, 20, payload.UsageLog.InputTokens)
	require.Equal(t, 10, payload.UsageLog.OutputTokens)
	require.Zero(t, payload.UsageLog.TotalCost)
	require.Zero(t, payload.UsageLog.ActualCost)
	require.Zero(t, payload.BillingCommand.BalanceCost)
	require.Zero(t, payload.BillingCommand.SubscriptionCost)
}

func TestOpenAIGatewayServiceBuildUsageFact_SubscriptionBillingSetsSubscriptionFields(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	subscription := &UserSubscription{ID: 99}

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_subscription_billing",
			Usage:     OpenAIUsage{InputTokens: 10, OutputTokens: 5},
			Model:     "gpt-5.1",
			Duration:  time.Second,
		},
		APIKey:       &APIKey{ID: 100, GroupID: i64p(88), Group: &Group{ID: 88, SubscriptionType: SubscriptionTypeSubscription, RateMultiplier: 1.0}},
		User:         &User{ID: 200},
		Account:      &Account{ID: 300},
		Subscription: subscription,
	})

	require.Equal(t, BillingTypeSubscription, payload.UsageLog.BillingType)
	require.NotNil(t, payload.UsageLog.SubscriptionID)
	require.Equal(t, subscription.ID, *payload.UsageLog.SubscriptionID)
	require.Greater(t, payload.BillingCommand.SubscriptionCost, 0.0)
	require.Zero(t, payload.BillingCommand.BalanceCost)
}

func TestOpenAIGatewayServiceBuildUsageFact_SubscriptionAuthorizationSetsSubscriptionFields(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	subscription := &UserSubscription{ID: 99}

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_subscription_authorized",
			Usage:     OpenAIUsage{InputTokens: 10, OutputTokens: 5},
			Model:     "gpt-5.1",
			Duration:  time.Second,
			BillingAuthorization: &OpenAIBillingAuthorization{
				Source:             BillingSourceSubscription,
				RequestFingerprint: "fp-subscription",
			},
		},
		APIKey:       &APIKey{ID: 100, GroupID: i64p(88), Group: &Group{ID: 88, SubscriptionType: SubscriptionTypeSubscription, RateMultiplier: 1.0}},
		User:         &User{ID: 200},
		Account:      &Account{ID: 300},
		Subscription: subscription,
	})

	require.Equal(t, BillingTypeSubscription, payload.UsageLog.BillingType)
	require.NotNil(t, payload.UsageLog.SubscriptionID)
	require.Equal(t, subscription.ID, *payload.UsageLog.SubscriptionID)
	require.Greater(t, payload.BillingCommand.SubscriptionCost, 0.0)
	require.Zero(t, payload.BillingCommand.BalanceCost)
}

func TestOpenAIGatewayServiceBuildUsageFact_SimpleModeClearsBillingCommandCosts(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.cfg.RunMode = config.RunModeSimple

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_simple_mode",
			Usage:     OpenAIUsage{InputTokens: 10, OutputTokens: 5},
			Model:     "gpt-5.1",
			Duration:  time.Second,
		},
		APIKey:  &APIKey{ID: 1000},
		User:    &User{ID: 2000},
		Account: &Account{ID: 3000},
	})

	require.Greater(t, payload.UsageLog.ActualCost, 0.0)
	require.Zero(t, payload.BillingCommand.BalanceCost)
	require.Zero(t, payload.BillingCommand.SubscriptionCost)
	require.Zero(t, payload.BillingCommand.TrafficPackCost)
}

func TestOpenAIGatewayServiceBuildUsageFact_ImageOnlyUsageStillPersists(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:  "resp_image_only_usage",
			Model:      "gpt-image-2",
			ImageCount: 2,
			ImageSize:  "1K",
			Duration:   time.Second,
		},
		APIKey:  &APIKey{ID: 1007},
		User:    &User{ID: 2007},
		Account: &Account{ID: 3007},
	})

	require.Equal(t, 2, payload.UsageLog.ImageCount)
	require.NotNil(t, payload.UsageLog.ImageSize)
	require.Equal(t, "1K", *payload.UsageLog.ImageSize)
	require.NotNil(t, payload.UsageLog.BillingMode)
	require.Equal(t, string(BillingModeToken), *payload.UsageLog.BillingMode)
}

func TestOpenAIGatewayServiceBuildUsageFact_OpenAIImageUsesMainAndImageTokenPricing(t *testing.T) {
	groupID := int64(501)
	svc := newOpenAIRecordUsageServiceWithOpenAITokenPricingForTest(t, groupID)

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:         "resp_openai_image_token_pricing",
			Model:             "gpt-main",
			MainBillingModel:  "gpt-main",
			ImageBillingModel: "gpt-image-2",
			ImageCount:        1,
			ImageSize:         "1K",
			Duration:          time.Second,
			Usage: OpenAIUsage{
				InputTokens:              130,
				ImageInputTokens:         20,
				OutputTokens:             70,
				CacheCreationInputTokens: 5,
				CacheReadInputTokens:     10,
				ImageOutputTokens:        40,
			},
			UsagePresence: OpenAIUsagePresence{
				Input:       true,
				ImageInput:  true,
				Output:      true,
				ImageOutput: true,
				CacheRead:   true,
			},
			UsageExpectation: OpenAIUsageExpectation{
				Input:       true,
				ImageInput:  true,
				Output:      true,
				ImageOutput: true,
				CacheRead:   true,
			},
		},
		APIKey:  &APIKey{ID: 1008, GroupID: &groupID, Group: &Group{ID: groupID, RateMultiplier: 1.5}},
		User:    &User{ID: 2008, Balance: 10},
		Account: &Account{ID: 3008},
	})

	require.NotNil(t, payload.OpenAIBilling)
	require.False(t, payload.OpenAIBilling.BillingIncomplete)
	require.Len(t, payload.OpenAIBilling.Components, 2)
	require.Equal(t, "main", payload.OpenAIBilling.Components[0].Component.Kind)
	require.Equal(t, "gpt-main", payload.OpenAIBilling.Components[0].Component.Model)
	require.Equal(t, "image", payload.OpenAIBilling.Components[1].Component.Kind)
	require.Equal(t, "gpt-image-2", payload.OpenAIBilling.Components[1].Component.Model)

	expectedTotal := 100*1e-6 + 30*2e-6 + 5*3e-6 + 10*0.5e-6 + 20*8e-6 + 40*30e-6
	expectedActual := expectedTotal * 1.5
	require.InDelta(t, expectedTotal, payload.UsageLog.TotalCost, 1e-12)
	require.InDelta(t, expectedActual, payload.UsageLog.ActualCost, 1e-12)
	require.Equal(t, 20, payload.UsageLog.ImageInputTokens)
	require.InDelta(t, 20*8e-6, payload.UsageLog.ImageInputCost, 1e-12)
	require.Equal(t, 40, payload.UsageLog.ImageOutputTokens)
	require.InDelta(t, 40*30e-6, payload.UsageLog.ImageOutputCost, 1e-12)
	require.False(t, payload.UsageLog.BillingIncomplete)
	require.NotNil(t, payload.UsageLog.BillingMode)
	require.Equal(t, string(BillingModeToken), *payload.UsageLog.BillingMode)
	require.InDelta(t, expectedActual, payload.BillingCommand.BalanceCost, 1e-12)
}

func TestOpenAIGatewayServiceBuildUsageFact_OpenAIImageCostIgnoresImageCount(t *testing.T) {
	groupID := int64(502)
	svc := newOpenAIRecordUsageServiceWithOpenAITokenPricingForTest(t, groupID)
	build := func(imageCount int) UsageFactPayload {
		return buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
			Result: &OpenAIForwardResult{
				RequestID:         "resp_openai_image_count",
				Model:             "gpt-main",
				MainBillingModel:  "gpt-main",
				ImageBillingModel: "gpt-image-2",
				ImageCount:        imageCount,
				ImageSize:         "1K",
				Duration:          time.Second,
				Usage: OpenAIUsage{
					InputTokens:       10,
					OutputTokens:      5,
					ImageOutputTokens: 3,
				},
			},
			APIKey:  &APIKey{ID: 1009, GroupID: &groupID, Group: &Group{ID: groupID, RateMultiplier: 1.5}},
			User:    &User{ID: 2009, Balance: 10},
			Account: &Account{ID: 3009},
		})
	}

	one := build(1)
	four := build(4)

	require.Equal(t, 1, one.UsageLog.ImageCount)
	require.Equal(t, 4, four.UsageLog.ImageCount)
	require.InDelta(t, one.UsageLog.TotalCost, four.UsageLog.TotalCost, 1e-12)
	require.InDelta(t, one.UsageLog.ActualCost, four.UsageLog.ActualCost, 1e-12)
	require.InDelta(t, one.BillingCommand.BalanceCost, four.BillingCommand.BalanceCost, 1e-12)
}

func newOpenAIImageChannelPricingResolverForTest(t *testing.T, groupID int64, model string, price float64) *ModelPricingResolver {
	t.Helper()
	cache := newEmptyChannelCache()
	cache.pricingByGroupModel[channelModelKey{groupID: groupID, model: model}] = &ChannelModelPricing{
		BillingMode:     BillingModeImage,
		PerRequestPrice: &price,
	}
	cache.channelByGroupID[groupID] = &Channel{ID: groupID, Status: StatusActive}
	cache.groupPlatform[groupID] = ""
	cache.loadedAt = time.Now()
	cs := &ChannelService{}
	cs.cache.Store(cache)
	return NewModelPricingResolver(cs, NewBillingService(&config.Config{}, nil))
}

func newOpenAIRecordUsageServiceWithOpenAITokenPricingForTest(t *testing.T, groupID int64) *openAIRecordUsageTestService {
	t.Helper()
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(
		&openAIRecordUsageLogRepoStub{},
		&openAIRecordUsageBillingRepoStub{},
		&openAIRecordUsageUserRepoStub{},
		&openAIRecordUsageSubRepoStub{},
		nil,
	)
	cache := newEmptyChannelCache()
	cache.pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformOpenAI, model: "gpt-main"}] = &ChannelModelPricing{
		BillingMode:     BillingModeToken,
		InputPrice:      testPtrFloat64(1e-6),
		OutputPrice:     testPtrFloat64(2e-6),
		CacheWritePrice: testPtrFloat64(3e-6),
		CacheReadPrice:  testPtrFloat64(0.5e-6),
	}
	cache.pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformOpenAI, model: "gpt-image-2"}] = &ChannelModelPricing{
		BillingMode:      BillingModeToken,
		InputPrice:       testPtrFloat64(8e-6),
		OutputPrice:      testPtrFloat64(9e-6),
		ImageOutputPrice: testPtrFloat64(30e-6),
	}
	cache.channelByGroupID[groupID] = &Channel{ID: groupID, Status: StatusActive}
	cache.groupPlatform[groupID] = PlatformOpenAI
	cache.loadedAt = time.Now()
	channelService := &ChannelService{}
	channelService.cache.Store(cache)
	svc.resolver = NewModelPricingResolver(channelService, svc.billingService)
	return svc
}

func TestGatewayServiceCalculateRecordUsageCost_ChannelImageBillingUsesImageCount(t *testing.T) {
	groupID := int64(126)
	billingService := NewBillingService(&config.Config{}, nil)
	svc := &GatewayService{
		billingService: billingService,
		resolver:       newOpenAIImageChannelPricingResolverForTest(t, groupID, "gemini-image", 0.25),
	}

	cost := svc.calculateRecordUsageCost(
		context.Background(),
		&ForwardResult{Model: "gemini-image", ImageCount: 2, ImageSize: "1K"},
		&APIKey{GroupID: i64p(groupID), Group: &Group{ID: groupID}},
		"gemini-image",
		0.15,
		nil,
	)

	require.NotNil(t, cost)
	require.Equal(t, string(BillingModeImage), cost.BillingMode)
	require.InDelta(t, 0.5, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.075, cost.ActualCost, 1e-12)
}

func TestGatewayServiceCalculateRecordUsageCost_ChannelImageBillingUsesSizeTier(t *testing.T) {
	groupID := int64(127)
	defaultPrice := 0.10
	price4K := 0.40
	cache := newEmptyChannelCache()
	cache.pricingByGroupModel[channelModelKey{groupID: groupID, model: "gemini-image"}] = &ChannelModelPricing{
		BillingMode:     BillingModeImage,
		PerRequestPrice: &defaultPrice,
		Intervals: []PricingInterval{{
			TierLabel:       "4K",
			PerRequestPrice: &price4K,
		}},
	}
	cache.channelByGroupID[groupID] = &Channel{ID: groupID, Status: StatusActive}
	cache.loadedAt = time.Now()
	channelService := &ChannelService{}
	channelService.cache.Store(cache)

	svc := &GatewayService{
		billingService: NewBillingService(&config.Config{}, nil),
		resolver:       NewModelPricingResolver(channelService, NewBillingService(&config.Config{}, nil)),
	}

	cost := svc.calculateRecordUsageCost(
		context.Background(),
		&ForwardResult{Model: "gemini-image", ImageCount: 2, ImageSize: "4K"},
		&APIKey{GroupID: i64p(groupID), Group: &Group{ID: groupID}},
		"gemini-image",
		1.0,
		nil,
	)

	require.NotNil(t, cost)
	require.Equal(t, string(BillingModeImage), cost.BillingMode)
	require.InDelta(t, 0.80, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.80, cost.ActualCost, 1e-12)
}

func TestOpenAIGatewayServiceBuildUsageFact_MarksCyberRequestType(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, &openAIUserGroupRateRepoStub{})

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		CyberBlocked: true,
		Result: &OpenAIForwardResult{
			Model:    "gpt-5",
			Duration: time.Second,
			Usage:    OpenAIUsage{InputTokens: 100, OutputTokens: 0},
		},
		APIKey:  &APIKey{ID: 2, Group: &Group{RateMultiplier: 1}},
		User:    &User{ID: 1},
		Account: &Account{ID: 3},
	})

	require.Equal(t, RequestTypeCyberBlocked, payload.UsageLog.RequestType)
	require.Equal(t, 100, payload.UsageLog.InputTokens, "计费 token 不变")
}

func TestGatewayServiceCalculateRecordUsageCost_ChannelImageBillingNormalizesMissingSizeTier(t *testing.T) {
	groupID := int64(128)
	defaultPrice := 0.10
	price2K := 0.22
	cache := newEmptyChannelCache()
	cache.pricingByGroupModel[channelModelKey{groupID: groupID, model: "gemini-image"}] = &ChannelModelPricing{
		BillingMode:     BillingModeImage,
		PerRequestPrice: &defaultPrice,
		Intervals: []PricingInterval{{
			TierLabel:       "2K",
			PerRequestPrice: &price2K,
		}},
	}
	cache.channelByGroupID[groupID] = &Channel{ID: groupID, Status: StatusActive}
	cache.loadedAt = time.Now()
	channelService := &ChannelService{}
	channelService.cache.Store(cache)

	svc := &GatewayService{
		billingService: NewBillingService(&config.Config{}, nil),
		resolver:       NewModelPricingResolver(channelService, NewBillingService(&config.Config{}, nil)),
	}

	cost := svc.calculateRecordUsageCost(
		context.Background(),
		&ForwardResult{Model: "gemini-image", ImageCount: 2, ImageSize: ""},
		&APIKey{GroupID: i64p(groupID), Group: &Group{ID: groupID}},
		"gemini-image",
		1.0,
		nil,
	)

	require.NotNil(t, cost)
	require.Equal(t, string(BillingModeImage), cost.BillingMode)
	require.InDelta(t, 0.44, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.44, cost.ActualCost, 1e-12)
}
