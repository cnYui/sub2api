//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type plazaAccountRepoStub struct {
	accountsByGroup map[int64][]Account
	err             error
}

func (s *plazaAccountRepoStub) ListByGroup(_ context.Context, groupID int64) ([]Account, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.accountsByGroup[groupID], nil
}

// newPlazaChannelService 构造 ListPlazaGroups 测试用的 ChannelService。
func newPlazaChannelService(channels []Channel, groups []Group, pricing *PricingService) *ChannelService {
	repo := &mockChannelRepository{
		listAllFn: func(ctx context.Context) ([]Channel, error) { return channels, nil },
	}
	svc := NewChannelService(repo, &stubGroupRepoForAvailable{activeGroups: groups}, nil, nil)
	svc.pricingService = pricing
	return svc
}

func plazaPricedChannel(id int64, name string, groupIDs []int64, platform string, models ...string) Channel {
	return Channel{
		ID:       id,
		Name:     name,
		Status:   StatusActive,
		GroupIDs: groupIDs,
		ModelPricing: []ChannelModelPricing{{
			Platform:    platform,
			Models:      models,
			BillingMode: BillingModeToken,
			InputPrice:  testPtrFloat64(3e-6),
			OutputPrice: testPtrFloat64(1.5e-5),
		}},
	}
}

func TestListPlazaGroups_GroupCentricAggregation(t *testing.T) {
	// 两个渠道挂同一分组:模型并入同一 PlazaGroup;无模型的分组不返回。
	channels := []Channel{
		plazaPricedChannel(1, "chA", []int64{10}, "anthropic", "claude-sonnet"),
		plazaPricedChannel(2, "chB", []int64{10}, "anthropic", "claude-opus"),
	}
	groups := []Group{
		{ID: 10, Name: "g-main", Description: "desc", Platform: "anthropic", RateMultiplier: 1},
		{ID: 20, Name: "g-empty", Platform: "anthropic", RateMultiplier: 0.5},
	}
	svc := newPlazaChannelService(channels, groups, nil)
	out, err := svc.ListPlazaGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1, "无模型的分组不应返回")
	require.Equal(t, int64(10), out[0].ID)
	require.Equal(t, "desc", out[0].Description)
	require.Len(t, out[0].Models, 2)
	// 组内模型按名称排序
	require.Equal(t, "claude-opus", out[0].Models[0].Name)
	require.Equal(t, "claude-sonnet", out[0].Models[1].Name)
}

func TestListPlazaGroups_DedupFirstWinsWithPricingUpgrade(t *testing.T) {
	// 同名模型:先见者胜;仅当已存条目无定价而新条目有定价时升级替换。
	unpriced := Channel{
		ID: 1, Name: "alpha", Status: StatusActive, GroupIDs: []int64{10},
		// mapping-only → SupportedModels 产出无定价条目
		ModelMapping: map[string]map[string]string{
			"anthropic": {"claude-sonnet": "claude-sonnet"},
		},
	}
	priced := plazaPricedChannel(2, "beta", []int64{10}, "anthropic", "claude-sonnet")
	groups := []Group{{ID: 10, Name: "g", Platform: "anthropic", RateMultiplier: 1}}

	// alpha(无价)按名称序先于 beta(有价):先见者无价,应被有价条目升级。
	svc := newPlazaChannelService([]Channel{priced, unpriced}, groups, nil)
	out, err := svc.ListPlazaGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Len(t, out[0].Models, 1)
	require.NotNil(t, out[0].Models[0].Pricing, "无价条目应被有价条目升级")
	require.NotNil(t, out[0].Models[0].Pricing.InputPrice)
}

func TestListPlazaGroups_PlatformIsolation(t *testing.T) {
	// 渠道同时有 anthropic/openai 定价,anthropic 分组只应看到 anthropic 模型。
	ch := Channel{
		ID: 1, Name: "multi", Status: StatusActive, GroupIDs: []int64{10, 20},
		ModelPricing: []ChannelModelPricing{
			{Platform: "anthropic", Models: []string{"claude-sonnet"}, InputPrice: testPtrFloat64(3e-6)},
			{Platform: "openai", Models: []string{"gpt-5"}, InputPrice: testPtrFloat64(2e-6)},
		},
	}
	groups := []Group{
		{ID: 10, Name: "g-claude", Platform: "anthropic", RateMultiplier: 1},
		{ID: 20, Name: "g-gpt", Platform: "openai", RateMultiplier: 1},
	}
	svc := newPlazaChannelService([]Channel{ch}, groups, nil)
	out, err := svc.ListPlazaGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 2)
	byName := map[string][]PlazaModel{}
	for _, g := range out {
		byName[g.Name] = g.Models
	}
	require.Len(t, byName["g-claude"], 1)
	require.Equal(t, "claude-sonnet", byName["g-claude"][0].Name)
	require.Len(t, byName["g-gpt"], 1)
	require.Equal(t, "gpt-5", byName["g-gpt"][0].Name)
}

