//go:build unit

package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// GPT-6 Astra 官方定价（2026-09-04 发布，https://developers.openai.com/api/docs/models/gpt-6-astra）：
// 输入 $10 / 缓存读取 $1 / 缓存写入 $12.50 / 输出 $50，每百万 token；
// 超过 272K 输入的整次请求按 2x 输入与缓存、1.5x 输出计价。
const (
	gpt6AstraInputPerToken  = 10e-6
	gpt6AstraOutputPerToken = 50e-6
	gpt6AstraCacheReadPerTk = 1e-6
	gpt6AstraCacheWritePerT = 12.5e-6

	// 末尾 DefaultTestModel 兜底命中的价格。任何 GPT-6 请求取到这两个数，
	// 就是踩了兜底：输入少收 4 倍、输出少收 3.33 倍。
	gpt54FallbackInputPerToken  = 2.5e-6
	gpt54FallbackOutputPerToken = 15e-6
)

// newGPT6ProductionLikeBillingService 构造一个**贴近生产接线**的计费服务：
// pricingService 非 nil，且目录里同时有 gpt-6-astra 与 gpt-5.4——
// 后者是 matchOpenAIModel 末尾 DefaultTestModel 兜底会命中的键。
//
// 这一点很关键：生产的 wire 注入是非 nil 的 PricingService（wire_gen.go），
// 取价顺序为「渠道/分组定价 → PricingService → 硬编码 fallbackPrices」。
// 用 nil pricingService 构造的测试只会走到第三级，覆盖不到生产实际走的路径。
func newGPT6ProductionLikeBillingService() *BillingService {
	return NewBillingService(&config.Config{}, &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-6-astra": {
				InputCostPerToken:           gpt6AstraInputPerToken,
				OutputCostPerToken:          gpt6AstraOutputPerToken,
				CacheReadInputTokenCost:     gpt6AstraCacheReadPerTk,
				CacheCreationInputTokenCost: gpt6AstraCacheWritePerT,
				LiteLLMProvider:             "openai",
				Mode:                        "chat",
				SupportsPromptCaching:       true,
			},
			"gpt-5.4": {
				InputCostPerToken:       gpt54FallbackInputPerToken,
				OutputCostPerToken:      gpt54FallbackOutputPerToken,
				CacheReadInputTokenCost: 0.25e-6,
				LiteLLMProvider:         "openai",
				Mode:                    "chat",
			},
		},
	})
}

// 回归测试：这是本次真正的缺陷。
//
// matchOpenAIModel 末尾有一个 DefaultTestModel（= gpt-5.4）兜底，任何以 gpt- 开头
// 又没被前面分支拦住的模型都会掉进去。远端价格目录只收录了精确串 gpt-6-astra，
// 所以 gpt-6 / gpt6 / 带 effort 或日期后缀的写法全部会按 $2.5/$15 计价，
// 相对 Astra 实际的 $10/$50 是输入少收 4 倍、输出少收 3.33 倍。
//
// 少收的金额一旦写进 usage_logs.actual_cost 就没有正确的重算依据，事后无法追回。
func TestGetModelPricing_GPT6AliasesMustNotFallBackToGPT54(t *testing.T) {
	svc := newGPT6ProductionLikeBillingService()

	aliases := []string{
		"gpt-6-astra",            // 精确串，目录直接命中
		"GPT-6-Astra",            // 大小写
		"gpt6-astra",             // 缺连字符
		"gpt-6",                  // 裸族名
		"gpt6",                   // 裸族名且缺连字符
		"gpt-6-astra-high",       // effort 后缀
		"gpt-6-astra-2026-09-04", // 日期快照
		"openai/gpt-6-astra",     // 带路径前缀
	}

	for _, model := range aliases {
		pricing, err := svc.GetModelPricing(model)
		require.NoError(t, err, "模型 %s", model)
		require.NotNil(t, pricing, "模型 %s 必须有价格", model)

		require.False(t, math.Abs(pricing.InputPricePerToken-gpt54FallbackInputPerToken) < 1e-12,
			"模型 %s 掉进了 gpt-5.4 兜底价（$2.5/M），会少收 4 倍", model)

		require.InDelta(t, gpt6AstraInputPerToken, pricing.InputPricePerToken, 1e-12,
			"模型 %s 输入价应为 $10/M", model)
		require.InDelta(t, gpt6AstraOutputPerToken, pricing.OutputPricePerToken, 1e-12,
			"模型 %s 输出价应为 $50/M", model)
	}
}

// 长上下文三个字段只能由静态价提供：远端目录用 *_above_272k_tokens 表达，
// 而解析器只认 long_context_*，目录里那三个键命中数为 0。
func TestMatchOpenAIModel_GPT6StaticPricingCarriesLongContext(t *testing.T) {
	svc := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		// 刻意不放 gpt-6-astra，强制走静态分支
		"gpt-5.4": {InputCostPerToken: gpt54FallbackInputPerToken, OutputCostPerToken: gpt54FallbackOutputPerToken},
	}}

	for _, model := range []string{"gpt-6-astra", "gpt-6", "gpt-6-astra-high"} {
		got := svc.matchOpenAIModel(model)
		require.NotNil(t, got, "模型 %s", model)
		require.InDelta(t, gpt6AstraInputPerToken, got.InputCostPerToken, 1e-12, model)
		require.InDelta(t, gpt6AstraOutputPerToken, got.OutputCostPerToken, 1e-12, model)
		require.InDelta(t, gpt6AstraCacheWritePerT, got.CacheCreationInputTokenCost, 1e-12, model)
		require.InDelta(t, gpt6AstraCacheReadPerTk, got.CacheReadInputTokenCost, 1e-12, model)

		require.Equal(t, 272000, got.LongContextInputTokenThreshold, model)
		require.InDelta(t, 2.0, got.LongContextInputCostMultiplier, 1e-12, model)
		require.InDelta(t, 1.5, got.LongContextOutputCostMultiplier, 1e-12, model)
	}
}

