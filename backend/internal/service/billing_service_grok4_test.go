//go:build unit

package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// xAI Grok 4 系列官方定价（https://docs.x.ai/developers/models，2026-09-22 核对）：
// 4.7 与 4.6 逐项同价，4.5 只有缓存读更便宜；三者 ≥200K 输入的整次请求
// 按 2x 输入 / 2x 缓存 / 2x 输出计价（注意输出倍率是 2x，不是 GPT 的 1.5x）。
const (
	grok4InputPerToken       = 2e-6
	grok4OutputPerToken      = 6e-6
	grok46CacheReadPerToken  = 0.5e-6
	grok45CacheReadPerToken  = 0.3e-6
	grok4LongCtxBoundaryToks = 199999
)

// newGrokProductionLikeBillingService 贴近生产接线：pricingService 非 nil，
// 但目录里**没有任何 grok-4.x 键**——这正是生产实情（远端 LiteLLM 目录至今
// 未收录 4.5/4.6/4.7）。用 nil pricingService 构造的测试只覆盖第 ③ 级取价，
// 测不到生产实际走的路径。
func newGrokProductionLikeBillingService() *BillingService {
	return NewBillingService(&config.Config{}, &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.4": {
				InputCostPerToken:  2.5e-6,
				OutputCostPerToken: 15e-6,
				LiteLLMProvider:    "openai",
				Mode:               "chat",
			},
		},
	})
}

// 核心回归：目录查不到 grok-4.7 时必须落到兜底官方价，而不是
// ErrModelPricingUnavailable（会被上层吞成零成本放行）或别的模型的价。
func TestGrokFallbackPricingMatchesOfficialRates(t *testing.T) {
	svc := newGrokProductionLikeBillingService()

	cases := []struct {
		model     string
		cacheRead float64
	}{
		{"grok-4.7", grok46CacheReadPerToken},
		{"grok-4.6", grok46CacheReadPerToken},
		{"grok-4.5", grok45CacheReadPerToken},
		// 带前缀/后缀的等价写法必须解析到同一份价格
		{"xai/grok-4.7", grok46CacheReadPerToken},
		{"grok-4.5-latest", grok45CacheReadPerToken},
		{"GROK-4.6", grok46CacheReadPerToken},
	}

	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			pricing, err := svc.GetModelPricing(tc.model)
			require.NoError(t, err)
			require.NotNil(t, pricing)
			require.InDelta(t, grok4InputPerToken, pricing.InputPricePerToken, 1e-12)
			require.InDelta(t, grok4OutputPerToken, pricing.OutputPricePerToken, 1e-12)
			require.InDelta(t, grok4InputPerToken, pricing.CacheCreationPricePerToken, 1e-12,
				"xAI 缓存写不单独计价，应等于输入价")
			require.InDelta(t, tc.cacheRead, pricing.CacheReadPricePerToken, 1e-12)
			require.Equal(t, grok4LongCtxBoundaryToks, pricing.LongContextInputThreshold)
			require.InDelta(t, 2.0, pricing.LongContextInputMultiplier, 1e-12)
			require.InDelta(t, 2.0, pricing.LongContextOutputMultiplier, 1e-12)
		})
	}
}

// 未上架的 grok 型号不能借道 grok-4.x 的兜底价：宁可 fail-closed，
// 也不要按错价计费（白名单语义，与 DeepSeek/Kimi 一致）。
func TestGrokUnlistedModelsDoNotBorrowGrok4Pricing(t *testing.T) {
	svc := newGrokProductionLikeBillingService()

	for _, model := range []string{
		"grok-4.3",
		"grok-build-0.1",
		"grok-4.20-0309-reasoning",
		"grok-imagine-image-2.0",
		"grok-imagine-video-1.5",
	} {
		t.Run(model, func(t *testing.T) {
			require.Nil(t, svc.getFallbackPricing(model))
			require.False(t, isXAIGrok4Model(model))
		})
	}
}

