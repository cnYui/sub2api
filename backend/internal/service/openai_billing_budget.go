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
	ImageCountField        string
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
	fixedInputUSD, err := plan.fixedInputCostUSD()
	if err != nil {
		return OpenAIBillingBudgetFit{}, err
	}
	fullImageCount := len(plan.ImageOutputUSDPerImage)
	fullImageOutputUSD := plan.imageOutputCostUSD(fullImageCount)
	fullCostUSD := fixedInputUSD + fullImageOutputUSD + float64(requestedOutputTokens)*plan.OutputUSDPerToken
	if fullCostUSD > OpenAIBillingHardCapUSD && mode == BillingAuthorizationReserveFull {
		return OpenAIBillingBudgetFit{}, ErrOpenAIBillingBudgetExceedsHardCap
	}

	capUSD := math.Min(OpenAIBillingHardCapUSD, math.Max(0, availableUSD))
	if fullCostUSD <= capUSD {
		return plan.newFit(requestedOutputTokens, fullImageCount, fixedInputUSD, fullImageOutputUSD)
	}
	if mode == BillingAuthorizationReserveFull {
		return OpenAIBillingBudgetFit{}, ErrOpenAIBillingBudgetInsufficient
	}

	minimumImageCount := 0
	if fullImageCount > 0 {
		minimumImageCount = 1
	}
	for imageCount := fullImageCount; imageCount >= minimumImageCount; imageCount-- {
		if imageCount < fullImageCount && plan.ImageCountField == "" {
			break
		}
		imageOutputUSD := plan.imageOutputCostUSD(imageCount)
		availableOutputUSD := capUSD - fixedInputUSD - imageOutputUSD
		if availableOutputUSD < 0 {
			continue
		}

		effectiveOutputTokens := requestedOutputTokens
		if float64(effectiveOutputTokens)*plan.OutputUSDPerToken > availableOutputUSD {
			if plan.ExplicitOutputLimit || plan.OutputUSDPerToken <= 0 {
				continue
			}
			effectiveOutputTokens = int(math.Floor(availableOutputUSD/plan.OutputUSDPerToken + 1e-9))
		}
		if effectiveOutputTokens < minimumOutputTokens && requestedOutputTokens > 0 {
			continue
		}
		return plan.newFit(effectiveOutputTokens, imageCount, fixedInputUSD, imageOutputUSD)
	}

	return OpenAIBillingBudgetFit{}, ErrOpenAIBillingBudgetInsufficient
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

func (p OpenAIBillingBudgetPlan) fixedInputCostUSD() (float64, error) {
	if p.FixedInputUSD < 0 || p.OutputUSDPerToken < 0 {
		return 0, ErrOpenAIBillingBudgetInvalidPlan
	}

	for _, costUSD := range p.ImageOutputUSDPerImage {
		if costUSD < 0 {
			return 0, ErrOpenAIBillingBudgetInvalidPlan
		}
	}
	return p.FixedInputUSD, nil
}

func (p OpenAIBillingBudgetPlan) imageOutputCostUSD(imageCount int) float64 {
	if imageCount <= 0 {
		return 0
	}
	if imageCount > len(p.ImageOutputUSDPerImage) {
		imageCount = len(p.ImageOutputUSDPerImage)
	}
	costUSD := 0.0
	for _, imageCostUSD := range p.ImageOutputUSDPerImage[:imageCount] {
		costUSD += imageCostUSD
	}
	return costUSD
}

func (p OpenAIBillingBudgetPlan) newFit(outputTokens, imageCount int, fixedInputUSD, imageOutputUSD float64) (OpenAIBillingBudgetFit, error) {
	body, err := p.withEffectiveLimits(outputTokens, imageCount)
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
		EffectiveImageCount:      imageCount,
		EffectiveMaxOutputTokens: outputTokens,
		ReserveUSD:               roundOpenAIBillingUSD(fixedInputUSD + imageOutputUSD + breakdown.OutputUSD),
		PricingSnapshot:          append(json.RawMessage(nil), p.PricingSnapshot...),
		EstimateBreakdown:        breakdownJSON,
	}, nil
}

func (p OpenAIBillingBudgetPlan) withEffectiveLimits(outputTokens, imageCount int) ([]byte, error) {
	updateOutput := !p.ExplicitOutputLimit && p.OutputLimitField != "" && outputTokens > 0
	updateImageCount := p.ImageCountField != "" && imageCount >= 0 && imageCount != len(p.ImageOutputUSDPerImage)
	if !updateOutput && !updateImageCount {
		return append([]byte(nil), p.OriginalBody...), nil
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(p.OriginalBody, &body); err != nil {
		return nil, ErrOpenAIBillingBudgetInvalidPlan
	}
	if updateOutput {
		encodedTokens, err := json.Marshal(outputTokens)
		if err != nil {
			return nil, err
		}
		body[p.OutputLimitField] = encodedTokens
	}
	if updateImageCount {
		encodedCount, err := json.Marshal(imageCount)
		if err != nil {
			return nil, err
		}
		body[p.ImageCountField] = encodedCount
	}
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

// EstimateOpenAIAttachmentInputTokens 将已解析附件换算为文本、图片和 PDF 的输入预算组件。
func EstimateOpenAIAttachmentInputTokens(model string, inspection OpenAIAttachmentInspection) (int, int) {
	imageTokens := 0
	for _, imageInput := range inspection.Images {
		imageTokens += estimateOpenAIImageInputTokens(model, imageInput)
	}

	pdfTokens := 0
	for _, pdfInput := range inspection.PDFs {
		pdfTokens += pdfInput.TextTokens
		for _, page := range pdfInput.Pages {
			pdfTokens += estimateOpenAIImageInputTokens(model, page)
		}
	}
	return imageTokens, pdfTokens
}

func estimateOpenAIImageInputTokens(_ string, imageInput OpenAIImageInput) int {
	if imageInput.Width <= 0 || imageInput.Height <= 0 {
		return 0
	}
	if strings.EqualFold(strings.TrimSpace(imageInput.Detail), "low") {
		return 85
	}

	width := imageInput.Width
	height := imageInput.Height
	if width > 2048 || height > 2048 {
		scale := math.Min(2048/float64(width), 2048/float64(height))
		width = int(math.Ceil(float64(width) * scale))
		height = int(math.Ceil(float64(height) * scale))
	}
	tiles := int(math.Ceil(float64(width)/512)) * int(math.Ceil(float64(height)/512))
	if tiles < 1 {
		tiles = 1
	}
	return 85 + tiles*170
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
