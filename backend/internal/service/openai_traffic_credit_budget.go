package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var ErrTrafficCreditInsufficient = errors.New("traffic credit is insufficient for request budget")
var ErrBillingPreauthUnavailable = errors.New("billing preauthorization is unavailable")

type OpenAITrafficBudgetInput struct {
	Model                   string
	GroupID                 *int64
	ServiceTier             string
	RateMultiplier          float64
	Body                    []byte
	AvailableUSD            float64
	ExplicitMaxOutputTokens *int
}

type OpenAITrafficCreditBudget struct {
	Body                     []byte
	InputTokenUpperBound     int
	EffectiveMaxOutputTokens int
	ReserveUSD               float64
	PricingSnapshot          json.RawMessage
}

type OpenAITrafficCreditBudgetEstimator struct {
	billingService         *BillingService
	minimumReserveUSD      float64
	minimumOutputTokens    int
	defaultMaxOutputTokens int
}

func NewOpenAITrafficCreditBudgetEstimator(
	billingService *BillingService,
	minimumReserveUSD float64,
	minimumOutputTokens int,
	defaultMaxOutputTokens int,
) *OpenAITrafficCreditBudgetEstimator {
	return &OpenAITrafficCreditBudgetEstimator{
		billingService:         billingService,
		minimumReserveUSD:      minimumReserveUSD,
		minimumOutputTokens:    minimumOutputTokens,
		defaultMaxOutputTokens: defaultMaxOutputTokens,
	}
}

func (e *OpenAITrafficCreditBudgetEstimator) Estimate(ctx context.Context, input OpenAITrafficBudgetInput) (*OpenAITrafficCreditBudget, error) {
	_ = ctx
	if e == nil || e.billingService == nil || !gjson.ValidBytes(input.Body) || normalizeKnownOpenAICodexModel(input.Model) == "" {
		return nil, ErrBillingPreauthUnavailable
	}
	if input.RateMultiplier == 0 {
		input.RateMultiplier = 1
	}
	availableUSD := roundTrafficCreditAmount(input.AvailableUSD)
	if availableUSD+1e-10 < e.minimumReserveUSD {
		return nil, ErrTrafficCreditInsufficient
	}

	explicitLimit, hasExplicit, err := resolveExplicitOpenAIOutputLimit(input)
	if err != nil {
		return nil, err
	}
	if hasExplicit {
		return e.buildBudget(input, append([]byte(nil), input.Body...), explicitLimit, availableUSD)
	}

	minimumBody, err := sjson.SetBytes(input.Body, "max_output_tokens", e.minimumOutputTokens)
	if err != nil {
		return nil, ErrBillingPreauthUnavailable
	}
	if _, err := e.buildBudget(input, minimumBody, e.minimumOutputTokens, availableUSD); err != nil {
		if errors.Is(err, ErrTrafficCreditInsufficient) {
			return nil, err
		}
		return nil, ErrBillingPreauthUnavailable
	}

	low := e.minimumOutputTokens
	high := e.defaultMaxOutputTokens
	best := low
	for low <= high {
		mid := low + (high-low)/2
		body, setErr := sjson.SetBytes(input.Body, "max_output_tokens", mid)
		if setErr != nil {
			return nil, ErrBillingPreauthUnavailable
		}
		_, estimateErr := e.buildBudget(input, body, mid, availableUSD)
		if estimateErr == nil {
			best = mid
			low = mid + 1
			continue
		}
		if !errors.Is(estimateErr, ErrTrafficCreditInsufficient) {
			return nil, estimateErr
		}
		high = mid - 1
	}
	finalBody, err := sjson.SetBytes(input.Body, "max_output_tokens", best)
	if err != nil {
		return nil, ErrBillingPreauthUnavailable
	}
	return e.buildBudget(input, finalBody, best, availableUSD)
}

func resolveExplicitOpenAIOutputLimit(input OpenAITrafficBudgetInput) (int, bool, error) {
	if input.ExplicitMaxOutputTokens != nil {
		if *input.ExplicitMaxOutputTokens <= 0 {
			return 0, false, ErrBillingPreauthUnavailable
		}
		return *input.ExplicitMaxOutputTokens, true, nil
	}
	maxOutput := gjson.GetBytes(input.Body, "max_output_tokens")
	maxCompletion := gjson.GetBytes(input.Body, "max_completion_tokens")
	limit := 0
	found := false
	for _, result := range []gjson.Result{maxOutput, maxCompletion} {
		if !result.Exists() {
			continue
		}
		if result.Type != gjson.Number || result.Int() <= 0 || result.Float() != float64(result.Int()) {
			return 0, false, ErrBillingPreauthUnavailable
		}
		if int(result.Int()) > limit {
			limit = int(result.Int())
		}
		found = true
	}
	return limit, found, nil
}

func (e *OpenAITrafficCreditBudgetEstimator) buildBudget(
	input OpenAITrafficBudgetInput,
	body []byte,
	outputTokens int,
	availableUSD float64,
) (*OpenAITrafficCreditBudget, error) {
	inputTokens := len(body)
	cost, pricing, err := e.billingService.EstimateMaximumTokenCost(
		input.Model,
		inputTokens,
		outputTokens,
		input.RateMultiplier,
		input.ServiceTier,
	)
	if err != nil || pricing == nil {
		return nil, ErrBillingPreauthUnavailable
	}
	reserveUSD := ceilTrafficCreditUSD(math.Max(cost, e.minimumReserveUSD))
	if reserveUSD > availableUSD+1e-10 {
		return nil, ErrTrafficCreditInsufficient
	}
	snapshot, err := json.Marshal(map[string]any{
		"model":                       strings.TrimSpace(input.Model),
		"group_id":                    input.GroupID,
		"service_tier":                strings.TrimSpace(input.ServiceTier),
		"rate_multiplier":             input.RateMultiplier,
		"input_token_upper_bound":     inputTokens,
		"effective_max_output_tokens": outputTokens,
		"reserve_usd":                 reserveUSD,
		"input_price_per_token":       pricing.InputPricePerToken,
		"output_price_per_token":      pricing.OutputPricePerToken,
		"cache_creation_price":        pricing.CacheCreationPricePerToken,
		"cache_creation_5m_price":     pricing.CacheCreation5mPrice,
		"cache_creation_1h_price":     pricing.CacheCreation1hPrice,
	})
	if err != nil {
		return nil, ErrBillingPreauthUnavailable
	}
	return &OpenAITrafficCreditBudget{
		Body:                     body,
		InputTokenUpperBound:     inputTokens,
		EffectiveMaxOutputTokens: outputTokens,
		ReserveUSD:               reserveUSD,
		PricingSnapshot:          snapshot,
	}, nil
}

func ceilTrafficCreditUSD(value float64) float64 {
	return math.Ceil(value*1e10-1e-12) / 1e10
}
