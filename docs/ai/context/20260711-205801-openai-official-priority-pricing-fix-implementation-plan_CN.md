# OpenAI 官方 Priority 计费修复实施计划

> **执行要求：** 当前会话内联执行；严格按 TDD 红绿循环完成，不启用子代理。

**目标：** GPT-5.4 Priority 按 2x、GPT-5.5 按 2.5x、GPT-5.6 Sol/Terra/Luna 按 2x 计费，其他行为保持不变。

**架构：** 在 `BillingService` 内集中解析模型级 Priority 倍率，并让价格查询展示与实际 token 计费共用同一解析结果。目标模型使用官方倍率，其他模型保留现有 1.5x；Flex、Standard、长上下文和分组倍率不变。

**技术栈：** Go、Testify、内置 JSON 定价资源。

---

## 文件范围

- 修改 `backend/internal/service/billing_service.go`：增加模型级 Priority 倍率并贯通价格查询和实际计费。
- 修改 `backend/internal/service/billing_service_test.go`：覆盖 GPT-5.4、GPT-5.5、GPT-5.6 和范围外模型。
- 修改 `backend/internal/service/openai_gateway_record_usage_test.go`：验证 usage 记录路径使用官方倍率。
- 修改 `backend/internal/service/pricing_service_test.go`：验证内置价格资源。
- 修改 `backend/resources/model-pricing/model_prices_and_context_window.json`：同步官方 Priority 字段。
- 新增 `docs/ai/context/YYYYMMDD-HHMMSS-openai-official-priority-pricing-fix-result_CN.md`：记录实现与验证结果。
- 修改 `AGENTS.md`：写入最终结论。

### 任务 1：先写模型级 Priority 失败测试

**文件：**

- 修改：`backend/internal/service/billing_service_test.go`
- 修改：`backend/internal/service/openai_gateway_record_usage_test.go`

- [ ] **步骤 1：把旧的 1.5x 目标测试改成官方倍率测试**

新增表驱动断言：

```go
func TestOpenAIPriorityServiceTierMultiplierUsesOfficialModelRates(t *testing.T) {
	tests := []struct {
		model      string
		multiplier float64
	}{
		{model: "gpt-5.4", multiplier: 2.0},
		{model: "gpt-5.5", multiplier: 2.5},
		{model: "gpt-5.6-sol", multiplier: 2.0},
		{model: "gpt-5.6-terra", multiplier: 2.0},
		{model: "gpt-5.6-luna", multiplier: 2.0},
		{model: "gpt-5.4-mini", multiplier: 1.5},
		{model: "claude-sonnet-4", multiplier: 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			require.InDelta(t, tt.multiplier, serviceTierCostMultiplier(tt.model, "priority"), 1e-12)
		})
	}
}
```

把 GPT-5.6 fallback 价格测试改为断言 Priority `2x`，把 GPT-5.6 token 成本测试改为同时断言输入、缓存读取、缓存写入和输出均为 `2x`。

- [ ] **步骤 2：增加 GPT-5.5 2.5x 与 GPT-5.4 2x 成本测试**

```go
func TestCalculateCostWithServiceTier_OpenAIPriorityUsesOfficialModelMultiplier(t *testing.T) {
	tests := []struct {
		model      string
		multiplier float64
	}{
		{model: "gpt-5.4", multiplier: 2.0},
		{model: "gpt-5.5", multiplier: 2.5},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			tokens := UsageTokens{InputTokens: 100, OutputTokens: 50, CacheReadTokens: 20}
			baseCost, err := svc.CalculateCost(tt.model, tokens, 1.0)
			require.NoError(t, err)
			priorityCost, err := svc.CalculateCostWithServiceTier(tt.model, tokens, 1.0, "priority")
			require.NoError(t, err)
			require.InDelta(t, baseCost.TotalCost*tt.multiplier, priorityCost.TotalCost, 1e-10)
		})
	}
}
```

- [ ] **步骤 3：把 usage 记录测试改为 GPT-5.4 Priority 2x**

将 `TestOpenAIGatewayServiceRecordUsage_ServiceTierPriorityUsesOnePointFiveMultiplier` 重命名为官方倍率语义，并将期望输入/输出/总费用从 1.5x 改为 2x。

- [ ] **步骤 4：运行目标测试确认 RED**

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service \
  -run 'Test(OpenAIPriorityServiceTierMultiplierUsesOfficialModelRates|GetModelPricing_OpenAIGPT56FallbacksUseOfficialPriorityRates|CalculateCost_OpenAIGPT56PriorityUsesOfficialMultiplier|CalculateCostWithServiceTier_OpenAIPriorityUsesOfficialModelMultiplier|OpenAIGatewayServiceRecordUsage_ServiceTierPriorityUsesOfficialMultiplier)$' -v
```

预期：GPT-5.4、GPT-5.5、GPT-5.6 仍返回或扣除 1.5x，测试按期望值失败。

### 任务 2：实现模型级 Priority 倍率

**文件：**

- 修改：`backend/internal/service/billing_service.go`

- [ ] **步骤 1：增加模型级 Priority 倍率纯函数**

```go
func openAIPriorityServiceTierMultiplier(model string) float64 {
	switch normalizeKnownOpenAICodexModel(model) {
	case "gpt-5.5":
		return 2.5
	case "gpt-5.4", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna":
		return 2.0
	default:
		return defaultOpenAIPriorityServiceTierMultiplier
	}
}

