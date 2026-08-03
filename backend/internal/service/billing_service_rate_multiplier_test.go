//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// TestCalculateCost_RateMultiplier_NegativeClampedToZero 锁定负数倍率被
// 钳制为 0（而非历史上的 1.0），避免配置异常导致静默按标准价扣费。
func TestCalculateCost_RateMultiplier_NegativeClampedToZero(t *testing.T) {
	svc := newTestBillingService()
	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500}

	tests := []struct {
		name       string
		multiplier float64
		wantRatio  float64 // ActualCost / TotalCost
	}{
		{"negative clamped to 0", -1.5, 0},
		{"zero passes through as 0 (defense in depth)", 0, 0},
		{"positive 2x applied", 2.0, 2.0},
		{"positive 0.5x applied", 0.5, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost, err := svc.CalculateCost("claude-sonnet-4", tokens, tt.multiplier)
			require.NoError(t, err)
			require.Greater(t, cost.TotalCost, 0.0, "TotalCost should be non-zero")
			require.InDelta(t, tt.wantRatio*cost.TotalCost, cost.ActualCost, 1e-9)
		})
	}
}

// TestCalculateImageCost_RateMultiplier_NegativeClampedToZero 图片按次计费路径
// 同样遵循"负数 → 0"语义。
func TestCalculateImageCost_RateMultiplier_NegativeClampedToZero(t *testing.T) {
	svc := newTestBillingService()
	price := 0.04
	cfg := &ImagePriceConfig{Price1K: &price}

	tests := []struct {
		name       string
		multiplier float64
		wantRatio  float64
	}{
		{"negative clamped to 0", -0.5, 0},
		{"zero passes through", 0, 0},
		{"positive 3x applied", 3.0, 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := svc.CalculateImageCost("imagen-3", "1K", 2, cfg, tt.multiplier)
			require.NotNil(t, cost)
			require.Greater(t, cost.TotalCost, 0.0)
			require.InDelta(t, tt.wantRatio*cost.TotalCost, cost.ActualCost, 1e-9)
		})
	}
}

// TestFinalBillingMultiplierOnlyChangesActualCost 验证最终倍率覆盖所有计费模式，
// 且不污染 TotalCost 这一基础成本字段。
func TestFinalBillingMultiplierOnlyChangesActualCost(t *testing.T) {
	svc := NewBillingService(&config.Config{
		Billing: config.BillingConfig{FinalMultiplier: 10},
	}, nil)

	tokenCost, err := svc.CalculateCost("claude-sonnet-4", UsageTokens{InputTokens: 1000, OutputTokens: 500}, 1.5)
	require.NoError(t, err)
	require.InDelta(t, 0.0105, tokenCost.TotalCost, 1e-12)
	require.InDelta(t, 0.0105*1.5*10, tokenCost.ActualCost, 1e-12)

	imageCost := svc.CalculateImageCost("gemini-3-pro-image", "2K", 1, nil, 1.2)
	require.InDelta(t, 0.201, imageCost.TotalCost, 1e-12)
	require.InDelta(t, 0.201*1.2*10, imageCost.ActualCost, 1e-12)

	videoCost := svc.CalculateVideoCost("grok-imagine-video", "720p", 2, 3, nil, 0.5)
	require.InDelta(t, 0.07*2*3, videoCost.TotalCost, 1e-12)
	require.InDelta(t, 0.07*2*3*0.5*10, videoCost.ActualCost, 1e-12)

	webSearchCost := svc.CalculateWebSearchCost(2, nil, 0.75)
	require.InDelta(t, 0.02, webSearchCost.TotalCost, 1e-12)
	require.InDelta(t, 0.02*0.75*10, webSearchCost.ActualCost, 1e-12)

	resolver := NewModelPricingResolver(nil, svc)
	groupID := int64(1)
	perRequestCost, err := svc.CalculateCostUnified(CostInput{
		Model:          "claude-sonnet-4",
		GroupID:        &groupID,
		RequestCount:   2,
		RateMultiplier: 0.8,
		Resolver:       resolver,
		Resolved: &ResolvedPricing{
			Mode:                   BillingModePerRequest,
			DefaultPerRequestPrice: 0.04,
		},
	})
	require.NoError(t, err)
	require.InDelta(t, 0.08, perRequestCost.TotalCost, 1e-12)
	require.InDelta(t, 0.08*0.8*10, perRequestCost.ActualCost, 1e-12)
}

func TestGetEstimatedCostExcludesFinalBillingMultiplier(t *testing.T) {
	svc := NewBillingService(&config.Config{
		Billing: config.BillingConfig{FinalMultiplier: 10},
		Default: config.DefaultConfig{RateMultiplier: 1.25},
	}, nil)

	estimate, err := svc.GetEstimatedCost("claude-sonnet-4", 1000, 500)
	require.NoError(t, err)
	require.InDelta(t, 0.0105*1.25, estimate, 1e-12)
}
