# OpenAI 预算单位与套餐优先修复结果

## 结果

已完成本地代码修复，未部署，未修改运行态 DB、Redis、容器或 Nginx。

## 变更

- `OpenAITrafficCreditBudgetEstimator` 不再使用 `len(body)` 作为 `input_tokens` 上界。
- 新增 JSON 文本估算：递归统计可作为文本输入的字符串值，跳过 `input_image`、`image_url`、`b64_json`、`file_id` 等图片/文件传输载荷。
- 无法解析 JSON 时保留旧的保守回退，继续按 body 字节兜底。
- active subscription 存在时，OpenAI 预授权使用修正后的预算检查套餐剩余额度；预算能放进套餐则返回 `BillingSourceSubscription`，预算真实超过套餐剩余时保留流量卡兜底。
- 补齐部分 unit 测试 stub 的 `GetActiveBySHA256Hash` 方法，避免当前接口扩展后 service 包测试无法编译。

## 线上用户解释

用户 `1510623550@qq.com` 的 402 不是套餐已用完：

- active 套餐：`user_subscriptions.id=110`，`codex-pool-69-usd`。
- 当日成功套餐用量约 `1.1014481 / 69 USD`。
- 失败请求 `body_bytes≈24,263,139`，错误为 `traffic credit is insufficient for request budget`。

旧预算公式把 JSON 请求体字节数当成 token：

`24,263,139 * 0.0000025 * 2.0 = 121.315695 USD`

其中 `0.0000025` 是 `gpt-5.6-terra` 输入单价，`2.0` 是长上下文输入倍率。这个 121 USD 是错误预算，不是实际费用。修复后，图片/base64 传输载荷不会再被当作文本 input token，用户这类请求会按修正后的预算优先走套餐。

## 验证

- `go test -tags unit ./internal/service -run 'TestOpenAITrafficCreditBudget_DoesNotPriceBase64ImageBytesAsTextTokens' -count=1`
- `go test -tags unit ./internal/service -run 'TestOpenAIBillingAuthorization|TestOpenAITrafficCreditBudget' -count=1`
- `go test -tags unit ./internal/service -count=1`

以上均通过。
