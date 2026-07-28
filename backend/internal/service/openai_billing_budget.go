package service

import "encoding/json"

// OpenAIBillingBudgetPlan 是授权与预算器之间的不可变数据边界，避免仓储重新解析请求体。
type OpenAIBillingBudgetPlan struct {
	OriginalBody           []byte
	OutputLimitField       string
	ExplicitOutputLimit    bool
	RequestedOutputTokens  int
	DefaultOutputTokens    int
	MinimumOutputTokens    int
	FixedInputUSD          float64
	OutputUSDPerToken      float64
	ImageOutputUSDPerImage []float64
	PricingSnapshot        json.RawMessage
	EstimateBreakdown      OpenAIBillingEstimateBreakdown
}

type OpenAIBillingEstimateBreakdown struct {
	TextInputTokens  int     `json:"text_input_tokens"`
	ImageInputTokens int     `json:"image_input_tokens"`
	PDFInputTokens   int     `json:"pdf_input_tokens"`
	FixedInputUSD    float64 `json:"fixed_input_usd"`
	OutputUSD        float64 `json:"output_usd"`
	ImageOutputUSD   float64 `json:"image_output_usd"`
}
