# OpenAI 套餐优先预授权修复计划

## 背景

用户 `1510623550@qq.com` 反馈 OpenAI `/v1/responses` 返回 402。只读排查运行态 `sub2api-candidate` 与 `sub2api-candidate-postgres` 后确认：

- 用户 `id=41`，状态 active，API Key `id=45`。
- 当前 active 套餐 `user_subscriptions.id=110`，分组 `codex-pool-69-usd`，每日额度 `69 USD`。
- 2026-07-18 今日套餐成功计费约 `1.1014481 USD`，未超过每日额度。
- 10:25 起失败请求日志显示 `body_bytes≈24MB`，错误为 `traffic credit is insufficient for request budget`，HTTP 402。

## 根因判断

`body_bytes` 是网关读取到的原始 JSON 请求体长度，由 `len(body)` 写入日志；它不是模型真实 token 用量。预授权估算器在 `openai_traffic_credit_budget.go` 中同样使用 `inputTokens := len(body)` 作为输入 token 上界。对携带图片/base64/大输入的 OpenAI Responses 请求，这会把请求体字节数误当成 token 数，导致预算严重放大。

当前 `OpenAIBillingAuthorizationService.Authorize()` 的行为是：

1. 有 active subscription 时，先用预算估算值检查套餐限额。
2. 如果估算预算不能放进套餐剩余额度，就继续进入流量卡预授权。
3. 用户流量卡只剩 `0.00346455 USD`，因此返回 402。

这违反业务定论：active 套餐应优先计费；不能因为保守预算估算过大，就把套餐用户导向流量卡。

## 修复方案

采用最小行为修复：

- 修正 OpenAI 预算估算单位：不能把 JSON 请求体字节数当作模型 input token。
- active subscription + subscription group 存在时，使用修正后的预算检查套餐剩余额度；预算能放进套餐时来源为 `subscription`。
- 修正后的预算确实超过套餐剩余时，保留原有流量卡兜底逻辑。
- 无 active subscription 时，保留余额与流量卡预授权逻辑。
- 实际结算仍使用上游返回的真实 `ActualCost`，继续由 `usage_billing` 写入唯一计费来源。

## 测试

- 新增回归测试：包含 base64 图片载荷的大 JSON 请求不能把图片传输字节计入文本 input token 预算。
- 保留原有无套餐/余额/流量卡路径测试。

## 风险与边界

- 本次不修改运行态 DB、Redis、容器或 Nginx。
- 本次不引入真实 tokenizer，文本 token 上界仍按文本字符串长度保守估算。
- 如果将来需要严格防止单次请求真实费用超过套餐剩余额度，应单独设计“套餐预留/债务/并发预算”机制。
