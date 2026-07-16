//go:build unit

package service

import (
	"context"
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