func TestListPlazaGroups_InactiveChannelSkipped(t *testing.T) {
	inactive := plazaPricedChannel(1, "off", []int64{10}, "anthropic", "claude-sonnet")
	inactive.Status = "inactive"
	groups := []Group{{ID: 10, Name: "g", Platform: "anthropic", RateMultiplier: 1}}
	svc := newPlazaChannelService([]Channel{inactive}, groups, nil)
	out, err := svc.ListPlazaGroups(context.Background())
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestListPlazaGroups_SortedByRateMultiplierAsc(t *testing.T) {
	channels := []Channel{
		plazaPricedChannel(1, "ch", []int64{10, 20, 30}, "anthropic", "claude-sonnet"),
	}
	groups := []Group{
		{ID: 10, Name: "b-standard", Platform: "anthropic", RateMultiplier: 1},
		{ID: 20, Name: "a-standard", Platform: "anthropic", RateMultiplier: 1},
		{ID: 30, Name: "cheap", Platform: "anthropic", RateMultiplier: 0.5},
	}
	svc := newPlazaChannelService(channels, groups, nil)
	out, err := svc.ListPlazaGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 3)
	require.Equal(t, "cheap", out[0].Name, "倍率低者在前")
	require.Equal(t, "a-standard", out[1].Name, "同倍率按名称")
	require.Equal(t, "b-standard", out[2].Name)
}

func TestListPlazaGroups_OfficialPricingFill(t *testing.T) {
	pricingSvc := newStubPricingServiceFromMap(map[string]*LiteLLMModelPricing{
		"claude-sonnet": {
			Mode:                                "chat",
			InputCostPerToken:                   3e-6,
			OutputCostPerToken:                  1.5e-5,
			CacheCreationInputTokenCost:         3.75e-6,
			CacheCreationInputTokenCostAbove1hr: 6e-6,
			CacheReadInputTokenCost:             3e-7,
		},
		"token-absent": {Mode: "image_generation", TokenPricingAbsent: true, OutputCostPerImage: 0.04},
	})
	channels := []Channel{
		plazaPricedChannel(1, "ch", []int64{10}, "anthropic", "claude-sonnet", "unknown-model", "token-absent"),
	}
	groups := []Group{{ID: 10, Name: "g", Platform: "anthropic", RateMultiplier: 1}}
	svc := newPlazaChannelService(channels, groups, pricingSvc)
	out, err := svc.ListPlazaGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Len(t, out[0].Models, 3)

	byName := map[string]PlazaModel{}
	for _, m := range out[0].Models {
		byName[m.Name] = m
	}
	// 命中:填充完整官方价(含 1h 缓存写入)
	official := byName["claude-sonnet"].OfficialPricing
	require.NotNil(t, official)
	require.InDelta(t, 3e-6, *official.InputPrice, 1e-12)
	require.InDelta(t, 6e-6, *official.CacheWrite1hPrice, 1e-12)
	require.InDelta(t, 3e-7, *official.CacheReadPrice, 1e-12)
	// 未命中:nil(GetModelPricing 的 claude 系列模糊匹配对非 claude 名不生效)
	require.Nil(t, byName["unknown-model"].OfficialPricing)
	// TokenPricingAbsent 条目不作为官方 token 价展示
	require.Nil(t, byName["token-absent"].OfficialPricing)
}

func TestListPlazaGroups_UsesBoundAccountModelMappings(t *testing.T) {
	pricingSvc := newStubPricingServiceFromMap(map[string]*LiteLLMModelPricing{
		"kimi-k3": {
			Mode:                        "chat",
			InputCostPerToken:           3e-6,
			OutputCostPerToken:          15e-6,
			CacheCreationInputTokenCost: 3e-6,
			CacheReadInputTokenCost:     0.3e-6,
		},
	})
	accounts := &plazaAccountRepoStub{accountsByGroup: map[int64][]Account{
		7: []Account{{
			Status: StatusActive,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"kimi-k3": "kimi-k3",
				},
			},
		}},
	}}
	repo := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return nil, nil }}
	svc := NewChannelService(
		repo,
		&stubGroupRepoForAvailable{activeGroups: []Group{{ID: 7, Name: "Kimi", Platform: "openai", RateMultiplier: 3.5}}},
		nil,
		pricingSvc,
		accounts,
	)

	out, err := svc.ListPlazaGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Len(t, out[0].Models, 1)
	require.Equal(t, "kimi-k3", out[0].Models[0].Name)
	require.NotNil(t, out[0].Models[0].Pricing)
	require.InDelta(t, 3e-6, *out[0].Models[0].Pricing.InputPrice, 1e-12)
}

