# GPT-5.6 完整模型名与 Priority 1.5x 计费 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 永久支持 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna` 的模型展示、原样调用与官方 token 计费，并确保只有 `service_tier=priority` 使用 1.5 倍计费，其它思考程度/文案均按 1x 和官方基础定价计算。

**Architecture:** 模型展示继续由 `openai.DefaultModels` 提供默认列表；动态价格优先走 `PricingService` 的本地/远程 JSON，失败后走 `BillingService` 硬编码 fallback。计费不新增每模型数据库列，继续按 usage token、cache token、service tier 和现有 usage/cost 字段记录。

**Tech Stack:** Go 后端、Gin handler、内置 JSON 定价资源、现有 `BillingService` / `PricingService` / OpenAI gateway 单测，按 TDD 实施。

---

## 最高约束

- 只展示并调用完整模型名：`gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`。
- 不展示裸 `gpt-5.6` alias，也不鼓励调用。
- `Advanced`、`Faster`、`Consumes Usage Limits Faster`、`Smarter` 不作为计费 key。
- 除 `service_tier=priority` 外，所有 reasoning effort / reasoning mode 都按 1x 与官方 token 单价计算。
- `service_tier=priority` 统一按 1.5x 计费；不是 2x。
- `output_tokens_details.reasoning_tokens` 已包含在 `output_tokens` 内，只能用于展示/审计，不能重复扣费。
- 数据库不新增每模型字段；现有 `usage_logs.*`、`channel_model_pricing.*`、`channel_pricing_intervals` 足够承载。

## 文件结构

- Create: `backend/internal/pkg/openai/models_test.go`：验证默认模型列表包含三款完整模型，不包含裸 `gpt-5.6`。
- Modify: `backend/internal/pkg/openai/constants.go`：在默认模型列表中加入三款 GPT-5.6。
- Modify: `backend/internal/handler/gateway_models_test.go`：验证 OpenAI 分组默认 fallback `/v1/models` 返回三款 GPT-5.6。
- Modify: `backend/internal/service/openai_model_alias.go`：支持 `gpt5.6-sol` 等紧凑写法；阻止裸 `gpt-5.6` 和未知 `gpt-5.6-*` 回退到 GPT-5.4。
- Modify: `backend/internal/service/openai_compat_model_test.go`：增加 GPT-5.6 alias/归一化测试。
- Modify: `backend/internal/service/pricing_service.go`：解析 long context 字段；避免 GPT-5.6 泛型价格误匹配。
- Modify: `backend/internal/service/pricing_service_test.go`：验证 long context 解析、内置 JSON、priority 1.5x、未知 GPT-5.6 不误回退。
- Modify: `backend/resources/model-pricing/model_prices_and_context_window.json`：增加三款 GPT-5.6 定价资源；不增加裸 `gpt-5.6` 条目。
- Modify: `backend/internal/service/billing_service.go`：增加三款 GPT-5.6 fallback 定价；长上下文策略覆盖 GPT-5.6；保持 priority 1.5x。
- Modify: `backend/internal/service/billing_service_test.go`：验证三款 fallback、priority 1.5x、long context、reasoning tokens 不重复扣费。
- Optionally Modify: `backend/internal/service/openai_gateway_service_test.go`：若缺少 usage 解析覆盖，补 `reasoning_tokens` 不叠加到 `OutputTokens` 的回归测试。
- Modify: `AGENTS.md`：记录本轮实施计划结论。

---

### Task 1: 默认模型列表支持完整 GPT-5.6

**Files:**
- Create: `backend/internal/pkg/openai/models_test.go`
- Modify: `backend/internal/pkg/openai/constants.go`

- [ ] **Step 1: 写失败测试**

在 `backend/internal/pkg/openai/models_test.go` 新建：

