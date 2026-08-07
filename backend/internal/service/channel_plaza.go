package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// PlazaOfficialPricing 模型广场展示用的 LiteLLM 官方参考价（USD per token）。
// 字段为 nil 表示官方数据中该项缺失（0 视为未配置）。
type PlazaOfficialPricing struct {
	InputPrice        *float64
	OutputPrice       *float64
	CacheWritePrice   *float64 // 5m 缓存写入（= LiteLLM cache_creation）
	CacheWrite1hPrice *float64 // 1h 缓存写入（LiteLLM cache_creation_above_1hr）
	CacheReadPrice    *float64
	Intervals         []PricingInterval
}

// PlazaModel 模型广场中单个模型条目：渠道定价 + 官方参考价。
type PlazaModel struct {
	Name            string
	Platform        string
	Pricing         *ChannelModelPricing
	OfficialPricing *PlazaOfficialPricing
}

// PlazaGroup 模型广场中以分组为顶层的条目。
//
// 与 AvailableGroupRef 相比多了 Description 与 Models；Models 来自该分组关联渠道的
// 支持模型（按分组平台隔离，防跨平台泄漏），与「可用渠道」页口径一致。
type PlazaGroup struct {
	ID                 int64
	Name               string
	Description        string
	Platform           string
	SubscriptionType   string
	RateMultiplier     float64
	PeakRateEnabled    bool
	PeakStart          string
	PeakEnd            string
	PeakRateMultiplier float64
	IsExclusive        bool
	Models             []PlazaModel
}

