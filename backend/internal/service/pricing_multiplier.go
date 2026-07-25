package service

// normalizeUnitPriceMultiplier 把非法倍率兜底为 1，避免配置异常放大或缩小计费。
func normalizeUnitPriceMultiplier(multiplier float64) float64 {
	if multiplier <= 0 {
		return 1
	}
	return multiplier
}

func scaledFloat64Ptr(v *float64, multiplier float64) *float64 {
	if v == nil {
		return nil
	}
	scaled := *v * multiplier
	return &scaled
}

// scaleLiteLLMModelPricing 返回缩放后的 LiteLLM 定价拷贝。
func scaleLiteLLMModelPricing(pricing *LiteLLMModelPricing, multiplier float64) *LiteLLMModelPricing {
	if pricing == nil {
		return nil
	}
	multiplier = normalizeUnitPriceMultiplier(multiplier)
	if multiplier == 1 {
		return pricing
	}

	cloned := *pricing
	cloned.InputCostPerToken *= multiplier
	cloned.InputCostPerTokenPriority *= multiplier
	cloned.OutputCostPerToken *= multiplier
	cloned.OutputCostPerTokenPriority *= multiplier
	cloned.CacheCreationInputTokenCost *= multiplier
	cloned.CacheCreationInputTokenCostAbove1hr *= multiplier
	cloned.CacheReadInputTokenCost *= multiplier
	cloned.CacheReadInputTokenCostPriority *= multiplier
	cloned.OutputCostPerImage *= multiplier
	cloned.InputCostPerImageToken *= multiplier
	cloned.OutputCostPerImageToken *= multiplier
	return &cloned
}

// scaleModelPricing 返回缩放后的 Billing ModelPricing 拷贝。
func scaleModelPricing(pricing *ModelPricing, multiplier float64) *ModelPricing {
	if pricing == nil {
		return nil
	}
	multiplier = normalizeUnitPriceMultiplier(multiplier)
	if multiplier == 1 {
		return pricing
	}

	cloned := *pricing
	cloned.InputPricePerToken *= multiplier
	cloned.InputPricePerTokenPriority *= multiplier
	cloned.ImageInputPricePerToken *= multiplier
	cloned.OutputPricePerToken *= multiplier
	cloned.OutputPricePerTokenPriority *= multiplier
	cloned.CacheCreationPricePerToken *= multiplier
	cloned.CacheReadPricePerToken *= multiplier
	cloned.CacheReadPricePerTokenPriority *= multiplier
	cloned.CacheCreation5mPrice *= multiplier
	cloned.CacheCreation1hPrice *= multiplier
	cloned.ImageOutputPricePerToken *= multiplier
	return &cloned
}

// scaleChannelModelPricing 返回缩放后的渠道定价拷贝。
func scaleChannelModelPricing(pricing *ChannelModelPricing, multiplier float64) *ChannelModelPricing {
	if pricing == nil {
		return nil
	}
	multiplier = normalizeUnitPriceMultiplier(multiplier)
	if multiplier == 1 {
		return pricing
	}

	cloned := pricing.Clone()
	cloned.InputPrice = scaledFloat64Ptr(cloned.InputPrice, multiplier)
	cloned.OutputPrice = scaledFloat64Ptr(cloned.OutputPrice, multiplier)
	cloned.CacheWritePrice = scaledFloat64Ptr(cloned.CacheWritePrice, multiplier)
	cloned.CacheReadPrice = scaledFloat64Ptr(cloned.CacheReadPrice, multiplier)
	cloned.ImageOutputPrice = scaledFloat64Ptr(cloned.ImageOutputPrice, multiplier)
	cloned.PerRequestPrice = scaledFloat64Ptr(cloned.PerRequestPrice, multiplier)
	for i := range cloned.Intervals {
		cloned.Intervals[i].InputPrice = scaledFloat64Ptr(cloned.Intervals[i].InputPrice, multiplier)
		cloned.Intervals[i].OutputPrice = scaledFloat64Ptr(cloned.Intervals[i].OutputPrice, multiplier)
		cloned.Intervals[i].CacheWritePrice = scaledFloat64Ptr(cloned.Intervals[i].CacheWritePrice, multiplier)
		cloned.Intervals[i].CacheReadPrice = scaledFloat64Ptr(cloned.Intervals[i].CacheReadPrice, multiplier)
		cloned.Intervals[i].PerRequestPrice = scaledFloat64Ptr(cloned.Intervals[i].PerRequestPrice, multiplier)
	}
	return &cloned
}