```go
//go:build unit

package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultModelsIncludeGPT56CompleteNamesOnly(t *testing.T) {
	ids := DefaultModelIDs()

	require.Contains(t, ids, "gpt-5.6-sol")
	require.Contains(t, ids, "gpt-5.6-terra")
	require.Contains(t, ids, "gpt-5.6-luna")
	require.NotContains(t, ids, "gpt-5.6")
}

func TestDefaultModelsGPT56DisplayNames(t *testing.T) {
	byID := make(map[string]Model, len(DefaultModels))
	for _, model := range DefaultModels {
		byID[model.ID] = model
	}

	require.Equal(t, "GPT-5.6 Sol", byID["gpt-5.6-sol"].DisplayName)
	require.Equal(t, "GPT-5.6 Terra", byID["gpt-5.6-terra"].DisplayName)
	require.Equal(t, "GPT-5.6 Luna", byID["gpt-5.6-luna"].DisplayName)
	require.Equal(t, "openai", byID["gpt-5.6-sol"].OwnedBy)
	require.Equal(t, "model", byID["gpt-5.6-sol"].Object)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/pkg/openai
```

Expected: `TestDefaultModelsIncludeGPT56CompleteNamesOnly` 因缺少三款 GPT-5.6 失败。

- [ ] **Step 3: 最小实现**

在 `backend/internal/pkg/openai/constants.go` 的 `DefaultModels` 顶部、`gpt-5.5` 前加入：

```go
	{ID: "gpt-5.6-sol", Object: "model", Created: 1783641600, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Sol"},
	{ID: "gpt-5.6-terra", Object: "model", Created: 1783641600, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Terra"},
	{ID: "gpt-5.6-luna", Object: "model", Created: 1783641600, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Luna"},
```

不加入 `gpt-5.6`。

- [ ] **Step 4: 运行测试确认通过**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/pkg/openai
```

Expected: PASS。

---

### Task 2: `/v1/models` 默认 fallback 返回三款 GPT-5.6

**Files:**
- Modify: `backend/internal/handler/gateway_models_test.go`

- [ ] **Step 1: 写失败测试**

在 `TestGatewayModels_CustomModelsListFiltersDefaultFallbackModels` 后增加：

```go
func TestGatewayModels_OpenAIDefaultFallbackIncludesGPT56CompleteNamesOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(28)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {{ID: 1, Platform: service.PlatformOpenAI}},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	ids := modelIDsForTest(got.Data)
	require.Contains(t, ids, "gpt-5.6-sol")
	require.Contains(t, ids, "gpt-5.6-terra")
	require.Contains(t, ids, "gpt-5.6-luna")
	require.NotContains(t, ids, "gpt-5.6")
}
```

- [ ] **Step 2: 运行测试**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run 'TestGatewayModels'
```

Expected: Task 1 前目标测试失败；Task 1 后所有 gateway models 测试 PASS。

---

### Task 3: GPT-5.6 模型名归一化只接受完整名

**Files:**
- Modify: `backend/internal/service/openai_model_alias.go`
- Modify: `backend/internal/service/openai_compat_model_test.go`

- [ ] **Step 1: 写失败测试**

在 `TestNormalizeOpenAICompatRequestedModel` 中追加 cases：

```go
		{name: "gpt56 sol compact spelling", input: "gpt5.6-sol", want: "gpt-5.6-sol"},
		{name: "gpt56 terra compact spelling", input: "openai/gpt5.6terra", want: "gpt-5.6-terra"},
		{name: "gpt56 luna compact spelling", input: "gpt_5.6_luna", want: "gpt-5.6-luna"},
		{name: "bare gpt56 is not rewritten to gpt54", input: "gpt-5.6", want: "gpt-5.6"},
```

新增测试：