func TestListPlazaGroups_UsesBillingFallbackWhenCatalogMissesModel(t *testing.T) {
	accounts := &plazaAccountRepoStub{accountsByGroup: map[int64][]Account{
		7: []Account{{
			Status: StatusActive,
			Credentials: map[string]any{
				"model_mapping": map[string]any{"kimi-k3": "kimi-k3"},
			},
		}},
	}}
	repo := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return nil, nil }}
	billing := NewBillingService(&config.Config{}, nil)
	svc := NewChannelService(
		repo,
		&stubGroupRepoForAvailable{activeGroups: []Group{{ID: 7, Name: "Kimi", Platform: "openai"}}},
		nil,
		newStubPricingServiceFromMap(nil),
		accounts,
	)
	svc.plazaBillingService = billing

	out, err := svc.ListPlazaGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.NotNil(t, out[0].Models[0].Pricing)
	require.InDelta(t, 3e-6, *out[0].Models[0].Pricing.InputPrice, 1e-12)
	require.InDelta(t, 15e-6, *out[0].Models[0].Pricing.OutputPrice, 1e-12)
}

func TestListPlazaGroups_KimiUsesCalibratedPricingForDisplay(t *testing.T) {
	pricingSvc := newStubPricingServiceFromMap(map[string]*LiteLLMModelPricing{
		"kimi-k3":   {InputCostPerToken: 9e-6, OutputCostPerToken: 99e-6, CacheReadInputTokenCost: 0.9e-6},
		"kimi-k2.6": {InputCostPerToken: 0.954e-6, OutputCostPerToken: 3.961e-6, CacheReadInputTokenCost: 0.161e-6},
		"kimi-k2.5": {InputCostPerToken: 0.6e-6, OutputCostPerToken: 2.25e-6, CacheReadInputTokenCost: 0.1e-6},
	})
	accounts := &plazaAccountRepoStub{accountsByGroup: map[int64][]Account{
		7: {{
			Status: StatusActive,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"kimi-k3": "kimi-k3", "kimi-k2.6": "kimi-k2.6", "kimi-k2.5": "kimi-k2.5",
				},
			},
		}},
	}}
	repo := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return nil, nil }}
	billing := NewBillingService(&config.Config{}, pricingSvc)
	svc := NewChannelService(
		repo,
		&stubGroupRepoForAvailable{activeGroups: []Group{{ID: 7, Name: "Kimi", Platform: "openai", RateMultiplier: 4.9}}},
		nil,
		pricingSvc,
		accounts,
	)
	svc.plazaBillingService = billing

	out, err := svc.ListPlazaGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, 4.9, out[0].RateMultiplier)

	byName := make(map[string]PlazaModel, len(out[0].Models))
	for _, model := range out[0].Models {
		byName[model.Name] = model
	}
	assertPlazaPricing(t, byName["kimi-k3"], 3e-6, 15e-6, 0.30e-6)
	assertPlazaPricing(t, byName["kimi-k2.6"], 0.95e-6, 4e-6, 0.16e-6)
	assertPlazaPricing(t, byName["kimi-k2.5"], 0.60e-6, 3e-6, 0.10e-6)
}

func assertPlazaPricing(t *testing.T, model PlazaModel, input, output, cacheRead float64) {
	t.Helper()
	require.NotNil(t, model.Pricing)
	require.NotNil(t, model.OfficialPricing)
	require.InDelta(t, input, *model.Pricing.InputPrice, 1e-12)
	require.InDelta(t, output, *model.Pricing.OutputPrice, 1e-12)
	require.InDelta(t, cacheRead, *model.Pricing.CacheReadPrice, 1e-12)
	require.InDelta(t, input, *model.OfficialPricing.InputPrice, 1e-12)
	require.InDelta(t, output, *model.OfficialPricing.OutputPrice, 1e-12)
	require.InDelta(t, cacheRead, *model.OfficialPricing.CacheReadPrice, 1e-12)
}

