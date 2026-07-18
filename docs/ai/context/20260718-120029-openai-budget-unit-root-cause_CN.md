# OpenAI 预算估算单位错误根因

## 结论

`/v1/responses` 的 OpenAI 预授权预算把 `len(body)` 当作 `input_tokens` 上界使用。`body` 是 HTTP JSON 请求体字节，不是模型 token。请求体里可能包含 JSON 字段名、结构、转义、base64 图片或其它传输载荷，这些不能按文本 token 计费。

## 为什么 24MB 会变成 100 多美元

失败请求日志中的 `body_bytes≈24,263,139` 来自 `content_moderation_helper.go` 的 `len(body)`。预算估算器在 `openai_traffic_credit_budget.go` 中执行：

- `inputTokens := len(body)`
- `gpt-5.6-terra` 输入单价约 `0.0000025 USD/token`
- 超过 `272000` token 后触发长上下文输入倍率 `2.0`

因此仅输入预算约为：

`24,263,139 * 0.0000025 * 2.0 = 121.315695 USD`

这不是实际模型使用成本，而是把传输字节误当 token 的结果。

## 修复边界

- 预算估算应从 JSON 中提取可计入模型文本输入的字符串值。
- `input_image.image_url`、Chat Completions `image_url.url`、`b64_json` 等图片载荷不应作为文本 token 统计。
- 无法解析 JSON 时保留旧的保守回退，避免放开未知格式。
- active 套餐仍优先计费，不能被保守预算导向流量卡。

## 测试

新增回归测试：包含 1MB base64 `input_image.image_url` 的 Responses 请求，只有短文本输入时，预算不应因 base64 字节而返回 `ErrTrafficCreditInsufficient`。