```go
func TestNormalizeKnownOpenAICodexModelGPT56CompleteNamesOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "sol", input: "gpt-5.6-sol", want: "gpt-5.6-sol"},
		{name: "terra", input: "gpt5.6terra", want: "gpt-5.6-terra"},
		{name: "luna", input: "openai/gpt_5.6_luna", want: "gpt-5.6-luna"},
		{name: "bare alias blocked", input: "gpt-5.6", want: ""},
		{name: "unknown suffix blocked", input: "gpt-5.6-unknown", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, normalizeKnownOpenAICodexModel(tt.input))
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestNormalize(OpenAICompatRequestedModel|KnownOpenAICodexModelGPT56)'
```

Expected: 紧凑写法或裸 `gpt-5.6` 相关断言失败。

- [ ] **Step 3: 实现归一化**

在 `canonicalizeOpenAIModelAliasSpelling()` 的 replacements 中补：

```go
		{"gpt-5.6sol", "gpt-5.6-sol"},
		{"gpt-5.6terra", "gpt-5.6-terra"},
		{"gpt-5.6luna", "gpt-5.6-luna"},
```

在 `normalizeKnownOpenAICodexModel()` 的 `switch` 最前面加入：

```go
	case strings.Contains(normalized, "gpt-5.6-sol"):
		return "gpt-5.6-sol"
	case strings.Contains(normalized, "gpt-5.6-terra"):
		return "gpt-5.6-terra"
	case strings.Contains(normalized, "gpt-5.6-luna"):
		return "gpt-5.6-luna"
	case strings.Contains(normalized, "gpt-5.6"):
		return ""
```

这段必须放在 `strings.Contains(normalized, "gpt-5") -> "gpt-5.4"` 之前。

- [ ] **Step 4: 运行测试确认通过**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestNormalize(OpenAICompatRequestedModel|KnownOpenAICodexModelGPT56)'
```

Expected: PASS。

---

### Task 4: PricingService 解析 long context 并拒绝 GPT-5.6 泛型误回退

**Files:**
- Modify: `backend/internal/service/pricing_service.go`
- Modify: `backend/internal/service/pricing_service_test.go`

- [ ] **Step 1: 写 long context 解析失败测试**

在 `backend/internal/service/pricing_service_test.go` 增加：

```go
func TestParsePricingData_PreservesLongContextFields(t *testing.T) {
	svc := &PricingService{}
	pricingData, err := svc.parsePricingData([]byte(`{
		"gpt-5.6-sol": {
			"input_cost_per_token": 0.000005,
			"output_cost_per_token": 0.00003,
			"cache_read_input_token_cost": 0.0000005,
			"long_context_input_token_threshold": 272000,
			"long_context_input_cost_multiplier": 2.0,
			"long_context_output_cost_multiplier": 1.5,
			"litellm_provider": "openai",
			"mode": "chat"
		}
	}`))
	require.NoError(t, err)

	pricing := pricingData["gpt-5.6-sol"]
	require.NotNil(t, pricing)
	require.Equal(t, 272000, pricing.LongContextInputTokenThreshold)
	require.InDelta(t, 2.0, pricing.LongContextInputCostMultiplier, 1e-12)
	require.InDelta(t, 1.5, pricing.LongContextOutputCostMultiplier, 1e-12)
}
```

- [ ] **Step 2: 写泛型误回退失败测试**

```go
func TestGetModelPricing_GPT56DoesNotFallbackToGenericAlias(t *testing.T) {
	generic := &LiteLLMModelPricing{InputCostPerToken: 99e-6}
	sol := &LiteLLMModelPricing{InputCostPerToken: 5e-6}

	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.6":     generic,
			"gpt-5.6-sol": sol,
		},
	}

	require.Same(t, sol, svc.GetModelPricing("gpt-5.6-sol"))
	require.Same(t, sol, svc.GetModelPricing("gpt-5.6-sol-20260710"))
	require.Nil(t, svc.GetModelPricing("gpt-5.6-terra"))
	require.Nil(t, svc.GetModelPricing("gpt-5.6-unknown"))
}
```

- [ ] **Step 3: 运行测试确认失败**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestParsePricingData_PreservesLongContextFields|TestGetModelPricing_GPT56DoesNotFallbackToGenericAlias'
```

