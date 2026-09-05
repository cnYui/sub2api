//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// GPT-6 Astra 官方定价（2026-09-04 发布，https://developers.openai.com/api/docs/models/gpt-6-astra）：
// 输入 $10 / 缓存读取 $1 / 缓存写入 $12.50 / 输出 $50，每百万 token；
// 超过 272K 输入的整次请求按 2x 输入与缓存、1.5x 输出计价。
func TestGetModelPricing_GPT6Astra(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := svc.GetModelPricing("gpt-6-astra")
	require.NoError(t, err)
	require.NotNil(t, pricing)

	require.InDelta(t, 10e-6, pricing.InputPricePerToken, 1e-12, "输入 $10/M")
	require.InDelta(t, 50e-6, pricing.OutputPricePerToken, 1e-12, "输出 $50/M")
	require.InDelta(t, 1e-6, pricing.CacheReadPricePerToken, 1e-12, "缓存读取 $1/M")
	require.InDelta(t, 12.5e-6, pricing.CacheCreationPricePerToken, 1e-12, "缓存写入 $12.50/M")

	// priority tier 为标准价的 2 倍，与 GPT-5.6 全系一致。
	require.InDelta(t, 20e-6, pricing.InputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 100e-6, pricing.OutputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 2e-6, pricing.CacheReadPricePerTokenPriority, 1e-12)
	require.InDelta(t, 25e-6, pricing.CacheCreationPricePerTokenPriority, 1e-12)

	// 缓存写入必须正好是输入价的 1.25 倍（OpenAI 全系惯例）。
	require.InDelta(t, pricing.InputPricePerToken*1.25, pricing.CacheCreationPricePerToken, 1e-12)

	require.Equal(t, 272000, pricing.LongContextInputThreshold)
	require.InDelta(t, 2.0, pricing.LongContextInputMultiplier, 1e-12)
	require.InDelta(t, 1.5, pricing.LongContextOutputMultiplier, 1e-12)
}

// 上游中转站可能用带后缀或缺连字符的写法暴露模型，这些都必须解析到同一份价格，
// 否则会重演「模型能调用但不计费」。
func TestGetModelPricing_GPT6AstraAliases(t *testing.T) {
	svc := newTestBillingService()

	aliases := []string{
		"gpt-6-astra",
		"GPT-6-Astra",
		"gpt6-astra",
		"gpt-6-astra-high",
		"gpt-6-astra-2026-09-04",
		"openai/gpt-6-astra",
		"gpt-6",
		"gpt6",
	}

	for _, model := range aliases {
		pricing, err := svc.GetModelPricing(model)
		require.NoError(t, err, "模型 %s", model)
		require.NotNil(t, pricing, "模型 %s 必须有价格，否则不会计费", model)
		require.InDelta(t, 10e-6, pricing.InputPricePerToken, 1e-12, "模型 %s 输入价格", model)
		require.InDelta(t, 50e-6, pricing.OutputPricePerToken, 1e-12, "模型 %s 输出价格", model)
	}
}

// 白名单语义：未知的 GPT-6 变体不得套用 Astra 价格。
// 将来若 OpenAI 上了更便宜的 gpt-6-mini，按 $10/$50 计价就是对用户超收——比漏计费更糟。
func TestNormalizeKnownOpenAICodexModel_UnknownGPT6VariantNotPriced(t *testing.T) {
	unknown := []string{
		"gpt-6-mini",
		"gpt-6-nano",
		"gpt-6-turbo",
	}

	for _, model := range unknown {
		require.Empty(t, normalizeKnownOpenAICodexModel(model),
			"未知 GPT-6 变体 %s 不应被归一化到已知型号", model)
	}
}

func TestIsOpenAIGPT6Model(t *testing.T) {
	positive := []string{"gpt-6", "gpt6", "gpt-6-astra", "GPT-6-ASTRA", "gpt-6-astra-high"}
	for _, model := range positive {
		require.True(t, isOpenAIGPT6Model(model), "模型 %s 应识别为 GPT-6 系列", model)
	}

	negative := []string{"gpt-6-mini", "gpt-5.6-sol", "gpt-5.4", "claude-opus-4.5", ""}
	for _, model := range negative {
		require.False(t, isOpenAIGPT6Model(model), "模型 %s 不应识别为 GPT-6 系列", model)
	}
}

