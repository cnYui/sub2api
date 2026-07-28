package service

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"
)

const (
	OpenAIBillingEstimatorVersion = "openai-local-v1"
	OpenAIBillingHardCapUSD       = 2.0
	OpenAIBillingMinOutputTokens  = 256
)

var ErrOpenAIBillingBudgetExceedsHardCap = errors.New("openai billing budget exceeds hard cap")
var ErrOpenAIBillingBudgetInsufficient = errors.New("openai billing budget is insufficient")
var ErrOpenAIBillingBudgetInvalidPlan = errors.New("openai billing budget plan is invalid")

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

type OpenAIBillingBudgetFit struct {
	EffectiveBody            []byte
	EffectiveImageCount      int
	EffectiveMaxOutputTokens int
	ReserveUSD               float64
	PricingSnapshot          json.RawMessage
	EstimateBreakdown        json.RawMessage
}

// FitOpenAIBillingBudget 在单一资金来源的原子可用额度内生成实际上游请求的预算。
func FitOpenAIBillingBudget(plan OpenAIBillingBudgetPlan, availableUSD float64, mode BillingAuthorizationReserveMode) (OpenAIBillingBudgetFit, error) {
	requestedOutputTokens := plan.requestedOutputTokens()
	minimumOutputTokens := plan.minimumOutputTokens()
	fixedCostUSD, imageOutputUSD, err := plan.fixedCostUSD()
	if err != nil {
		return OpenAIBillingBudgetFit{}, err
	}
	fullCostUSD := fixedCostUSD + float64(requestedOutputTokens)*plan.OutputUSDPerToken
	if fullCostUSD > OpenAIBillingHardCapUSD && mode == BillingAuthorizationReserveFull {
		return OpenAIBillingBudgetFit{}, ErrOpenAIBillingBudgetExceedsHardCap
	}

	capUSD := math.Min(OpenAIBillingHardCapUSD, math.Max(0, availableUSD))
	if fullCostUSD <= capUSD {
		return plan.newFit(requestedOutputTokens, fixedCostUSD, imageOutputUSD)
	}
	if mode == BillingAuthorizationReserveFull || plan.ExplicitOutputLimit || plan.OutputUSDPerToken <= 0 {
		return OpenAIBillingBudgetFit{}, ErrOpenAIBillingBudgetInsufficient
	}

	availableOutputUSD := capUSD - fixedCostUSD
	if availableOutputUSD < 0 {
		return OpenAIBillingBudgetFit{}, ErrOpenAIBillingBudgetInsufficient
	}
	effectiveOutputTokens := int(math.Floor(availableOutputUSD/plan.OutputUSDPerToken + 1e-9))
	if effectiveOutputTokens > requestedOutputTokens {
		effectiveOutputTokens = requestedOutputTokens
	}
	if effectiveOutputTokens < minimumOutputTokens {
		return OpenAIBillingBudgetFit{}, ErrOpenAIBillingBudgetInsufficient
	}

	return plan.newFit(effectiveOutputTokens, fixedCostUSD, imageOutputUSD)
}

func (p OpenAIBillingBudgetPlan) requestedOutputTokens() int {
	if p.RequestedOutputTokens > 0 {
		return p.RequestedOutputTokens
	}
	if p.DefaultOutputTokens > 0 {
		return p.DefaultOutputTokens
	}
	return 0
}

func (p OpenAIBillingBudgetPlan) minimumOutputTokens() int {
	if p.MinimumOutputTokens > 0 {
		return p.MinimumOutputTokens
	}
	return OpenAIBillingMinOutputTokens
}

func (p OpenAIBillingBudgetPlan) fixedCostUSD() (float64, float64, error) {
	if p.FixedInputUSD < 0 || p.OutputUSDPerToken < 0 {
		return 0, 0, ErrOpenAIBillingBudgetInvalidPlan
	}

	imageOutputUSD := 0.0
	for _, costUSD := range p.ImageOutputUSDPerImage {
		if costUSD < 0 {
			return 0, 0, ErrOpenAIBillingBudgetInvalidPlan
		}
		imageOutputUSD += costUSD
	}
	return p.FixedInputUSD + imageOutputUSD, imageOutputUSD, nil
}

func (p OpenAIBillingBudgetPlan) newFit(outputTokens int, fixedCostUSD, imageOutputUSD float64) (OpenAIBillingBudgetFit, error) {
	body, err := p.withOutputLimit(outputTokens)
	if err != nil {
		return OpenAIBillingBudgetFit{}, err
	}

	breakdown := p.EstimateBreakdown
	breakdown.FixedInputUSD = p.FixedInputUSD
	breakdown.OutputUSD = float64(outputTokens) * p.OutputUSDPerToken
	breakdown.ImageOutputUSD = imageOutputUSD
	breakdownJSON, err := json.Marshal(breakdown)
	if err != nil {
		return OpenAIBillingBudgetFit{}, err
	}

	return OpenAIBillingBudgetFit{
		EffectiveBody:            body,
		EffectiveImageCount:      len(p.ImageOutputUSDPerImage),
		EffectiveMaxOutputTokens: outputTokens,
		ReserveUSD:               roundOpenAIBillingUSD(fixedCostUSD + breakdown.OutputUSD),
		PricingSnapshot:          append(json.RawMessage(nil), p.PricingSnapshot...),
		EstimateBreakdown:        breakdownJSON,
	}, nil
}

func (p OpenAIBillingBudgetPlan) withOutputLimit(outputTokens int) ([]byte, error) {
	if p.ExplicitOutputLimit || p.OutputLimitField == "" || outputTokens <= 0 {
		return append([]byte(nil), p.OriginalBody...), nil
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(p.OriginalBody, &body); err != nil {
		return nil, ErrOpenAIBillingBudgetInvalidPlan
	}
	encodedTokens, err := json.Marshal(outputTokens)
	if err != nil {
		return nil, err
	}
	body[p.OutputLimitField] = encodedTokens
	return json.Marshal(body)
}

func roundOpenAIBillingUSD(value float64) float64 {
	return math.Round(value*1e10) / 1e10
}

// EstimateOpenAITextInputTokens 保守估算 JSON 协议中的可计费文本与结构开销。
func EstimateOpenAITextInputTokens(body []byte) (int, error) {
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return 0, ErrOpenAIBillingBudgetInvalidPlan
	}

	tokens := estimateOpenAIJSONTokens(decoded, "")
	if tokens < 1 {
		return 1, nil
	}
	return tokens, nil
}

func estimateOpenAIJSONTokens(value any, field string) int {
	switch typed := value.(type) {
	case map[string]any:
		tokens := 2
		for key, child := range typed {
			tokens += 1 + estimateOpenAIStringTokens(key)
			tokens += estimateOpenAIJSONTokens(child, key)
		}
		return tokens
	case []any:
		tokens := 1
		for _, child := range typed {
			tokens += estimateOpenAIJSONTokens(child, field)
		}
		return tokens
	case string:
		if isOpenAIBinaryPayload(field, typed) {
			return 1
		}
		return 1 + estimateOpenAIStringTokens(typed)
	case float64, bool, nil:
		return 1
	default:
		return 1
	}
}

func estimateOpenAIStringTokens(value string) int {
	characters := utf8.RuneCountInString(strings.TrimSpace(value))
	if characters == 0 {
		return 0
	}
	return int(math.Ceil(float64(characters) / 3))
}

func isOpenAIBinaryPayload(field, value string) bool {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "file_data", "b64_json", "image_base64":
		return true
	case "image_url", "url":
		return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "data:")
	default:
		return false
	}
}