Expected: long context 字段为 0，或 `gpt-5.6-terra` 被错误匹配到 `gpt-5.6`。

- [ ] **Step 4: 实现解析字段**

在 `LiteLLMRawEntry` 增加：

```go
	LongContextInputTokenThreshold  *int     `json:"long_context_input_token_threshold"`
	LongContextInputCostMultiplier  *float64 `json:"long_context_input_cost_multiplier"`
	LongContextOutputCostMultiplier *float64 `json:"long_context_output_cost_multiplier"`
```

在 `parsePricingData()` 中赋值：

```go
		if entry.LongContextInputTokenThreshold != nil {
			pricing.LongContextInputTokenThreshold = *entry.LongContextInputTokenThreshold
		}
		if entry.LongContextInputCostMultiplier != nil {
			pricing.LongContextInputCostMultiplier = *entry.LongContextInputCostMultiplier
		}
		if entry.LongContextOutputCostMultiplier != nil {
			pricing.LongContextOutputCostMultiplier = *entry.LongContextOutputCostMultiplier
		}
```

- [ ] **Step 5: 实现 GPT-5.6 安全回退**

在 `generateOpenAIModelVariants()` 中，`withoutDate` 添加后、基础版本号 fallback 前加入：

```go
	if strings.HasPrefix(model, "gpt-5.6-") {
		filtered := variants[:0]
		for _, variant := range variants {
			if variant == "gpt-5.6-sol" || variant == "gpt-5.6-terra" || variant == "gpt-5.6-luna" {
				filtered = append(filtered, variant)
			}
		}
		return filtered
	}
	if model == "gpt-5.6" {
		return variants
	}
```

- [ ] **Step 6: 运行测试确认通过**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestParsePricingData_PreservesLongContextFields|TestGetModelPricing_GPT56DoesNotFallbackToGenericAlias'
```

Expected: PASS。

---

### Task 5: 内置价格资源写入三款 GPT-5.6，priority 为 1.5x

**Files:**
- Modify: `backend/resources/model-pricing/model_prices_and_context_window.json`
- Modify: `backend/internal/service/pricing_service_test.go`

- [ ] **Step 1: 写资源验证失败测试**

在 `pricing_service_test.go` 增加：

```go
func TestDefaultPricingIncludesGPT56ModelsWithPriority15x(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)

	svc := &PricingService{}
	pricingData, err := svc.parsePricingData(data)
	require.NoError(t, err)

	tests := []struct {
		model      string
		input      float64
		cacheRead  float64
		cacheWrite float64
		output     float64
	}{
		{model: "gpt-5.6-sol", input: 5e-6, cacheRead: 0.5e-6, cacheWrite: 6.25e-6, output: 30e-6},
		{model: "gpt-5.6-terra", input: 2.5e-6, cacheRead: 0.25e-6, cacheWrite: 3.125e-6, output: 15e-6},
		{model: "gpt-5.6-luna", input: 1e-6, cacheRead: 0.1e-6, cacheWrite: 1.25e-6, output: 6e-6},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pricing := pricingData[tt.model]
			require.NotNil(t, pricing)
			require.InDelta(t, tt.input, pricing.InputCostPerToken, 1e-12)
			require.InDelta(t, tt.input*1.5, pricing.InputCostPerTokenPriority, 1e-12)
			require.InDelta(t, tt.cacheRead, pricing.CacheReadInputTokenCost, 1e-12)
			require.InDelta(t, tt.cacheRead*1.5, pricing.CacheReadInputTokenCostPriority, 1e-12)
			require.InDelta(t, tt.cacheWrite, pricing.CacheCreationInputTokenCost, 1e-12)
			require.InDelta(t, tt.output, pricing.OutputCostPerToken, 1e-12)
			require.InDelta(t, tt.output*1.5, pricing.OutputCostPerTokenPriority, 1e-12)
			require.Equal(t, 272000, pricing.LongContextInputTokenThreshold)
			require.InDelta(t, 2.0, pricing.LongContextInputCostMultiplier, 1e-12)
			require.InDelta(t, 1.5, pricing.LongContextOutputCostMultiplier, 1e-12)
			require.True(t, pricing.SupportsServiceTier)
			require.True(t, pricing.SupportsPromptCaching)
		})
	}

	require.Nil(t, pricingData["gpt-5.6"])
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run TestDefaultPricingIncludesGPT56ModelsWithPriority15x
```

Expected: 三款模型缺失导致 FAIL。

- [ ] **Step 3: 增加 JSON 资源**

在 OpenAI GPT 模型区域加入 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna` 三条。关键字段必须为：

