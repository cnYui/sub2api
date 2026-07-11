# OpenAI GPT-5.4 / 5.5 / 5.6 Priority 计费审计

## 结论

当前 Sub2API 的 Standard 短上下文基础价与 OpenAI 官方现行价格一致，但 Fast / Priority 计费不一致。

- `fast` 会归一化为 `service_tier=priority`。
- 计费核心把所有 `priority` 请求统一乘 `1.5`。
- OpenAI 官方现行 Priority 价格不是统一 1.5 倍：GPT-5.4 为 Standard 的 2 倍，GPT-5.5 为 2.5 倍，GPT-5.6 三款为 2 倍。
- 因此当前 GPT-5.4、GPT-5.6 Priority 只收官方价格的 75%，少收 25%；GPT-5.5 Priority 只收官方价格的 60%，少收 40%。

本轮只读审计，未修改业务代码、运行态配置、数据库、Redis、nginx 或容器。

## 官方价格

单位：USD / 1M tokens。下表只列短上下文文本 token 价格。

| 模型 | Standard 输入 | Standard 缓存输入 | Standard 缓存写入 | Standard 输出 | Priority 输入 | Priority 缓存输入 | Priority 缓存写入 | Priority 输出 | Priority 倍率 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `gpt-5.4` | 2.5 | 0.25 | - | 15 | 5 | 0.5 | - | 30 | 2x |
| `gpt-5.5` | 5 | 0.5 | - | 30 | 12.5 | 1.25 | - | 75 | 2.5x |
| `gpt-5.6-sol` | 5 | 0.5 | 6.25 | 30 | 10 | 1 | 12.5 | 60 | 2x |
| `gpt-5.6-terra` | 2.5 | 0.25 | 3.125 | 15 | 5 | 0.5 | 6.25 | 30 | 2x |
| `gpt-5.6-luna` | 1 | 0.1 | 1.25 | 6 | 2 | 0.2 | 2.5 | 12 | 2x |

官方来源：

- https://developers.openai.com/api/docs/pricing
- https://developers.openai.com/api/docs/guides/priority-processing
- https://developers.openai.com/api/docs/guides/reasoning

## 当前实现

`backend/internal/service/billing_service.go` 定义全局 `openAIPriorityServiceTierMultiplier = 1.5`，`serviceTierCostMultiplier()` 对所有 `priority` 返回该倍率。`computeTokenBreakdown()` 对输入、输出、缓存写入、缓存读取和图片输出费用统一乘该倍率。

`GetModelPricing()` 虽然先读取动态价格数据中的 `*_priority` 字段，但返回前调用 `withPriorityServiceTierPrices()`，重新按基础价乘 1.5 覆盖 Priority 字段。实际扣费核心也只读取 Standard 单价后再乘统一倍率，因此远程价格表中的模型级 Priority 价格不会生效。

`backend/internal/service/openai_gateway_service.go` 会把入站 `service_tier=fast` 归一为 `priority`。运行态没有 `openai_fast_policy_settings`，默认策略为空规则，Fast / Priority 会透传并进入 1.5 倍计费。

## 运行态证据

当前公网容器为 `sub2api-candidate:20260710-214000-a575f28c1-admin-sub-revoke-repurchase`，对应提交中的计费代码同样使用全局 1.5 倍。

运行态无 `channels`、`channel_model_pricing`、`channel_pricing_intervals` 记录；相关 OpenAI 分组 `rate_multiplier` 均为 `1.0000`，没有额外渠道倍率干扰。

运行态 Priority usage 示例：

- `gpt-5.5`：输入 `7.5 USD/1M`、缓存输入 `0.75 USD/1M`、输出 `45 USD/1M`，严格等于 Standard 的 1.5 倍；官方应为 `12.5 / 1.25 / 75`。
- `gpt-5.6-sol`：输入 `7.5 USD/1M`、缓存输入 `0.75 USD/1M`、输出 `45 USD/1M`；官方应为 `10 / 1 / 60`。
- `gpt-5.6-terra`：输入 `3.75 USD/1M`、缓存输入 `0.375 USD/1M`、输出 `22.5 USD/1M`；官方应为 `5 / 0.5 / 30`。

历史 GPT-5.4 Priority 记录曾按 2 倍落库，但当前运行镜像代码已经统一为 1.5 倍；发布后的新 GPT-5.4 Priority 请求会按当前 1.5 倍逻辑计算。

## 其他风险

- GPT-5.5 运行态动态价格文件中的 Priority 仍为 Standard 的 2 倍，也落后于官方现行 2.5 倍；当前被统一 1.5 倍覆盖，所以尚未直接决定扣费，但价格源本身也需要校正。
- 官方说明 Priority 请求在高流量突增时可能降级为 `service_tier=default` 并按 Standard 收费。当前 `OpenAIForwardResult.ServiceTier` 来自请求体，未发现从上游响应读取实际 `service_tier` 覆盖计费值，因此降级场景可能仍按请求的 Priority 计费。
- 官方说明 Priority 暂不支持 long context。当前代码会先应用长上下文倍率，再应用 Priority 倍率，没有基于上游响应实际 tier 处理降级，长上下文 Priority 的账单口径存在额外偏差风险。
- reasoning tokens 已包含在 output tokens 内；当前计费按 `output_tokens` 一次扣费，没有额外叠加 reasoning tokens，这一点符合官方口径。

## 验证

执行目标单测：

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'Test(GetModelPricing_OpenAIGPT56FallbacksUseOfficialRatesAndPriority15x|CalculateCost_OpenAIGPT56PriorityIsOnePointFiveX|CalculateCostWithServiceTier_OpenAIPriorityUsesOnePointFiveMultiplier|OpenAIGatewayServiceRecordUsage_ServiceTierPriorityUsesOnePointFiveMultiplier|ApplyOpenAIFastPolicyToBody_DefaultPassesPriorityAndFast|NormalizeOpenAIServiceTier)$' -v
```

结果为 PASS。测试明确把全局 Priority 1.5 倍和 `fast -> priority` 当作当前预期，证明该行为不是偶发运行态数据问题，而是代码和测试共同固化的规则。
