package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	RequestID                  string
	RequestFingerprint         string
	Model                      string
	ImageModel                 string
	GroupID                    *int64
	ServiceTier                string
	RateMultiplier             float64
	Body                       []byte
	AvailableUSD               float64
	ExplicitMaxOutputTokens    *int
	OutputLimitField           string
	ImageInputTokenUpperBound  int
	ImageOutputTokenUpperBound int
	DoNotClampOutputLimit      bool
}

type OpenAITrafficCreditBudget struct {
	Body                       []byte
	InputTokenUpperBound       int
	EffectiveMaxOutputTokens   int
	ImageInputTokenUpperBound  int
	ImageOutputTokenUpperBound int
	ReserveUSD                 float64
	PricingSnapshot            json.RawMessage
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
	if e == nil || e.billingService == nil || !isOpenAITrafficBudgetModel(input.Model) {
		return nil, ErrBillingPreauthUnavailable
	}
	if !input.DoNotClampOutputLimit && !gjson.ValidBytes(input.Body) {
		return nil, ErrBillingPreauthUnavailable
	}
	if input.RateMultiplier == 0 {
		input.RateMultiplier = 1
	}
	availableUSD := roundTrafficCreditAmount(input.AvailableUSD)
	if availableUSD+1e-10 < e.minimumReserveUSD {
		return nil, ErrTrafficCreditInsufficient
	}
	if input.DoNotClampOutputLimit {
		return e.buildBudget(input, append([]byte(nil), input.Body...), 0, availableUSD)
	}

	outputLimitField, err := normalizeOpenAIOutputLimitField(input.OutputLimitField)
	if err != nil {
		return nil, err
	}
	explicitLimit, hasExplicit, err := resolveExplicitOpenAIOutputLimit(input)
	if err != nil {
		return nil, err
	}
	if hasExplicit {
		return e.buildBudget(input, append([]byte(nil), input.Body...), explicitLimit, availableUSD)
	}

	minimumBody, err := sjson.SetBytes(input.Body, outputLimitField, e.minimumOutputTokens)
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
		body, setErr := sjson.SetBytes(input.Body, outputLimitField, mid)
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
	finalBody, err := sjson.SetBytes(input.Body, outputLimitField, best)
	if err != nil {
		return nil, ErrBillingPreauthUnavailable
	}
	return e.buildBudget(input, finalBody, best, availableUSD)
}

func isOpenAITrafficBudgetModel(model string) bool {
	model = strings.TrimSpace(model)
	return normalizeKnownOpenAICodexModel(model) != "" || isOpenAIImageGenerationModel(model)
}