```json
"gpt-5.6-sol": {
  "cache_creation_input_token_cost": 6.25e-06,
  "cache_read_input_token_cost": 5e-07,
  "cache_read_input_token_cost_flex": 2.5e-07,
  "cache_read_input_token_cost_priority": 7.5e-07,
  "input_cost_per_token": 5e-06,
  "input_cost_per_token_flex": 2.5e-06,
  "input_cost_per_token_priority": 7.5e-06,
  "long_context_input_token_threshold": 272000,
  "long_context_input_cost_multiplier": 2.0,
  "long_context_output_cost_multiplier": 1.5,
  "litellm_provider": "openai",
  "mode": "chat",
  "output_cost_per_token": 3e-05,
  "output_cost_per_token_flex": 1.5e-05,
  "output_cost_per_token_priority": 4.5e-05,
  "supports_prompt_caching": true,
  "supports_reasoning": true,
  "supports_service_tier": true
}
```

`gpt-5.6-terra` 价格字段：

```json
"cache_creation_input_token_cost": 3.125e-06,
"cache_read_input_token_cost": 2.5e-07,
"cache_read_input_token_cost_flex": 1.25e-07,
"cache_read_input_token_cost_priority": 3.75e-07,
"input_cost_per_token": 2.5e-06,
"input_cost_per_token_flex": 1.25e-06,
"input_cost_per_token_priority": 3.75e-06,
"output_cost_per_token": 1.5e-05,
"output_cost_per_token_flex": 7.5e-06,
"output_cost_per_token_priority": 2.25e-05
```

`gpt-5.6-luna` 价格字段：

```json
"cache_creation_input_token_cost": 1.25e-06,
"cache_read_input_token_cost": 1e-07,
"cache_read_input_token_cost_flex": 5e-08,
"cache_read_input_token_cost_priority": 1.5e-07,
"input_cost_per_token": 1e-06,
"input_cost_per_token_flex": 5e-07,
"input_cost_per_token_priority": 1.5e-06,
"output_cost_per_token": 6e-06,
"output_cost_per_token_flex": 3e-06,
"output_cost_per_token_priority": 9e-06
```

不加入 `"gpt-5.6"` 条目。

- [ ] **Step 4: 校验 JSON 和资源测试**

Run:

```bash
python3 -m json.tool backend/resources/model-pricing/model_prices_and_context_window.json >/tmp/sub2api-model-pricing.json
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run TestDefaultPricingIncludesGPT56ModelsWithPriority15x
```

Expected: JSON 解析成功，测试 PASS。

---

### Task 6: BillingService fallback 支持三款 GPT-5.6 和 priority 1.5x

**Files:**
- Modify: `backend/internal/service/billing_service.go`
- Modify: `backend/internal/service/billing_service_test.go`

- [ ] **Step 1: 写 fallback、priority、未知模型测试**

在 `billing_service_test.go` 的 GPT-5.4 fallback 测试附近增加：