// ListPlazaGroups 返回模型广场数据：每个活跃分组附带其可用模型与定价。
//
// 聚合口径以实际可调度账号的模型映射为准，并兼容 Active 渠道配置：
//
//   - 账号绑定分组是本项目真正的模型可用性来源；部分部署不会创建 channels 定价行，
//     此时不能把模型广场错误地展示为空；
//   - Active 渠道仍可提供显式定价覆盖，账号映射中未被渠道声明的模型回退全局定价；
//   - 渠道按 lower(name) 排序后遍历，保证同名模型去重结果确定；
//   - 同分组同名模型「先见者胜」，仅当已存条目无定价而新条目有定价时升级替换；
//   - 每个模型附带 LiteLLM 官方参考价（查不到为 nil）；
//   - 只返回 Models 非空的分组；分组按 RateMultiplier 升序（同倍率按名称），
//     组内模型按名称排序。
//
// 可见性过滤（专属分组）不在此层做，由 handler 按登录态裁剪。
func (s *ChannelService) ListPlazaGroups(ctx context.Context) ([]PlazaGroup, error) {
	channels, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}

	sort.SliceStable(channels, func(i, j int) bool {
		return strings.ToLower(channels[i].Name) < strings.ToLower(channels[j].Name)
	})

	byGroup := make(map[int64]*PlazaGroup, len(groups))
	order := make([]int64, 0, len(groups))
	for i := range groups {
		g := groups[i]
		byGroup[g.ID] = &PlazaGroup{
			ID:                 g.ID,
			Name:               g.Name,
			Description:        g.Description,
			Platform:           g.Platform,
			SubscriptionType:   g.SubscriptionType,
			RateMultiplier:     g.RateMultiplier,
			PeakRateEnabled:    g.PeakRateEnabled,
			PeakStart:          g.PeakStart,
			PeakEnd:            g.PeakEnd,
			PeakRateMultiplier: g.PeakRateMultiplier,
			IsExclusive:        g.IsExclusive,
		}
		order = append(order, g.ID)
	}

	// modelIdx[groupID][modelName] = index into byGroup[groupID].Models
	modelIdx := make(map[int64]map[string]int, len(groups))
	for i := range channels {
		ch := &channels[i]
		if ch.Status != StatusActive {
			continue
		}
		ch.normalizeBillingModelSource()
		supported := ch.SupportedModels()
		s.fillGlobalPricingFallback(supported)

		for _, gid := range ch.GroupIDs {
			pg, ok := byGroup[gid]
			if !ok {
				continue
			}
			idx := modelIdx[gid]
			if idx == nil {
				idx = make(map[string]int, len(supported))
				modelIdx[gid] = idx
			}
			for j := range supported {
				m := supported[j]
				if m.Platform != pg.Platform {
					continue
				}
				modelKey := strings.ToLower(m.Name)
				if at, seen := idx[modelKey]; seen {
					// 先见者胜；仅当已存条目无定价而新条目有定价时升级。
					if pg.Models[at].Pricing == nil && m.Pricing != nil {
						pg.Models[at].Pricing = m.Pricing
					}
					continue
				}
				idx[modelKey] = len(pg.Models)
				pg.Models = append(pg.Models, PlazaModel{
					Name:     m.Name,
					Platform: m.Platform,
					Pricing:  m.Pricing,
				})
			}
		}
	}

	if err := s.appendAccountMappedPlazaModels(ctx, byGroup, order, modelIdx); err != nil {
		return nil, err
	}

	officialMemo := make(map[string]*PlazaOfficialPricing)
	out := make([]PlazaGroup, 0, len(order))
	for _, gid := range order {
		pg := byGroup[gid]
		if len(pg.Models) == 0 {
			continue
		}
		sort.SliceStable(pg.Models, func(i, j int) bool { return pg.Models[i].Name < pg.Models[j].Name })
		for j := range pg.Models {
			pg.Models[j].OfficialPricing = s.lookupOfficialPricing(pg.Models[j].Name, officialMemo)
		}
		out = append(out, *pg)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RateMultiplier != out[j].RateMultiplier {
			return out[i].RateMultiplier < out[j].RateMultiplier
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// appendAccountMappedPlazaModels 将绑定账号的显式模型映射并入广场。
// channels 中配置的价格优先；只有缺少模型或缺少价格时，账号映射和全局定价才补齐。
func (s *ChannelService) appendAccountMappedPlazaModels(
	ctx context.Context,
	byGroup map[int64]*PlazaGroup,
	groupIDs []int64,
	modelIdx map[int64]map[string]int,
) error {
	if s.plazaAccountRepo == nil {
		return nil
	}

	for _, groupID := range groupIDs {
		group := byGroup[groupID]
		accounts, err := s.plazaAccountRepo.ListByGroup(ctx, groupID)
		if err != nil {
			return fmt.Errorf("list group %d accounts: %w", groupID, err)
		}
		index := modelIdx[groupID]
		if index == nil {
			index = make(map[string]int)
			modelIdx[groupID] = index
		}

		for i := range accounts {
			account := &accounts[i]
			if !account.IsActive() {
				continue
			}
			modelNames := make([]string, 0, len(account.GetModelMapping()))
			for modelName := range account.GetModelMapping() {
				modelNames = append(modelNames, modelName)
			}
			sort.Strings(modelNames)

			for _, modelName := range modelNames {
				key := strings.ToLower(modelName)
				if at, exists := index[key]; exists {
					if group.Models[at].Pricing == nil {
						group.Models[at].Pricing = s.plazaFallbackPricing(modelName)
					}
					continue
				}
				index[key] = len(group.Models)
				group.Models = append(group.Models, PlazaModel{
					Name:     modelName,
					Platform: group.Platform,
					Pricing:  s.plazaFallbackPricing(modelName),
				})
			}
		}
	}
	return nil
}

// plazaFallbackPricing 复用用户可见价格的全局回退规则，避免模型广场与实际计费基础价分叉。
func (s *ChannelService) plazaFallbackPricing(modelName string) *ChannelModelPricing {
	if s.plazaBillingService != nil {
		if pricing, err := s.plazaBillingService.GetModelPricing(modelName); err == nil {
			return synthesizePricingFromBilling(pricing)
		}
	}
	if s.pricingService == nil {
		return nil
	}
	return synthesizePricingFromLiteLLM(s.pricingService.GetModelPricing(modelName), nil)
}

// synthesizePricingFromBilling 将服务端实际计费解析结果转换为广场展示结构。
// 计费解析器包含本地兜底价和已校准价格，优先复用它可避免目录缺项造成空价或口径漂移。
func synthesizePricingFromBilling(pricing *ModelPricing) *ChannelModelPricing {
	if pricing == nil {
		return nil
	}
	input := pricing.InputPricePerToken
	output := pricing.OutputPricePerToken
	cacheWrite := pricing.CacheCreationPricePerToken
	cacheRead := pricing.CacheReadPricePerToken
	if input == 0 && output == 0 && cacheWrite == 0 && cacheRead == 0 {
		return nil
	}
	result := &ChannelModelPricing{
		BillingMode: BillingModeToken,
		Intervals:   pricingIntervalsFromBilling(pricing),
	}
	if input != 0 {
		result.InputPrice = &input
	}
	if output != 0 {
		result.OutputPrice = &output
	}
	if cacheWrite != 0 {
		result.CacheWritePrice = &cacheWrite
	}
	if cacheRead != 0 {
		result.CacheReadPrice = &cacheRead
	}
	return result
}

// pricingIntervalsFromBilling 将回退价中的长上下文规则转为广场可展示的两个区间。
// 广场与计费复用一套基准，才能避免 GLM-5.1 只展示短上下文价格而实际请求进入长上下文档。
func pricingIntervalsFromBilling(pricing *ModelPricing) []PricingInterval {
	if pricing == nil || pricing.LongContextInputThreshold <= 0 ||
		(pricing.LongContextInputMultiplier <= 1 && pricing.LongContextOutputMultiplier <= 1 && pricing.LongContextCacheReadMultiplier <= 1) {
		return nil
	}

	shortMax := pricing.LongContextInputThreshold
	nextThreshold := shortMax + 1
	cacheReadMultiplier := pricing.LongContextCacheReadMultiplier
	if cacheReadMultiplier <= 0 {
		cacheReadMultiplier = pricing.LongContextInputMultiplier
	}
	shortLabel := fmt.Sprintf("输入 <%dK", nextThreshold/1000)
	longLabel := fmt.Sprintf("输入 >=%dK", nextThreshold/1000)

	return []PricingInterval{
		{
			MinTokens:       0,
			MaxTokens:       &shortMax,
			TierLabel:       shortLabel,
			InputPrice:      nonZeroPtr(pricing.InputPricePerToken),
			OutputPrice:     nonZeroPtr(pricing.OutputPricePerToken),
			CacheWritePrice: nonZeroPtr(pricing.CacheCreationPricePerToken),
			CacheReadPrice:  nonZeroPtr(pricing.CacheReadPricePerToken),
		},
		{
			MinTokens:       shortMax,
			TierLabel:       longLabel,
			InputPrice:      nonZeroPtr(pricing.InputPricePerToken * pricing.LongContextInputMultiplier),
			OutputPrice:     nonZeroPtr(pricing.OutputPricePerToken * pricing.LongContextOutputMultiplier),
			CacheWritePrice: nonZeroPtr(pricing.CacheCreationPricePerToken * pricing.LongContextInputMultiplier),
			CacheReadPrice:  nonZeroPtr(pricing.CacheReadPricePerToken * cacheReadMultiplier),
		},
	}
}

func plazaOfficialPricingFromBilling(pricing *ModelPricing) *PlazaOfficialPricing {
	if pricing == nil {
		return nil
	}
	result := &PlazaOfficialPricing{
		InputPrice:      nonZeroPtr(pricing.InputPricePerToken),
		OutputPrice:     nonZeroPtr(pricing.OutputPricePerToken),
		CacheWritePrice: nonZeroPtr(pricing.CacheCreationPricePerToken),
		CacheReadPrice:  nonZeroPtr(pricing.CacheReadPricePerToken),
		Intervals:       pricingIntervalsFromBilling(pricing),
	}
	if result.InputPrice == nil && result.OutputPrice == nil &&
		result.CacheWritePrice == nil && result.CacheReadPrice == nil {
		return nil
	}
	return result
}

// lookupOfficialPricing 查询模型的 LiteLLM 官方参考价，带 memo 避免同名模型重复转换。
// pricingService 为 nil（测试场景）或查不到时返回 nil。
func (s *ChannelService) lookupOfficialPricing(modelName string, memo map[string]*PlazaOfficialPricing) *PlazaOfficialPricing {
	if s.pricingService == nil {
		return nil
	}
	if cached, ok := memo[modelName]; ok {
		return cached
	}
	var result *PlazaOfficialPricing
	// Kimi、GLM、DeepSeek 的远程目录价格可能带上游展示换算或同步漂移。广场
	// 官方基准价必须与实际计费采用的校准基准保持一致，避免“官方价”和实付价不可比较。
	if s.plazaBillingService != nil && usesCalibratedFallbackPricing(modelName) {
		if pricing, err := s.plazaBillingService.GetModelPricing(modelName); err == nil {
			result = plazaOfficialPricingFromBilling(pricing)
		}
	}
	if result == nil {
		if lp := s.pricingService.GetModelPricing(modelName); lp != nil && !lp.TokenPricingAbsent {
			result = &PlazaOfficialPricing{
				InputPrice:        nonZeroPtr(lp.InputCostPerToken),
				OutputPrice:       nonZeroPtr(lp.OutputCostPerToken),
				CacheWritePrice:   nonZeroPtr(lp.CacheCreationInputTokenCost),
				CacheWrite1hPrice: nonZeroPtr(lp.CacheCreationInputTokenCostAbove1hr),
				CacheReadPrice:    nonZeroPtr(lp.CacheReadInputTokenCost),
			}
			if result.InputPrice == nil && result.OutputPrice == nil &&
				result.CacheWritePrice == nil && result.CacheWrite1hPrice == nil && result.CacheReadPrice == nil {
				result = nil
			}
		}
	}
	memo[modelName] = result
	return result
}