func TestListPlazaGroups_GLMUsesOfficialDomesticPriceTiers(t *testing.T) {
	pricingSvc := newStubPricingServiceFromMap(map[string]*LiteLLMModelPricing{
		"glm-5.1": {InputCostPerToken: 9e-6, OutputCostPerToken: 99e-6, CacheReadInputTokenCost: 0.9e-6},
	})
	accounts := &plazaAccountRepoStub{accountsByGroup: map[int64][]Account{
		6: {{
			Status: StatusActive,
			Credentials: map[string]any{
				"model_mapping": map[string]any{"glm-5.1": "glm-5.1"},
			},
		}},
	}}
	repo := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return nil, nil }}
	billing := NewBillingService(&config.Config{}, pricingSvc)
	svc := NewChannelService(
		repo,
		&stubGroupRepoForAvailable{activeGroups: []Group{{ID: 6, Name: "GLM", Platform: "openai", RateMultiplier: 3.5}}},
		nil,
		pricingSvc,
		accounts,
	)
	svc.plazaBillingService = billing

	out, err := svc.ListPlazaGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, 3.5, out[0].RateMultiplier)
	require.Len(t, out[0].Models, 1)
	model := out[0].Models[0]
	require.NotNil(t, model.Pricing)
	require.NotNil(t, model.OfficialPricing)
	require.InDelta(t, 6.0/7.0*1e-6, *model.Pricing.InputPrice, 1e-12)
	require.InDelta(t, 24.0/7.0*1e-6, *model.Pricing.OutputPrice, 1e-12)
	require.InDelta(t, 1.3/7.0*1e-6, *model.Pricing.CacheReadPrice, 1e-12)
	require.Len(t, model.Pricing.Intervals, 2)
	require.Len(t, model.OfficialPricing.Intervals, 2)
	require.Equal(t, "输入 <32K", model.Pricing.Intervals[0].TierLabel)
	require.Equal(t, "输入 >=32K", model.Pricing.Intervals[1].TierLabel)
	require.InDelta(t, 8.0/7.0*1e-6, *model.Pricing.Intervals[1].InputPrice, 1e-12)
	require.InDelta(t, 4e-6, *model.Pricing.Intervals[1].OutputPrice, 1e-12)
	require.InDelta(t, 2.0/7.0*1e-6, *model.Pricing.Intervals[1].CacheReadPrice, 1e-12)
}

func TestListPlazaGroups_DeepSeekUsesOfficialPrice(t *testing.T) {
	pricingSvc := newStubPricingServiceFromMap(map[string]*LiteLLMModelPricing{
		"deepseek-v4-flash": {InputCostPerToken: 9e-6, OutputCostPerToken: 99e-6, CacheReadInputTokenCost: 0.9e-6},
		"deepseek-v4-pro":   {InputCostPerToken: 9e-6, OutputCostPerToken: 99e-6, CacheReadInputTokenCost: 0.9e-6},
	})
	accounts := &plazaAccountRepoStub{accountsByGroup: map[int64][]Account{
		8: {{
			Status: StatusActive,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"deepseek-v4-flash": "deepseek-v4-flash",
					"deepseek-v4-pro":   "deepseek-v4-pro",
				},
			},
		}},
	}}
	repo := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return nil, nil }}
	billing := NewBillingService(&config.Config{}, pricingSvc)
	svc := NewChannelService(
		repo,
		&stubGroupRepoForAvailable{activeGroups: []Group{{ID: 8, Name: "DeepSeek", Platform: "openai", RateMultiplier: 3.5}}},
		nil,
		pricingSvc,
		accounts,
	)
	svc.plazaBillingService = billing

	out, err := svc.ListPlazaGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.InDelta(t, 3.5, out[0].RateMultiplier, 1e-12)
	byName := make(map[string]PlazaModel, len(out[0].Models))
	for _, model := range out[0].Models {
		byName[model.Name] = model
	}
	assertPlazaPricing(t, byName["deepseek-v4-flash"], 0.22e-6, 0.66e-6, 0.007e-6)
	assertPlazaPricing(t, byName["deepseek-v4-pro"], 0.66e-6, 1.98e-6, 0.022e-6)
}

func TestListPlazaGroups_AccountSourceErrorsPropagate(t *testing.T) {
	sentinel := errors.New("account source failed")
	repo := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return nil, nil }}
	svc := NewChannelService(
		repo,
		&stubGroupRepoForAvailable{activeGroups: []Group{{ID: 7, Name: "Kimi", Platform: "openai"}}},
		nil,
		nil,
		&plazaAccountRepoStub{err: sentinel},
	)

	out, err := svc.ListPlazaGroups(context.Background())
	require.Nil(t, out)
	require.ErrorIs(t, err, sentinel)
}

func TestListPlazaGroups_RepoErrorsPropagate(t *testing.T) {
	sentinel := errors.New("boom")
	repo := &mockChannelRepository{
		listAllFn: func(ctx context.Context) ([]Channel, error) { return nil, sentinel },
	}
	svc := NewChannelService(repo, &stubGroupRepoForAvailable{}, nil, nil)
	out, err := svc.ListPlazaGroups(context.Background())
	require.Nil(t, out)
	require.ErrorIs(t, err, sentinel)

	svc2 := NewChannelService(
		&mockChannelRepository{listAllFn: func(ctx context.Context) ([]Channel, error) { return nil, nil }},
		&stubGroupRepoForAvailable{listActiveErr: sentinel},
		nil, nil,
	)
	out2, err2 := svc2.ListPlazaGroups(context.Background())
	require.Nil(t, out2)
	require.ErrorIs(t, err2, sentinel)
}