func serviceTierCostMultiplier(model, serviceTier string) float64 {
	switch normalizeBillingServiceTier(serviceTier) {
	case "priority":
		return openAIPriorityServiceTierMultiplier(model)
	case "flex":
		return 0.5
	default:
		return 1.0
	}
}
```

未知 GPT-5.6 型号由现有归一函数返回空字符串，因此保留默认 1.5x，不误命中三款正式模型。

- [ ] **步骤 2：让价格查询使用同一模型级倍率**

将 helper 改为携带模型：

```go
func withPriorityServiceTierPrices(model string, pricing *ModelPricing) *ModelPricing
func applyPriorityServiceTierPrices(model string, pricing *ModelPricing)
```

三类 Priority 字段统一使用：

```go
multiplier := openAIPriorityServiceTierMultiplier(model)
pricing.InputPricePerTokenPriority = pricing.InputPricePerToken * multiplier
pricing.OutputPricePerTokenPriority = pricing.OutputPricePerToken * multiplier
pricing.CacheReadPricePerTokenPriority = pricing.CacheReadPricePerToken * multiplier
```

更新 `GetModelPricing()`、`GetModelPricingWithChannel()` 调用点。

- [ ] **步骤 3：让实际 token 计费携带模型**

将核心签名改为：

```go
func (s *BillingService) computeTokenBreakdown(
	model string, pricing *ModelPricing, tokens UsageTokens,
	rateMultiplier float64, serviceTier string,
	applyLongCtx bool,
) *CostBreakdown
```

并使用：

```go
tierMultiplier := serviceTierCostMultiplier(model, serviceTier)
```

更新统一计费和旧计费入口两个调用点，其他计算顺序不动。

- [ ] **步骤 4：运行目标测试确认 GREEN**

运行任务 1 的目标测试命令，预期全部 PASS。

### 任务 3：同步官方 Priority 定价资源

**文件：**

- 修改：`backend/resources/model-pricing/model_prices_and_context_window.json`
- 修改：`backend/internal/service/pricing_service_test.go`

- [ ] **步骤 1：先修改资源测试期望并确认 RED**

把 GPT-5.6 资源测试扩展为 Standard 与 Priority 价格矩阵：

```go
{model: "gpt-5.6-sol", input: 5e-6, cacheRead: 0.5e-6, cacheWrite: 6.25e-6, output: 30e-6, priorityMultiplier: 2.0}
{model: "gpt-5.6-terra", input: 2.5e-6, cacheRead: 0.25e-6, cacheWrite: 3.125e-6, output: 15e-6, priorityMultiplier: 2.0}
{model: "gpt-5.6-luna", input: 1e-6, cacheRead: 0.1e-6, cacheWrite: 1.25e-6, output: 6e-6, priorityMultiplier: 2.0}
```

增加 GPT-5.5 Priority 输入 `12.5e-6`、缓存读取 `1.25e-6`、输出 `75e-6` 断言。

运行：

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestDefaultPricingIncludesOfficialOpenAIPriorityRates' -v
```

预期：现有 GPT-5.5 与 GPT-5.6 Priority 字段不符合官方值，测试失败。

- [ ] **步骤 2：修改 JSON 官方价格**

目标值：

```text
gpt-5.5: input 12.5e-6, cache read 1.25e-6, output 75e-6
gpt-5.6-sol: input 10e-6, cache read 1e-6, cache write 12.5e-6, output 60e-6
gpt-5.6-terra: input 5e-6, cache read 0.5e-6, cache write 6.25e-6, output 30e-6
gpt-5.6-luna: input 2e-6, cache read 0.2e-6, cache write 2.5e-6, output 12e-6
```

GPT-5.4 已是官方 2x，保持现值。

- [ ] **步骤 3：运行资源测试确认 GREEN**

运行步骤 1 命令，预期 PASS；再运行：

```bash
python3 -m json.tool backend/resources/model-pricing/model_prices_and_context_window.json >/tmp/sub2api-openai-priority-pricing.json
```

预期 exit 0。

### 任务 4：回归验证和结果文档

**文件：**

- 新增：`docs/ai/context/YYYYMMDD-HHMMSS-openai-official-priority-pricing-fix-result_CN.md`
- 修改：`AGENTS.md`

- [ ] **步骤 1：运行完整相关测试**

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/pkg/openai ./internal/handler
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
git diff --check
```

预期全部 exit 0。

- [ ] **步骤 2：检查改动范围**

```bash
git status --short
git diff --stat
git diff -- backend/internal/service/billing_service.go \
  backend/internal/service/billing_service_test.go \
  backend/internal/service/openai_gateway_record_usage_test.go \
  backend/internal/service/pricing_service_test.go \
  backend/resources/model-pricing/model_prices_and_context_window.json
```

确认没有运行态文件、部署配置或无关模块改动。

- [ ] **步骤 3：写结果文档并更新项目记忆**

记录红绿证据、官方倍率、验证命令、未部署和未补扣历史 usage。

- [ ] **步骤 4：提交实现**

```bash
git add AGENTS.md backend/internal/service/billing_service.go \
  backend/internal/service/billing_service_test.go \
  backend/internal/service/openai_gateway_record_usage_test.go \
  backend/internal/service/pricing_service_test.go \
  backend/resources/model-pricing/model_prices_and_context_window.json \
  docs/ai/context/*openai-official-priority-pricing-fix-result_CN.md
git commit -m "fix: align OpenAI priority pricing with official rates"
```
