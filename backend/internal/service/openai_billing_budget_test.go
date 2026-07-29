//go:build unit

package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestFitOpenAIBillingBudgetCapsAtTwoUSD(t *testing.T) {
	plan := testTextBudgetPlan(3.00, false)

	_, err := FitOpenAIBillingBudget(plan, 5.00, BillingAuthorizationReserveFull)

	require.ErrorIs(t, err, ErrOpenAIBillingBudgetExceedsHardCap)
}

func TestFitOpenAIBillingBudgetUsesExactOneDollarAvailability(t *testing.T) {
	plan := testAdjustableTextBudgetPlan(0.10, 0.00003, 256, 128000)

	got, err := FitOpenAIBillingBudget(plan, 1.00, BillingAuthorizationReserveFit)

	require.NoError(t, err)
	require.LessOrEqual(t, got.ReserveUSD, 1.00)
	require.GreaterOrEqual(t, got.EffectiveMaxOutputTokens, 256)
	require.Less(t, got.EffectiveMaxOutputTokens, 128000)
	require.Equal(t, 30000, effectiveOutputLimit(t, got.EffectiveBody, "max_output_tokens"))
}

func TestFitOpenAIBillingBudgetUsesExactFortySevenCentsAvailability(t *testing.T) {
	plan := testAdjustableTextBudgetPlan(0.10, 0.00003, 256, 128000)

	got, err := FitOpenAIBillingBudget(plan, 0.47, BillingAuthorizationReserveFit)

	require.NoError(t, err)
	require.LessOrEqual(t, got.ReserveUSD, 0.47)
	require.GreaterOrEqual(t, got.EffectiveMaxOutputTokens, 256)
}

func TestFitOpenAIBillingBudgetDoesNotRewriteExplicitLimit(t *testing.T) {
	plan := testTextBudgetPlan(1.20, true)

	_, err := FitOpenAIBillingBudget(plan, 1.00, BillingAuthorizationReserveFit)

	require.ErrorIs(t, err, ErrOpenAIBillingBudgetInsufficient)
}

func TestFitOpenAIBillingBudgetRejectsInputCostAboveAvailability(t *testing.T) {
	plan := testAdjustableTextBudgetPlan(0.48, 0.00003, 256, 128000)

	_, err := FitOpenAIBillingBudget(plan, 0.47, BillingAuthorizationReserveFit)

	require.ErrorIs(t, err, ErrOpenAIBillingBudgetInsufficient)
}

func TestFitOpenAIBillingBudgetRejectsWhenMinimumOutputCannotFit(t *testing.T) {
	plan := testAdjustableTextBudgetPlan(0.463, 0.00003, 256, 128000)

	_, err := FitOpenAIBillingBudget(plan, 0.47, BillingAuthorizationReserveFit)

	require.ErrorIs(t, err, ErrOpenAIBillingBudgetInsufficient)
}

func TestFitOpenAIBillingBudgetEmbeddingsOnlyReserveInputCost(t *testing.T) {
	plan := OpenAIBillingBudgetPlan{
		OriginalBody:  []byte(`{"model":"text-embedding-3-large","input":"hello"}`),
		FixedInputUSD: 0.12,
	}

	got, err := FitOpenAIBillingBudget(plan, 0.12, BillingAuthorizationReserveFull)

	require.NoError(t, err)
	require.InDelta(t, 0.12, got.ReserveUSD, 1e-12)
	require.Equal(t, plan.OriginalBody, got.EffectiveBody)
}

func TestOpenAIBillingPricingSnapshotUsesEffectivePricesOnce(t *testing.T) {
	service := newOpenAIBillingPricingTestService(t)

	tests := []struct {
		model               string
		groupRateMultiplier float64
		inputUSDPerToken    float64
		outputUSDPerToken   float64
		imageInputUSD       float64
		imageOutputUSD      float64
	}{
		{"gpt-5.5", 1, 5.0 / 1_000_000, 30.0 / 1_000_000, 0, 0},
		{"gpt-5.6-sol", 1.25, 6.25 / 1_000_000, 37.5 / 1_000_000, 0, 0},
		{"gpt-image-2", 1.25, 6.25 / 1_000_000, 12.5 / 1_000_000, 10.0 / 1_000_000, 37.5 / 1_000_000},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			raw, err := service.OpenAIBillingPricingSnapshot(tt.model, "standard", tt.groupRateMultiplier)
			require.NoError(t, err)

			var snapshot OpenAIBillingPricingSnapshot
			require.NoError(t, json.Unmarshal(raw, &snapshot))
			require.Equal(t, "test-pricing-hash", snapshot.SourceHash)
			require.Equal(t, tt.model, snapshot.Model)
			require.Equal(t, "standard", snapshot.ServiceTier)
			require.InDelta(t, tt.groupRateMultiplier, snapshot.GroupRateMultiplier, 1e-12)
			require.InDelta(t, tt.inputUSDPerToken, snapshot.InputUSDPerToken, 1e-12)
			require.InDelta(t, tt.outputUSDPerToken, snapshot.OutputUSDPerToken, 1e-12)
			require.InDelta(t, tt.imageInputUSD, snapshot.ImageInputUSDPerToken, 1e-12)
			require.InDelta(t, tt.imageOutputUSD, snapshot.ImageOutputUSDPerToken, 1e-12)
		})
	}
}