```go
func TestGetModelPricing_OpenAIGPT56FallbacksUseOfficialRatesAndPriority15x(t *testing.T) {
	svc := newTestBillingService()

	tests := []struct {
		model      string
		input      float64
		cacheRead  float64
		cacheWrite float64
		output     float64
	}{
		{model: "gpt-5.6-sol", input: 5e-6, cacheRead: 0.5e-6, cacheWrite: 6.25e-6, output: 30e-6},
		{model: "gpt-5.6-terra", input: 2.5e-6, cacheRead: 0.25e-6, cacheWrite: 3.125e-6, output: 15e-6},
		{model: "gpt-5.6-luna", input: 1e-6, cacheRead: 0.1e-6, cacheWrite: 1.25e-6, output: 6e-6},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pricing, err := svc.GetModelPricing(tt.model)
			require.NoError(t, err)
			require.InDelta(t, tt.input, pricing.InputPricePerToken, 1e-12)
			require.InDelta(t, tt.input*1.5, pricing.InputPricePerTokenPriority, 1e-12)
			require.InDelta(t, tt.cacheRead, pricing.CacheReadPricePerToken, 1e-12)
			require.InDelta(t, tt.cacheRead*1.5, pricing.CacheReadPricePerTokenPriority, 1e-12)
			require.InDelta(t, tt.cacheWrite, pricing.CacheCreationPricePerToken, 1e-12)
			require.InDelta(t, tt.output, pricing.OutputPricePerToken, 1e-12)
			require.InDelta(t, tt.output*1.5, pricing.OutputPricePerTokenPriority, 1e-12)
			require.Equal(t, 272000, pricing.LongContextInputThreshold)
			require.InDelta(t, 2.0, pricing.LongContextInputMultiplier, 1e-12)
			require.InDelta(t, 1.5, pricing.LongContextOutputMultiplier, 1e-12)
		})
	}
}

func TestGetModelPricing_OpenAIGPT56UnknownDoesNotFallbackToOtherGPT(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := svc.GetModelPricing("gpt-5.6-unknown")
	require.ErrorIs(t, err, ErrModelPricingUnavailable)
	require.Nil(t, pricing)
}
```

- [ ] **Step 2: 写 long context 与 reasoning 不重复扣费测试**

```go
func TestCalculateCost_OpenAIGPT56LongContextUsesOfficialMultipliers(t *testing.T) {
	svc := newTestBillingService()
	tokens := UsageTokens{InputTokens: 273000, CacheReadTokens: 1000, CacheCreationTokens: 2000, OutputTokens: 100}

	cost, err := svc.CalculateCost("gpt-5.6-sol", tokens, 1.0)
	require.NoError(t, err)

	require.InDelta(t, float64(tokens.InputTokens)*5e-6*2.0, cost.InputCost, 1e-10)
	require.InDelta(t, float64(tokens.CacheReadTokens)*0.5e-6*2.0, cost.CacheReadCost, 1e-10)
	require.InDelta(t, float64(tokens.CacheCreationTokens)*6.25e-6*2.0, cost.CacheCreationCost, 1e-10)
	require.InDelta(t, float64(tokens.OutputTokens)*30e-6*1.5, cost.OutputCost, 1e-10)
}

func TestCalculateCost_OpenAIGPT56PriorityIsOnePointFiveX(t *testing.T) {
	svc := newTestBillingService()
	tokens := UsageTokens{InputTokens: 100, OutputTokens: 50, CacheCreationTokens: 40, CacheReadTokens: 20}

	baseCost, err := svc.CalculateCost("gpt-5.6-terra", tokens, 1.0)
	require.NoError(t, err)
	priorityCost, err := svc.CalculateCostWithServiceTier("gpt-5.6-terra", tokens, 1.0, "priority")
	require.NoError(t, err)

	require.InDelta(t, baseCost.TotalCost*1.5, priorityCost.TotalCost, 1e-10)
	require.InDelta(t, baseCost.ActualCost*1.5, priorityCost.ActualCost, 1e-10)
}

func TestCalculateCost_OpenAIGPT56OutputTokensChargedOnceWhenTheyIncludeReasoning(t *testing.T) {
	svc := newTestBillingService()

	// 假设上游 usage.output_tokens=100，其中 output_tokens_details.reasoning_tokens=80。
	// BillingService 只接收 output_tokens=100，不能按 180 计费。
	tokens := UsageTokens{InputTokens: 10, OutputTokens: 100}

	cost, err := svc.CalculateCost("gpt-5.6-luna", tokens, 1.0)
	require.NoError(t, err)

	require.InDelta(t, 10e-6, cost.InputCost, 1e-12)
	require.InDelta(t, 600e-6, cost.OutputCost, 1e-12)
	require.InDelta(t, 610e-6, cost.TotalCost, 1e-12)
}
```

