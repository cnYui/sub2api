# GPT-5.6 思考程度与计费口径设计

## 背景

用户确认 SubTube API 后续需要支持 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna` 三个完整模型名，并追问这些模型的“思考程度”如何计费：

1. Advanced
2. Faster
3. Consumes Usage Limits Faster
4. Smarter

本轮只做官方资料核对与计费设计，不改业务代码、数据库或运行态配置。

## 官方资料结论

OpenAI API 文档中，GPT-5.6 的正式可控参数不是 `Advanced`、`Faster`、`Consumes Usage Limits Faster`、`Smarter` 这四个字符串，而是：

- `reasoning.effort`：GPT-5.6 支持 `none`、`low`、`medium`、`high`、`xhigh`、`max`。
- `reasoning.mode`：GPT-5.6 支持 `standard` 和 `pro`，默认 `standard`；`pro` 不是单独模型名，仍使用所选 GPT-5.6 模型。

官方 API 口径是 token 计费：

- `reasoning.effort` 控制模型思考量。低 effort 更偏速度和低 token，高 effort 通常延迟和 token 成本更高。
- reasoning tokens 对 API 不可见，但会占上下文窗口，并按 output tokens 计费。
- usage 里会在 `output_tokens_details.reasoning_tokens` 展示 reasoning token 数；这些 token 已包含在 `output_tokens` 内，计费时不能把 `output_tokens` 和 `reasoning_tokens` 再相加，否则会重复扣费。
- `reasoning.mode=pro` 会执行更多模型工作，但仍按所选模型的标准 token rates 计费，不是另换一个 `*-pro` 模型 slug。

官方价格页为 GPT-5.6 三个模型给的是模型维度、上下文维度、服务层维度的 token 单价，不是 reasoning 档位单价：

| 模型 | Standard 短上下文 input | cached input | cache writes | output |
| --- | ---: | ---: | ---: | ---: |
| `gpt-5.6-sol` | 5.00 | 0.50 | 6.25 | 30.00 |
| `gpt-5.6-terra` | 2.50 | 0.25 | 3.125 | 15.00 |
| `gpt-5.6-luna` | 1.00 | 0.10 | 1.25 | 6.00 |

单位均为 USD / 1M tokens。长上下文、Batch、Flex、Priority 有各自价格表；其中 Priority 不是思考程度，而是 API service tier。

## 四个展示文案如何理解

这四个更像产品 UI 文案或模型选择描述，不应作为 SubTube API 的独立计费维度：

- `Advanced`：可理解为更高级能力或更高 reasoning 配置的展示文案，但 API 不按 `Advanced` 额外加价。
- `Faster`：如果只是降低 reasoning effort 或选择更轻模型，仍按实际 usage token 计费；如果未来映射到 OpenAI API `service_tier=priority`，才应使用 Priority 价格表。
- `Consumes Usage Limits Faster`：这是额度消耗提示，不是固定倍率。它通常意味着该配置会产生更多 output/reasoning tokens，或使用更贵的 service tier，因此实际扣费更快。
- `Smarter`：同 `Advanced`，不是官方 API 价格档位；更强配置只通过更多 token 或更高模型单价体现成本。

要避免的错误设计：

- 不要在数据库中给 `Advanced/Faster/Smarter` 单独加价格列。
- 不要对 `high/xhigh/max` 人为套固定倍率。
- 不要将 `output_tokens_details.reasoning_tokens` 作为额外 output 再扣一次。
- 不要把 Codex/ChatGPT 的 credit、Fast mode 与 OpenAI API token 计费混成同一套规则。官方 Codex 文档明确：使用 API key 时 Codex 走标准 API pricing，不能使用 ChatGPT/Codex Fast mode credits。

## SubTube API 推荐计费方案

永久方案应按“上游 usage 事实”扣费：

```text
cost =
  input_uncached_tokens * input_price / 1_000_000
+ cached_input_tokens * cached_input_price / 1_000_000
+ cache_write_tokens * cache_write_price / 1_000_000
+ output_tokens * output_price / 1_000_000
+ tool_costs
```

其中：

- `output_tokens` 已包含 reasoning tokens。
- `reasoning_tokens` 只用于展示、审计、成本解释和风控，不直接参与额外扣费。
- `cache_write_tokens` 需要单独支持；GPT-5.6 官方价格表已有 cache writes 价格。
- `service_tier=flex` 或 Batch 使用对应折扣价；`service_tier=priority` 使用 Priority 表。
- 如果响应 usage 缺失，继续沿用现有保护策略：不能准确计费就不要假扣费，流式终止 usage 缺失仍应视为需要修复的链路问题。

## 对产品展示的建议

面向用户展示时，可以把官方参数和业务文案分开：

| 展示文案 | 推荐 API 映射 | 计费方式 |
| --- | --- | --- |
| Faster | `reasoning.effort=low` 或更低；如确需加速服务层，单独显式支持 `service_tier=priority` | 按实际 token；只有 Priority 才按 Priority 价格 |
| Balanced | `reasoning.effort=medium` | 按实际 token |
| Smarter / Advanced | `reasoning.effort=high` 或 `xhigh` | 按实际 token |
| Maximum / Consumes Usage Limits Faster | `reasoning.effort=max`，必要时 `reasoning.mode=pro` | 按实际 token，可能更贵但不设固定倍率 |

如果前端必须展示用户给出的四个词，建议用“标签/说明”承载，不要存成价格 key。真正落库字段应保存规范 API 值，例如 `reasoning_effort`、`reasoning_mode`、`service_tier`，便于审计和重放。

## 实施影响

后续实现 GPT-5.6 计费时，需要把上一份设计补充为：

- 模型价格数据增加 `cache_write` 单价，至少 GPT-5.6 三款要支持。
- usage log 如已有 JSON 原文，应保留 `input_tokens_details`、`output_tokens_details`、`service_tier`、`reasoning` 请求参数，方便解释为什么某次扣费高。
- 单测增加“reasoning_tokens 不重复扣费”的回归用例。
- 若支持 `reasoning.mode=pro`，只记录 mode 并按实际 token 计费，不使用 `gpt-5.6-pro` 价格。

## 官方来源

- OpenAI API Model guidance：GPT-5.6 模型名、reasoning effort、pro mode 与 cache writes 说明，`https://developers.openai.com/api/docs/guides/latest-model`
- OpenAI API Reasoning models：`reasoning.effort`、`reasoning.mode`、reasoning tokens 按 output tokens 计费，`https://developers.openai.com/api/docs/guides/reasoning`
- OpenAI API Pricing：GPT-5.6 三模型 Standard、Batch、Flex、Priority token 价格，`https://developers.openai.com/api/docs/pricing`
- OpenAI Codex Speed：使用 API key 时走标准 API pricing，不能使用 ChatGPT/Codex Fast mode credits，`https://developers.openai.com/codex/speed`