func TestEstimateOpenAITextInputTokensIncludesToolsAndJSONSchema(t *testing.T) {
	plain, err := EstimateOpenAITextInputTokens([]byte(`{
"model":"gpt-5.6-terra",
"input":[{"role":"user","content":"find a nearby coffee shop"}]
}`))
	require.NoError(t, err)

	withTools, err := EstimateOpenAITextInputTokens([]byte(`{
"model":"gpt-5.6-terra",
"input":[{"role":"user","content":"find a nearby coffee shop"}],
"tools":[{"type":"function","name":"find_coffee_shop","description":"Find nearby coffee shops by city and dietary requirement.","parameters":{"type":"object","properties":{"city":{"type":"string","description":"The city to search."},"dietary_requirement":{"type":"string","enum":["vegan","gluten-free","none"]}},"required":["city"]}}]
}`))
	require.NoError(t, err)

	require.Greater(t, plain, 0)
	require.Greater(t, withTools, plain)
}

func TestEstimateOpenAIAttachmentInputTokensIncludesPDFTextAndPages(t *testing.T) {
	inspection := OpenAIAttachmentInspection{
		Images: []OpenAIImageInput{{Width: 1024, Height: 768, Detail: "high"}},
		PDFs: []OpenAIPDFInspection{{
			TextTokens: 41,
			Pages:      []OpenAIImageInput{{Width: 612, Height: 792, Detail: "high"}, {Width: 612, Height: 792, Detail: "high"}},
		}},
	}

	imageTokens, pdfTokens := EstimateOpenAIAttachmentInputTokens("gpt-5.6-terra", inspection)

	require.Greater(t, imageTokens, 0)
	require.Greater(t, pdfTokens, inspection.PDFs[0].TextTokens)
}

func TestFitOpenAIBillingBudgetReducesImageCountButKeepsAtLeastOne(t *testing.T) {
	plan := OpenAIBillingBudgetPlan{
		OriginalBody:           []byte(`{"model":"gpt-image-2","n":4}`),
		ImageCountField:        "n",
		FixedInputUSD:          0.05,
		ImageOutputUSDPerImage: []float64{0.30, 0.30, 0.30, 0.30},
	}

	got, err := FitOpenAIBillingBudget(plan, 0.70, BillingAuthorizationReserveFit)

	require.NoError(t, err)
	require.Equal(t, 2, got.EffectiveImageCount)
	require.LessOrEqual(t, got.ReserveUSD, 0.70)
	require.Equal(t, 2, effectiveOutputLimit(t, got.EffectiveBody, "n"))
}

func testTextBudgetPlan(fullCostUSD float64, explicit bool) OpenAIBillingBudgetPlan {
	return OpenAIBillingBudgetPlan{
		OriginalBody:          []byte(`{"model":"gpt-5.6-terra"}`),
		OutputLimitField:      "max_output_tokens",
		ExplicitOutputLimit:   explicit,
		RequestedOutputTokens: 1000,
		DefaultOutputTokens:   1000,
		MinimumOutputTokens:   OpenAIBillingMinOutputTokens,
		FixedInputUSD:         0.20,
		OutputUSDPerToken:     (fullCostUSD - 0.20) / 1000,
	}
}

func testAdjustableTextBudgetPlan(fixedInputUSD, outputUSDPerToken float64, minimumOutputTokens, requestedOutputTokens int) OpenAIBillingBudgetPlan {
	return OpenAIBillingBudgetPlan{
		OriginalBody:          []byte(`{"model":"gpt-5.6-terra","input":"hello"}`),
		OutputLimitField:      "max_output_tokens",
		RequestedOutputTokens: requestedOutputTokens,
		DefaultOutputTokens:   requestedOutputTokens,
		MinimumOutputTokens:   minimumOutputTokens,
		FixedInputUSD:         fixedInputUSD,
		OutputUSDPerToken:     outputUSDPerToken,
	}
}

func effectiveOutputLimit(t *testing.T, body []byte, field string) int {
	t.Helper()

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	value, ok := decoded[field].(float64)
	require.True(t, ok)
	return int(value)
}

func newOpenAIBillingPricingTestService(t *testing.T) *BillingService {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)
	pricingService := &PricingService{cfg: &config.Config{}, localHash: "test-pricing-hash"}
	pricingService.pricingData, err = pricingService.parsePricingData(raw)
	require.NoError(t, err)
	return NewBillingService(&config.Config{}, pricingService)
}