- [ ] **Step 3: 运行测试确认失败**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'Test(GetModelPricing_OpenAIGPT56|CalculateCost_OpenAIGPT56)'
```

Expected: fallback 定价缺失导致 FAIL。

- [ ] **Step 4: 实现 fallback 定价**

在 `billing_service.go` 中增加 helper：

```go
func openAIGPT56Pricing(input, cacheRead, cacheWrite, output float64) *ModelPricing {
	return &ModelPricing{
		InputPricePerToken:          input,
		OutputPricePerToken:         output,
		CacheCreationPricePerToken:  cacheWrite,
		CacheReadPricePerToken:      cacheRead,
		SupportsCacheBreakdown:      false,
		LongContextInputThreshold:   openAIGPT54LongContextInputThreshold,
		LongContextInputMultiplier:  openAIGPT54LongContextInputMultiplier,
		LongContextOutputMultiplier: openAIGPT54LongContextOutputMultiplier,
	}
}
```

在 `initFallbackPricing()` 的 OpenAI GPT 区域加入：

```go
	s.fallbackPrices["gpt-5.6-sol"] = openAIGPT56Pricing(5e-6, 0.5e-6, 6.25e-6, 30e-6)
	s.fallbackPrices["gpt-5.6-terra"] = openAIGPT56Pricing(2.5e-6, 0.25e-6, 3.125e-6, 15e-6)
	s.fallbackPrices["gpt-5.6-luna"] = openAIGPT56Pricing(1e-6, 0.1e-6, 1.25e-6, 6e-6)
```

在 `getFallbackPricing()` 的 OpenAI `switch normalized` 中加入三款模型 case。

把 `isOpenAIGPT54Model()` 改名为 `isOpenAILongContextPricedModel()`，并返回：

```go
	return normalized == "gpt-5.6-sol" ||
		normalized == "gpt-5.6-terra" ||
		normalized == "gpt-5.6-luna" ||
		normalized == "gpt-5.4" ||
		normalized == "gpt-5.5"
```

- [ ] **Step 5: 运行测试确认通过**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'Test(GetModelPricing_OpenAIGPT56|CalculateCost_OpenAIGPT56)'
```

Expected: PASS。

---

### Task 7: usage 解析层确认 reasoning tokens 不参与额外扣费

**Files:**
- Modify: `backend/internal/service/openai_gateway_service_test.go`

- [ ] **Step 1: 判断是否已有等价测试**

Run:

```bash
rg -n "reasoning_tokens|output_tokens_details" backend/internal/service/*_test.go
```

如果已有测试明确断言 `output_tokens_details.reasoning_tokens` 不增加 `OutputTokens`，跳过本任务后续步骤；否则继续。

- [ ] **Step 2: 写 usage 解析回归测试**

在 `openai_gateway_service_test.go` 的 usage extraction 测试附近增加：

```go
func TestExtractOpenAIUsageFromJSONBytes_ReasoningTokensRemainInsideOutputTokens(t *testing.T) {
	usage, ok := extractOpenAIUsageFromJSONBytes([]byte(`{
		"id": "resp_reasoning",
		"usage": {
			"input_tokens": 10,
			"output_tokens": 100,
			"total_tokens": 110,
			"output_tokens_details": {"reasoning_tokens": 80}
		}
	}`))

	require.True(t, ok)
	require.Equal(t, 10, usage.InputTokens)
	require.Equal(t, 100, usage.OutputTokens)
}
```

