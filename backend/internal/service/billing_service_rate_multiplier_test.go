//go:build unit

package service

import (
	"testing"

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