// 白名单语义：未知的 GPT-6 变体不得套用 Astra 价格。
// 将来若 OpenAI 上了更便宜的 gpt-6-mini，按 $10/$50 计价就是对用户超收——比漏计费更糟。
//
// 注意：这些型号仍会掉进 DefaultTestModel(gpt-5.4) 兜底，这是本函数既有的全局行为、
// 对所有未知 gpt-* 型号一视同仁，不在本次修复范围内。这里只断言不会误用 Astra 价。
func TestNormalizeKnownOpenAICodexModel_UnknownGPT6VariantNotPriced(t *testing.T) {
	for _, model := range []string{"gpt-6-mini", "gpt-6-nano", "gpt-6-turbo"} {
		require.Empty(t, normalizeKnownOpenAICodexModel(model),
			"未知 GPT-6 变体 %s 不应被归一化到已知型号", model)
		require.False(t, isOpenAIGPT6Model(model),
			"未知 GPT-6 变体 %s 不应被识别为 GPT-6 系列，否则会套用 Astra 价格造成超收", model)
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

// 目录完全不可用时（远端同步失败、条目被删），硬编码 fallbackPrices 是最后一道正确价来源。
// 这条走的是取价链的第三级，生产上只有目录失效时才会命中。
func TestGetModelPricing_GPT6FallbackWhenCatalogUnavailable(t *testing.T) {
	svc := newTestBillingService() // pricingService 为 nil，强制走 fallbackPrices

	pricing, err := svc.GetModelPricing("gpt-6-astra")
	require.NoError(t, err)
	require.NotNil(t, pricing)

	require.InDelta(t, gpt6AstraInputPerToken, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, gpt6AstraOutputPerToken, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, gpt6AstraCacheReadPerTk, pricing.CacheReadPricePerToken, 1e-12)
	require.InDelta(t, gpt6AstraCacheWritePerT, pricing.CacheCreationPricePerToken, 1e-12)

	// 缓存写入必须正好是输入价的 1.25 倍（OpenAI 全系惯例）。
	require.InDelta(t, pricing.InputPricePerToken*1.25, pricing.CacheCreationPricePerToken, 1e-12)

	// priority tier 为标准价的 2 倍，与 GPT-5.6 全系一致。
	require.InDelta(t, 20e-6, pricing.InputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 100e-6, pricing.OutputPricePerTokenPriority, 1e-12)

	require.Equal(t, 272000, pricing.LongContextInputThreshold)
	require.InDelta(t, 2.0, pricing.LongContextInputMultiplier, 1e-12)
	require.InDelta(t, 1.5, pricing.LongContextOutputMultiplier, 1e-12)
}

// 渠道/数据库配置的价格若缺长上下文与缓存写入参数，补全策略必须对 GPT-6 生效。
func TestApplyModelSpecificPricingPolicy_GPT6BackfillsFromChannelConfig(t *testing.T) {
	svc := newGPT6ProductionLikeBillingService()

	partial := &ModelPricing{
		InputPricePerToken:         gpt6AstraInputPerToken,
		InputPricePerTokenPriority: 20e-6,
		OutputPricePerToken:        gpt6AstraOutputPerToken,
	}

	got := svc.applyModelSpecificPricingPolicy("gpt-6-astra", partial)
	require.NotNil(t, got)

	require.Equal(t, 272000, got.LongContextInputThreshold, "长上下文阈值应被补全")
	require.InDelta(t, 2.0, got.LongContextInputMultiplier, 1e-12)
	require.InDelta(t, 1.5, got.LongContextOutputMultiplier, 1e-12)
	require.InDelta(t, gpt6AstraCacheWritePerT, got.CacheCreationPricePerToken, 1e-12, "缓存写入应按输入价 1.25 倍补全")

	// 不得就地改写调用方传入的对象。
	require.Zero(t, partial.LongContextInputThreshold)
	require.Zero(t, partial.CacheCreationPricePerToken)
}

// 端到端金额判别：这正是上线后要在生产上核对的那个数。
func TestCalculateCost_GPT6AstraAmountDiscriminates(t *testing.T) {
	svc := newGPT6ProductionLikeBillingService()

	// 1000 输入 token，低于 272K 阈值，走标准价。
	// $10/M → $0.01；若踩了 gpt-5.4 兜底则是 $0.0025，差 4 倍。
	cost, err := svc.CalculateCost("gpt-6-astra", UsageTokens{InputTokens: 1000}, 1.0)
	require.NoError(t, err)
	require.InDelta(t, 0.01, cost.TotalCost, 1e-9,
		"1000 输入 token 应为 $0.01；若为 $0.0025 说明踩了 gpt-5.4 兜底")

	// 分组倍率线性作用于 ActualCost，不影响 TotalCost。
	cost028, err := svc.CalculateCost("gpt-6-astra", UsageTokens{InputTokens: 1000}, 0.28)
	require.NoError(t, err)
	require.InDelta(t, cost.TotalCost, cost028.TotalCost, 1e-9)
	require.InDelta(t, cost.ActualCost*0.28, cost028.ActualCost, 1e-9)
}