// 目录哪天收录了 grok-4.x 但价格漂移时，仍以本地校准价为准。
// 这条同时钉住 usesCalibratedFallbackPricing 覆盖了 Grok。
func TestGrokPrefersCalibratedFallbackOverDriftedCatalog(t *testing.T) {
	svc := NewBillingService(&config.Config{}, &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"grok-4.7": {
				InputCostPerToken:       0.2e-6, // 漂移成 1/10
				OutputCostPerToken:      0.6e-6,
				CacheReadInputTokenCost: 0.05e-6,
				LiteLLMProvider:         "xai",
				Mode:                    "chat",
			},
		},
	})

	require.True(t, usesCalibratedFallbackPricing("grok-4.7"))

	pricing, err := svc.GetModelPricing("grok-4.7")
	require.NoError(t, err)
	require.InDelta(t, grok4InputPerToken, pricing.InputPricePerToken, 1e-12,
		"目录漂移价不得覆盖校准价，否则是少收")
	require.InDelta(t, grok4OutputPerToken, pricing.OutputPricePerToken, 1e-12)
}

// 长上下文档：边界是严格大于 199999，即 200000 个输入 token 起翻倍。
// 判定口径为 输入 + 缓存写 + 缓存读 三者之和。
func TestGrokLongContextBillingBoundary(t *testing.T) {
	svc := newGrokProductionLikeBillingService()
	pricing, err := svc.GetModelPricing("grok-4.7")
	require.NoError(t, err)

	require.False(t, svc.shouldApplySessionLongContextPricing(
		UsageTokens{InputTokens: grok4LongCtxBoundaryToks}, pricing))
	require.True(t, svc.shouldApplySessionLongContextPricing(
		UsageTokens{InputTokens: 200000}, pricing))
	// 缓存 token 也计入阈值判定
	require.True(t, svc.shouldApplySessionLongContextPricing(
		UsageTokens{InputTokens: 100000, CacheReadTokens: 100000}, pricing))

	short, err := svc.CalculateCost("grok-4.7",
		UsageTokens{InputTokens: 100000, OutputTokens: 1000}, 1)
	require.NoError(t, err)
	require.False(t, short.LongContextBillingApplied)
	require.InDelta(t, 100000*grok4InputPerToken, short.InputCost, 1e-9)
	require.InDelta(t, 1000*grok4OutputPerToken, short.OutputCost, 1e-9)

	long, err := svc.CalculateCost("grok-4.7",
		UsageTokens{InputTokens: 300000, OutputTokens: 1000}, 1)
	require.NoError(t, err)
	require.True(t, long.LongContextBillingApplied)
	require.InDelta(t, 300000*grok4InputPerToken*2, long.InputCost, 1e-9)
	require.InDelta(t, 1000*grok4OutputPerToken*2, long.OutputCost, 1e-9,
		"Grok 的长上下文输出倍率是 2x，不是 GPT 的 1.5x")
}

// 模型广场「官方」列：本次改动的直接目的。
// 没有 usesCalibratedFallbackPricing 覆盖 Grok 时，lookupOfficialPricing
// 第一分支不会被调用，而远端目录又没有这几个键，官方列只能显示 "-"。
func TestGrokPlazaOfficialPricingIsPopulated(t *testing.T) {
	svc := newGrokProductionLikeBillingService()
	pricing, err := svc.GetModelPricing("grok-4.7")
	require.NoError(t, err)

	official := plazaOfficialPricingFromBilling(pricing)
	require.NotNil(t, official, "官方价不能为空，否则广场显示 -")
	require.NotNil(t, official.InputPrice)
	require.InDelta(t, grok4InputPerToken, *official.InputPrice, 1e-12)
	require.NotNil(t, official.OutputPrice)
	require.InDelta(t, grok4OutputPerToken, *official.OutputPrice, 1e-12)
	require.NotNil(t, official.CacheReadPrice)
	require.InDelta(t, grok46CacheReadPerToken, *official.CacheReadPrice, 1e-12)

	// 官方列的两个档位要和渠道级实付区间对齐（渠道 4 配的是 0~199999 / 199999~∞）
	require.Len(t, official.Intervals, 2)
	require.Equal(t, 0, official.Intervals[0].MinTokens)
	require.NotNil(t, official.Intervals[0].MaxTokens)
	require.Equal(t, grok4LongCtxBoundaryToks, *official.Intervals[0].MaxTokens)
	require.Equal(t, "输入 <200K", official.Intervals[0].TierLabel)
	require.Equal(t, grok4LongCtxBoundaryToks, official.Intervals[1].MinTokens)
	require.Nil(t, official.Intervals[1].MaxTokens)
	require.Equal(t, "输入 >=200K", official.Intervals[1].TierLabel)
	require.NotNil(t, official.Intervals[1].InputPrice)
	require.InDelta(t, grok4InputPerToken*2, *official.Intervals[1].InputPrice, 1e-12)
	require.NotNil(t, official.Intervals[1].CacheReadPrice)
	require.InDelta(t, grok46CacheReadPerToken*2, *official.Intervals[1].CacheReadPrice, 1e-12,
		"缓存读未单独设倍率时复用输入倍率，官方口径同为 2x")
}