- [ ] **Step 3: 运行测试**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run TestExtractOpenAIUsageFromJSONBytes_ReasoningTokensRemainInsideOutputTokens
```

Expected: PASS。若失败，修 usage extraction，只取 `usage.output_tokens`，不要把 `output_tokens_details.reasoning_tokens` 加到 `OutputTokens`。

---

### Task 8: 全量目标回归

**Files:**
- No source edits unless tests expose a real regression.

- [ ] **Step 1: 运行 OpenAI 模型与计费相关单测**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/pkg/openai ./internal/handler ./internal/service
```

Expected: PASS。

- [ ] **Step 2: 运行 server 编译门**

Run:

```bash
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
```

Expected: PASS。

- [ ] **Step 3: 检查补丁格式**

Run:

```bash
git diff --check
```

Expected: 无输出，exit code 0。

---

### Task 9: 发布前真实链路验收计划

**Files:**
- No code edits.

- [ ] **Step 1: 构建并替换应用容器**

沿用当前 18084 候选发布流程，只替换 `sub2api-candidate` 应用容器，不重建 Postgres/Redis/nginx/Cloudflare Tunnel。

- [ ] **Step 2: 验证健康检查**

Run:

```bash
curl -fsS http://127.0.0.1:18084/health
curl -fsS http://127.0.0.1:8080/health
```

Expected: 两者返回健康状态。

- [ ] **Step 3: 验证 `/v1/models`**

Run:

```bash
curl -fsS https://api.aaccx.pw/v1/models \
  -H "Authorization: Bearer $SUB2API_TEST_KEY"
```

Expected: 包含 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`，不包含裸 `gpt-5.6`。

- [ ] **Step 4: 三款模型最小真实请求**

分别请求：

```bash
curl -fsS https://api.aaccx.pw/v1/responses \
  -H "Authorization: Bearer $SUB2API_TEST_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-5.6-sol","input":"只回复 OK","reasoning":{"effort":"low"}}'
```

将 `model` 分别改为 `gpt-5.6-terra`、`gpt-5.6-luna` 再执行。

Expected: HTTP 200；DB `usage_logs` 中 `requested_model/model/upstream_model/total_cost` 有记录且 cost > 0。

- [ ] **Step 5: priority 真实扣费抽样**

Run:

```bash
curl -fsS https://api.aaccx.pw/v1/responses \
  -H "Authorization: Bearer $SUB2API_TEST_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-5.6-luna","input":"只回复 OK","service_tier":"priority","reasoning":{"effort":"low"}}'
```

Expected: `usage_logs.service_tier=priority`，同等 token 下计费口径为基础价 1.5x；不是 2x。

---

## 自检

- Spec 覆盖：完整模型名展示、调用、官方定价、priority 1.5x、reasoning 1x、数据库不新增列均有对应任务。
- 占位扫描：计划中没有未定义的占位实现；每个代码修改任务都给了目标测试和最小代码方向。
- 类型一致性：使用现有 `ModelPricing`、`LiteLLMModelPricing`、`UsageTokens`、`CostInput.ServiceTier`，不引入新业务类型。
- 风险控制：不展示 `gpt-5.6` alias，不让三款差异模型回退到泛型价格，不重复计费 reasoning tokens。

## 执行选择

Plan complete and saved to `docs/ai/context/20260710-095810-gpt56-models-priority15-billing-implementation-plan_CN.md`. Two execution options:

1. Subagent-Driven (recommended) - dispatch a fresh subagent per task, review between tasks, fast iteration.
2. Inline Execution - execute tasks in this session using executing-plans, batch execution with checkpoints.
