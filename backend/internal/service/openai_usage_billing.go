package service

import "strings"

type OpenAIUsagePresence struct {
	Input         bool `json:"input"`
	CacheCreation bool `json:"cache_creation"`
	CacheRead     bool `json:"cache_read"`
	Output        bool `json:"output"`
	ImageInput    bool `json:"image_input"`
	ImageOutput   bool `json:"image_output"`
}

type OpenAIUsageExpectation struct {
	Input       bool `json:"input"`
	CacheRead   bool `json:"cache_read"`
	Output      bool `json:"output"`
	ImageInput  bool `json:"image_input"`
	ImageOutput bool `json:"image_output"`
}

type OpenAIBillingComponent struct {
	Kind   string      `json:"kind"`
	Model  string      `json:"model"`
	Tokens UsageTokens `json:"tokens"`
}

type OpenAIModelPricingSnapshot struct {
	Source                     string  `json:"source"`
	InputPricePerToken         float64 `json:"input_price_per_token"`
	ImageInputPricePerToken    float64 `json:"image_input_price_per_token"`
	OutputPricePerToken        float64 `json:"output_price_per_token"`
	CacheCreationPricePerToken float64 `json:"cache_creation_price_per_token"`
	CacheReadPricePerToken     float64 `json:"cache_read_price_per_token"`
	ImageOutputPricePerToken   float64 `json:"image_output_price_per_token"`
}

type OpenAIBillingComponentSnapshot struct {
	Component OpenAIBillingComponent     `json:"component"`
	Pricing   OpenAIModelPricingSnapshot `json:"pricing"`
	Cost      CostBreakdown              `json:"cost"`
}

type OpenAIUsageBillingSnapshot struct {
	Components             []OpenAIBillingComponentSnapshot `json:"components"`
	ServiceTier            string                           `json:"service_tier"`
	RateMultiplier         float64                          `json:"rate_multiplier"`
	MissingUsageComponents []string                         `json:"missing_usage_components"`
	BillingIncomplete      bool                             `json:"billing_incomplete"`
}

func BuildOpenAIBillingComponents(usage OpenAIUsage, mainBillingModel, imageBillingModel string) []OpenAIBillingComponent {
	mainModel := strings.TrimSpace(mainBillingModel)
	imageModel := strings.TrimSpace(imageBillingModel)
	if imageModel == "" {
		imageModel = "gpt-image-2"
	}

	mainInput := maxInt(usage.InputTokens-usage.CacheReadInputTokens-usage.ImageInputTokens, 0)
	textOutput := maxInt(usage.OutputTokens-usage.ImageOutputTokens, 0)

	components := make([]OpenAIBillingComponent, 0, 2)
	if mainModel != "" || mainInput > 0 || usage.CacheCreationInputTokens > 0 || usage.CacheReadInputTokens > 0 || textOutput > 0 {
		components = append(components, OpenAIBillingComponent{
			Kind:  "main",
			Model: mainModel,
			Tokens: UsageTokens{
				InputTokens:         mainInput,
				OutputTokens:        textOutput,
				CacheCreationTokens: usage.CacheCreationInputTokens,
				CacheReadTokens:     usage.CacheReadInputTokens,
			},
		})
	}
	if usage.ImageInputTokens > 0 || usage.ImageOutputTokens > 0 {
		components = append(components, OpenAIBillingComponent{
			Kind:  "image",
			Model: imageModel,
			Tokens: UsageTokens{
				ImageInputTokens:  usage.ImageInputTokens,
				ImageOutputTokens: usage.ImageOutputTokens,
			},
		})
	}
	return components
}

func MergeCostBreakdowns(costs ...*CostBreakdown) *CostBreakdown {
	merged := &CostBreakdown{}
	for _, cost := range costs {
		if cost == nil {
			continue
		}
		merged.InputCost += cost.InputCost
		merged.ImageInputCost += cost.ImageInputCost
		merged.OutputCost += cost.OutputCost
		merged.ImageOutputCost += cost.ImageOutputCost
		merged.CacheCreationCost += cost.CacheCreationCost
		merged.CacheReadCost += cost.CacheReadCost
		merged.TotalCost += cost.TotalCost
		merged.ActualCost += cost.ActualCost
		if merged.BillingMode == "" && cost.BillingMode != "" {
			merged.BillingMode = cost.BillingMode
		}
	}
	return merged
}

func MissingOpenAIUsageComponents(expect OpenAIUsageExpectation, presence OpenAIUsagePresence) []string {
	missing := make([]string, 0, 5)
	if expect.Input && !presence.Input {
		missing = append(missing, "input_tokens")
	}
	if expect.CacheRead && !presence.CacheRead {
		missing = append(missing, "cache_read_tokens")
	}
	if expect.Output && !presence.Output {
		missing = append(missing, "output_tokens")
	}
	if expect.ImageInput && !presence.ImageInput {
		missing = append(missing, "image_input_tokens")
	}
	if expect.ImageOutput && !presence.ImageOutput {
		missing = append(missing, "image_output_tokens")
	}
	return missing
}

func HasBillableOpenAIUsage(usage OpenAIUsage) bool {
	return usage.InputTokens > 0 ||
		usage.CacheCreationInputTokens > 0 ||
		usage.CacheReadInputTokens > 0 ||
		usage.OutputTokens > 0 ||
		usage.ImageInputTokens > 0 ||
		usage.ImageOutputTokens > 0
}

func maxInt(value, floor int) int {
	if value < floor {
		return floor
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
