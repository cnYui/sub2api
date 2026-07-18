//go:build unit

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAITrafficCreditBudget_RejectsTinyResidual(t *testing.T) {
	estimator := newTestTrafficBudgetEstimator(0.01, 256, 8192)
	_, err := estimator.Estimate(context.Background(), OpenAITrafficBudgetInput{
		Model: "gpt-5.6-sol", Body: []byte(`{"input":"hello"}`), AvailableUSD: 0.00111155,
	})
	require.ErrorIs(t, err, ErrTrafficCreditInsufficient)
}

func TestOpenAITrafficCreditBudget_RejectsExplicitUnaffordableLimit(t *testing.T) {
	maxOutput := 8192
	estimator := newTestTrafficBudgetEstimator(0.01, 256, 8192)
	_, err := estimator.Estimate(context.Background(), OpenAITrafficBudgetInput{
		Model: "gpt-5.6-sol", Body: []byte(`{"input":"hello","max_output_tokens":8192}`),
		ExplicitMaxOutputTokens: &maxOutput, AvailableUSD: 0.02,
	})
	require.ErrorIs(t, err, ErrTrafficCreditInsufficient)
}

func TestOpenAITrafficCreditBudget_InjectsAffordableLimitWhenMissing(t *testing.T) {
	estimator := newTestTrafficBudgetEstimator(0.01, 256, 8192)
	got, err := estimator.Estimate(context.Background(), OpenAITrafficBudgetInput{
		Model: "gpt-5.6-sol", Body: []byte(`{"input":"hello"}`), AvailableUSD: 0.2,
	})

	require.NoError(t, err)
	require.GreaterOrEqual(t, got.EffectiveMaxOutputTokens, 256)
	require.LessOrEqual(t, got.ReserveUSD, 0.2)
	require.True(t, gjson.GetBytes(got.Body, "max_output_tokens").Exists())
}

func TestOpenAITrafficCreditBudget_InjectsConfiguredOutputLimitField(t *testing.T) {
	estimator := newTestTrafficBudgetEstimator(0.01, 256, 8192)
	got, err := estimator.Estimate(context.Background(), OpenAITrafficBudgetInput{
		Model:            "gpt-5.6-sol",
		Body:             []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
		AvailableUSD:     0.2,
		OutputLimitField: "max_tokens",
	})

	require.NoError(t, err)
	require.True(t, gjson.GetBytes(got.Body, "max_tokens").Exists())
	require.False(t, gjson.GetBytes(got.Body, "max_output_tokens").Exists())
}

func TestOpenAITrafficCreditBudget_DoesNotPriceBase64ImageBytesAsTextTokens(t *testing.T) {
	estimator := newTestTrafficBudgetEstimator(0.01, 256, 8192)
	base64Image := strings.Repeat("A", 1_000_000)
	body := []byte(`{"input":[{"role":"user","content":[{"type":"input_text","text":"draw a cat"},{"type":"input_image","image_url":"data:image/png;base64,` + base64Image + `"}]}]}`)

	got, err := estimator.Estimate(context.Background(), OpenAITrafficBudgetInput{
		Model:        "gpt-5.6-terra",
		Body:         body,
		AvailableUSD: 0.05,
	})

	require.NoError(t, err)
	require.Less(t, got.InputTokenUpperBound, len(body)/10)
	require.LessOrEqual(t, got.ReserveUSD, 0.05)
}

func TestOpenAITrafficCreditBudget_UsesExplicitMaxTokens(t *testing.T) {
	estimator := newTestTrafficBudgetEstimator(0.01, 256, 8192)
	got, err := estimator.Estimate(context.Background(), OpenAITrafficBudgetInput{
		Model:        "gpt-5.6-sol",
		Body:         []byte(`{"messages":[{"role":"user","content":"hello"}],"max_tokens":512}`),
		AvailableUSD: 0.2,
	})

	require.NoError(t, err)
	require.Equal(t, 512, got.EffectiveMaxOutputTokens)
	require.Equal(t, int64(512), gjson.GetBytes(got.Body, "max_tokens").Int())
}

func TestOpenAITrafficCreditBudget_ImageUsesImageTokenBoundsWithoutOutputClamp(t *testing.T) {
	estimator := newTestTrafficBudgetEstimatorWithImagePricing(0.01, 256, 8192)
	got, err := estimator.Estimate(context.Background(), OpenAITrafficBudgetInput{
		Model:                      "gpt-5.6-sol",
		ImageModel:                 "gpt-image-2",
		Body:                       []byte(`{"model":"gpt-image-2","prompt":"draw a cat","n":1}`),
		AvailableUSD:               1,
		ImageInputTokenUpperBound:  7,
		ImageOutputTokenUpperBound: 1756,
		DoNotClampOutputLimit:      true,
	})

	require.NoError(t, err)
	require.False(t, gjson.GetBytes(got.Body, "max_output_tokens").Exists())
	require.Equal(t, 7, got.ImageInputTokenUpperBound)
	require.Equal(t, 1756, got.ImageOutputTokenUpperBound)
	require.GreaterOrEqual(t, got.ReserveUSD, 7*8e-6+1756*30e-6)
	require.Equal(t, "gpt-image-2", gjson.GetBytes(got.PricingSnapshot, "image_model").String())
	require.Equal(t, int64(1756), gjson.GetBytes(got.PricingSnapshot, "image_output_token_upper_bound").Int())
	require.InDelta(t, 30e-6, gjson.GetBytes(got.PricingSnapshot, "image_output_price_per_token").Float(), 1e-12)
}

func newTestTrafficBudgetEstimator(minimumReserve float64, minimumOutput, defaultMaxOutput int) *OpenAITrafficCreditBudgetEstimator {
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	return NewOpenAITrafficCreditBudgetEstimator(
		NewBillingService(cfg, nil),
		minimumReserve,
		minimumOutput,
		defaultMaxOutput,
	)
}

func newTestTrafficBudgetEstimatorWithImagePricing(minimumReserve float64, minimumOutput, defaultMaxOutput int) *OpenAITrafficCreditBudgetEstimator {
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	return NewOpenAITrafficCreditBudgetEstimator(
		NewBillingService(cfg, &PricingService{pricingData: map[string]*LiteLLMModelPricing{
			"gpt-image-2": {
				InputCostPerToken:       5e-6,
				InputCostPerImageToken:  8e-6,
				OutputCostPerToken:      10e-6,
				OutputCostPerImageToken: 30e-6,
			},
		}}),
		minimumReserve,
		minimumOutput,
		defaultMaxOutput,
	)
}
