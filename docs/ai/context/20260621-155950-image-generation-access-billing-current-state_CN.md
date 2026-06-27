# 生图能力访问与计费当前状态

## 调查目标

确认当前 Sub2API 项目对图片生成的放行条件、计费方式，以及当前在售套餐是否可用生图。

## 代码侧规则

- 生图请求识别范围：
  - 独立图片接口：`/v1/images/generations`、`/v1/images/edits`，以及无 `/v1` 前缀变体。
  - `/v1/responses` 中请求模型是 `gpt-image-*`。
  - `/v1/responses` 请求体包含 `tools[].type = image_generation`。
  - `/v1/responses` 请求体通过 `tool_choice` 显式选择 `image_generation`。
- 放行条件：
  - API Key 所属分组必须满足 `groups.allow_image_generation = true`。
  - 不满足时返回 403，错误文案为 `Image generation is not enabled for this group`。
  - 放行后仍会经过内容审核、图片并发槽、订阅/余额/配额资格检查、上游账号调度。
- Codex 图片桥接：
  - `gateway.codex_image_generation_bridge_enabled` 默认是 `false`。
  - 默认不会把普通 Codex 文本请求自动注入 `image_generation` 工具。
  - 显式携带 `image_generation` 工具的请求仍按分组生图开关处理。

## 计费规则

- 图片生成优先按 `image_count` 和 `image_size` 计费，除非渠道定价明确配置为 token 模式。
- 图片尺寸归一为 `1K`、`2K`、`4K`：
  - 输出尺寸优先，其次请求输入尺寸。
  - 无法识别或未提供时默认按 `2K`。
- 图片单价优先级：
  1. 渠道级 pricing（per request / image 模式）。
  2. 分组自定义价格：`image_price_1k`、`image_price_2k`、`image_price_4k`。
  3. 默认价格：优先读取模型 `output_cost_per_image`，否则硬编码 base `0.134 USD`。
- 默认价格倍率：
  - `1K = base`
  - `2K = base * 1.5`
  - `4K = base * 2`
- 最终费用：
  - `ActualCost = 单价 * 图片数量 * 图片倍率`
  - 当前分组 `image_rate_independent=false` 时，图片倍率共享分组/用户有效倍率。
  - `image_rate_independent=true` 时，使用 `image_rate_multiplier`。
- 订阅套餐用户：
  - 费用写入 `SubscriptionCost = ActualCost`，消耗订阅分组日额度。
  - 例如开通后，29/39/59 元套餐会分别消耗对应的每日 19/29/49 USD 限额。
- 余额用户：
  - 费用写入 `BalanceCost = ActualCost`，直接扣余额。

## 当前运行态配置

只读查询本地后端 `http://127.0.0.1:18080/api/v1`，使用管理员 JWT 查询后台分组和套餐；未打印或记录 token、密码、API Key。

| 套餐 | 价格 | group_id | 分组 | 日额度 | allow_image_generation | 自定义图片价格 |
| --- | ---: | ---: | --- | ---: | --- | --- |
| 29 元订阅池 | 29 | 2 | `codex-pool-19-usd` | 19 USD | false | 未配置 |
| 39 元订阅池 | 39 | 3 | `codex-pool-29-usd` | 29 USD | false | 未配置 |
| 59 元订阅池 | 59 | 4 | `codex-pool-49-usd` | 49 USD | false | 未配置 |

本机自用分组 `codex-pool-local-unlimited` 当前 `allow_image_generation=false`，且无日额度限制、无自定义图片价格。

## 结论

- 当前代码支持 OpenAI 图片生成接口和 `/v1/responses` 的 `image_generation` 工具。
- 当前运行态没有任何在售套餐可以使用生图，因为三个在售分组的 `allow_image_generation` 都是 `false`。
- 即使 `/v1/models` 暴露 `gpt-image-*` 模型，分组开关关闭时仍会被网关拦截为 403。
- 若要开放生图，应在后台分组页只对目标档位打开「允许当前分组生图」，并按需要配置 `image_price_1k/2k/4k` 或渠道级图片定价。