func normalizeOpenAIOutputLimitField(field string) (string, error) {
	switch strings.TrimSpace(field) {
	case "":
		return "max_output_tokens", nil
	case "max_output_tokens", "max_completion_tokens", "max_tokens":
		return strings.TrimSpace(field), nil
	default:
		return "", ErrBillingPreauthUnavailable
	}
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
	maxTokens := gjson.GetBytes(input.Body, "max_tokens")
	limit := 0
	found := false
	for _, result := range []gjson.Result{maxOutput, maxCompletion, maxTokens} {
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
	inputTokens := estimateOpenAIRequestTextTokenUpperBound(body)
	mainTokens := UsageTokens{InputTokens: inputTokens, OutputTokens: outputTokens}
	mainCost, err := e.billingService.CalculateCostWithServiceTier(
		input.Model,
		mainTokens,
		input.RateMultiplier,
		input.ServiceTier,
	)
	mainPricing, pricingErr := e.billingService.GetModelPricing(input.Model)
	if err == nil {
		err = pricingErr
	}
	if err != nil || mainPricing == nil {
		return nil, ErrBillingPreauthUnavailable
	}
	cost := mainCost.ActualCost
	imageModel := resolveOpenAITrafficBudgetImageModel(input.Model, input.ImageModel)
	var imagePricing *ModelPricing
	var imageCost *CostBreakdown
	if input.ImageInputTokenUpperBound > 0 || input.ImageOutputTokenUpperBound > 0 {
		imageTokens := UsageTokens{
			InputTokens:       maxInt(input.ImageInputTokenUpperBound, 0),
			ImageInputTokens:  maxInt(input.ImageInputTokenUpperBound, 0),
			OutputTokens:      maxInt(input.ImageOutputTokenUpperBound, 0),
			ImageOutputTokens: maxInt(input.ImageOutputTokenUpperBound, 0),
		}
		imageCost, err = e.billingService.CalculateCostWithServiceTier(
			imageModel,
			imageTokens,
			input.RateMultiplier,
			input.ServiceTier,
		)
		imagePricing, pricingErr = e.billingService.GetModelPricing(imageModel)
		if err == nil {
			err = pricingErr
		}
		if err != nil || imagePricing == nil {
			return nil, ErrBillingPreauthUnavailable
		}
		cost += imageCost.ActualCost
	}
	reserveUSD := ceilTrafficCreditUSD(math.Max(cost, e.minimumReserveUSD))
	if reserveUSD > availableUSD+1e-10 {
		return nil, ErrTrafficCreditInsufficient
	}
	bodySHA := sha256.Sum256(body)
	snapshot, err := json.Marshal(map[string]any{
		"model":                          strings.TrimSpace(input.Model),
		"image_model":                    imageModel,
		"group_id":                       input.GroupID,
		"request_id":                     strings.TrimSpace(input.RequestID),
		"request_fingerprint":            strings.TrimSpace(input.RequestFingerprint),
		"request_body_sha256":            hex.EncodeToString(bodySHA[:]),
		"service_tier":                   strings.TrimSpace(input.ServiceTier),
		"rate_multiplier":                input.RateMultiplier,
		"output_limit_field":             strings.TrimSpace(input.OutputLimitField),
		"input_token_upper_bound":        inputTokens,
		"effective_max_output_tokens":    outputTokens,
		"image_input_token_upper_bound":  maxInt(input.ImageInputTokenUpperBound, 0),
		"image_output_token_upper_bound": maxInt(input.ImageOutputTokenUpperBound, 0),
		"reserve_usd":                    reserveUSD,
		"input_price_per_token":          mainPricing.InputPricePerToken,
		"output_price_per_token":         mainPricing.OutputPricePerToken,
		"cache_creation_price":           mainPricing.CacheCreationPricePerToken,
		"cache_creation_5m_price":        mainPricing.CacheCreation5mPrice,
		"cache_creation_1h_price":        mainPricing.CacheCreation1hPrice,
		"image_input_price_per_token":    openAITrafficBudgetImageInputPrice(imagePricing),
		"image_output_price_per_token":   openAITrafficBudgetImageOutputPrice(imagePricing),
	})
	if err != nil {
		return nil, ErrBillingPreauthUnavailable
	}
	return &OpenAITrafficCreditBudget{
		Body:                       body,
		InputTokenUpperBound:       inputTokens,
		EffectiveMaxOutputTokens:   outputTokens,
		ImageInputTokenUpperBound:  maxInt(input.ImageInputTokenUpperBound, 0),
		ImageOutputTokenUpperBound: maxInt(input.ImageOutputTokenUpperBound, 0),
		ReserveUSD:                 reserveUSD,
		PricingSnapshot:            snapshot,
	}, nil
}

func estimateOpenAIRequestTextTokenUpperBound(body []byte) int {
	if len(body) == 0 {
		return 0
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return len(body)
	}
	return estimateOpenAIJSONTextTokenUpperBound(payload, "", false)
}

func estimateOpenAIJSONTextTokenUpperBound(value any, key string, insideImageRef bool) int {
	switch v := value.(type) {
	case map[string]any:
		imageRef := insideImageRef || isOpenAIImageReferenceObject(v)
		total := 0
		for childKey, child := range v {
			childImageRef := imageRef || strings.EqualFold(strings.TrimSpace(childKey), "image_url")
			total += estimateOpenAIJSONTextTokenUpperBound(child, childKey, childImageRef)
		}
		return total
	case []any:
		total := 0
		for _, item := range v {
			total += estimateOpenAIJSONTextTokenUpperBound(item, key, insideImageRef)
		}
		return total
	case string:
		if insideImageRef || shouldSkipOpenAIStringInTextTokenBudget(key, v) {
			return 0
		}
		return len(v)
	default:
		return 0
	}
}

func isOpenAIImageReferenceObject(value map[string]any) bool {
	typeValue, _ := value["type"].(string)
	switch strings.TrimSpace(typeValue) {
	case "input_image", "image_url":
		return true
	}
	_, hasImageURL := value["image_url"]
	return hasImageURL
}

func shouldSkipOpenAIStringInTextTokenBudget(key string, value string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "b64_json" || k == "file_id" {
		return true
	}
	return k == "url" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "data:image/")
}

func resolveOpenAITrafficBudgetImageModel(mainModel, imageModel string) string {
	if trimmed := strings.TrimSpace(imageModel); trimmed != "" {
		return trimmed
	}
	if isOpenAIImageGenerationModel(mainModel) {
		return strings.TrimSpace(mainModel)
	}
	return "gpt-image-2"
}

func openAITrafficBudgetImageInputPrice(pricing *ModelPricing) float64 {
	if pricing == nil {
		return 0
	}
	if pricing.ImageInputPricePerToken > 0 {
		return pricing.ImageInputPricePerToken
	}
	return pricing.InputPricePerToken
}

func openAITrafficBudgetImageOutputPrice(pricing *ModelPricing) float64 {
	if pricing == nil {
		return 0
	}
	if pricing.ImageOutputPricePerToken > 0 || pricing.ImageOutputPriceExplicit {
		return pricing.ImageOutputPricePerToken
	}
	return pricing.OutputPricePerToken
}

func ceilTrafficCreditUSD(value float64) float64 {
	return math.Ceil(value*1e10-1e-12) / 1e10
}