// 渠道/数据库配置的价格若缺长上下文与缓存写入参数，补全策略必须对 GPT-6 生效，
// 否则「管理员在渠道里配价」这条路径会漏掉这些字段。
func TestApplyModelSpecificPricingPolicy_GPT6BackfillsFromChannelConfig(t *testing.T) {
	svc := newTestBillingService()

	// 模拟渠道只配了输入/输出价，没配长上下文和缓存写入。
	partial := &ModelPricing{
		InputPricePerToken:         10e-6,
		InputPricePerTokenPriority: 20e-6,
		OutputPricePerToken:        50e-6,
	}

	got := svc.applyModelSpecificPricingPolicy("gpt-6-astra", partial)
	require.NotNil(t, got)

	require.Equal(t, 272000, got.LongContextInputThreshold, "长上下文阈值应被补全")
	require.InDelta(t, 2.0, got.LongContextInputMultiplier, 1e-12)
	require.InDelta(t, 1.5, got.LongContextOutputMultiplier, 1e-12)
	require.InDelta(t, 12.5e-6, got.CacheCreationPricePerToken, 1e-12, "缓存写入应按输入价 1.25 倍补全")
	require.InDelta(t, 25e-6, got.CacheCreationPricePerTokenPriority, 1e-12)

	// 不得就地改写调用方传入的对象。
	require.Zero(t, partial.LongContextInputThreshold)
	require.Zero(t, partial.CacheCreationPricePerToken)
}

// 端到端：确认 gpt-6-astra 真的会产生非零扣费，并符合
// 「标准成本 × 分组倍率」的既有公式（最终隐藏倍率在更外层叠加）。
func TestCalculateCost_GPT6AstraProducesNonZeroCost(t *testing.T) {
	svc := newTestBillingService()

	// 输入合计 100K，低于 272K 阈值，走标准价：0.1M×$10 + 0.1M×$50 = $6。
	tokens := UsageTokens{InputTokens: 100_000, OutputTokens: 100_000}

	cost, err := svc.CalculateCost("gpt-6-astra", tokens, 1.0)
	require.NoError(t, err)
	require.InDelta(t, 6.0, cost.TotalCost, 1e-6)
	require.Greater(t, cost.ActualCost, 0.0, "必须产生实际扣费，否则就是用了不计费")

	// 分组倍率按既有语义线性作用于 ActualCost。
	cost028, err := svc.CalculateCost("gpt-6-astra", tokens, 0.28)
	require.NoError(t, err)
	require.InDelta(t, cost.TotalCost, cost028.TotalCost, 1e-6, "TotalCost 不受分组倍率影响")
	require.InDelta(t, cost.ActualCost*0.28, cost028.ActualCost, 1e-6)
}

// 超过 272K 输入后整次请求转长上下文价：输入与缓存 2x、输出 1.5x。
func TestCalculateCost_GPT6AstraLongContext(t *testing.T) {
	svc := newTestBillingService()

	// 输入 1M > 272K，触发长上下文：1M×$10×2 + 1M×$50×1.5 = $20 + $75 = $95。
	tokens := UsageTokens{InputTokens: 1_000_000, OutputTokens: 1_000_000}

	cost, err := svc.CalculateCost("gpt-6-astra", tokens, 1.0)
	require.NoError(t, err)
	require.InDelta(t, 95.0, cost.TotalCost, 1e-6, "长上下文应按 2x 输入 / 1.5x 输出计价")

	// 与刚好不越过阈值的请求对比，确认阈值确实生效而非恒定加价。
	under := UsageTokens{InputTokens: 272_000, OutputTokens: 1_000_000}
	costUnder, err := svc.CalculateCost("gpt-6-astra", under, 1.0)
	require.NoError(t, err)
	// 0.272M×$10 + 1M×$50 = $2.72 + $50 = $52.72
	require.InDelta(t, 52.72, costUnder.TotalCost, 1e-6, "恰好等于阈值时仍按标准价")
}