// 渠道级区间定价存在时按区间取价，账号上的 openai_long_context_billing_enabled
// 开关不参与——生产的 Grok 账号那个开关就是 false，别误判成在少收。
func TestGrokChannelIntervalsWinOverAccountLongContextSwitch(t *testing.T) {
	svc := newGrokProductionLikeBillingService()
	longInput := 300000

	withSwitchOff, err := svc.calculateCostWithServiceTierPolicy("grok-4.7",
		UsageTokens{InputTokens: longInput, OutputTokens: 1000}, 1, "", false)
	require.NoError(t, err)
	require.False(t, withSwitchOff.LongContextBillingApplied,
		"没有区间定价且账号开关关闭时，长上下文不生效——这正是渠道区间存在的理由")

	resolved := &ResolvedPricing{Intervals: grok4ChannelPricing().Intervals}
	iv := FindMatchingInterval(resolved.Intervals, longInput)
	require.NotNil(t, iv)
	require.NotNil(t, iv.InputPrice)
	require.InDelta(t, grok4InputPerToken*2, *iv.InputPrice, 1e-12)
	require.False(t, math.IsNaN(*iv.InputPrice))
}

// 本次给兜底价加了 LongContext* 字段，于是多出一个新风险：渠道区间已经把
// ≥200K 表达成 2x 了，如果 applyLongCtx 还叠上来就会变成 4x——广场显示 2x、
// 实扣 4x。`applyLongCtx := len(resolved.Intervals) == 0 && ...` 挡住了这一点，
// 这条用例把它钉死。
func TestGrokChannelIntervalsDoNotDoubleApplyLongContextMultiplier(t *testing.T) {
	svc := newGrokProductionLikeBillingService()
	channelPricing := grok4ChannelPricing()

	const longInput = 300000
	breakdown, err := svc.calculateCostInternal("grok-4.7",
		UsageTokens{InputTokens: longInput, OutputTokens: 1000}, 1, "", channelPricing)
	require.NoError(t, err)

	require.InDelta(t, longInput*grok4InputPerToken*2, breakdown.InputCost, 1e-9,
		"应当只按区间的长档价收一次 2x，而不是区间 2x 再叠 LongContextInputMultiplier")
	require.InDelta(t, 1000*grok4OutputPerToken*2, breakdown.OutputCost, 1e-9)

	short, err := svc.calculateCostInternal("grok-4.7",
		UsageTokens{InputTokens: 100000, OutputTokens: 1000}, 1, "", channelPricing)
	require.NoError(t, err)
	require.InDelta(t, 100000*grok4InputPerToken, short.InputCost, 1e-9)
	require.InDelta(t, 1000*grok4OutputPerToken, short.OutputCost, 1e-9)
}

// grok4ChannelPricing 复刻生产渠道 4 对 grok-4.7 的区间配置。
func grok4ChannelPricing() *ChannelModelPricing {
	boundary := grok4LongCtxBoundaryToks
	return &ChannelModelPricing{
		Platform:    "openai",
		Models:      []string{"grok-4.7"},
		BillingMode: BillingModeToken,
		InputPrice:  ptrFloat(grok4InputPerToken),
		OutputPrice: ptrFloat(grok4OutputPerToken),
		Intervals: []PricingInterval{
			{
				MinTokens: 0, MaxTokens: &boundary,
				InputPrice: ptrFloat(grok4InputPerToken), OutputPrice: ptrFloat(grok4OutputPerToken),
				CacheWritePrice: ptrFloat(grok4InputPerToken), CacheReadPrice: ptrFloat(grok46CacheReadPerToken),
			},
			{
				MinTokens:  grok4LongCtxBoundaryToks,
				InputPrice: ptrFloat(grok4InputPerToken * 2), OutputPrice: ptrFloat(grok4OutputPerToken * 2),
				CacheWritePrice: ptrFloat(grok4InputPerToken * 2), CacheReadPrice: ptrFloat(grok46CacheReadPerToken * 2),
			},
		},
	}
}

func ptrFloat(v float64) *float64 { return &v }
