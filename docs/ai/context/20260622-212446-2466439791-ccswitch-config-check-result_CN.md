# 2466439791@qq.com CCSwitch 配置核对结果

## 结论

截图中的 CCSwitch 配置里，Base URL 和 API Key 本身没有问题。

- Base URL：`https://api.aaccx.pw/v1`，正确。
- API Key：与 `2466439791@qq.com` 当前 active Key 匹配，掩码为 `sk-263e6...ec751d`。
- `auth.json` 中的 `OPENAI_API_KEY` 与上方 API Key 输入框一致。

截图中已经暴露完整 API Key，后续不应继续公开传播；如已经发到公开群或外部渠道，建议轮换 Key。

## 数据库核对

- 用户：`users.id=38`
- 邮箱：`2466439791@qq.com`
- API Key：`api_keys.id=43`
- Key 掩码：`sk-263e6...ec751d`
- Key 长度：`67`
- Key 状态：`active`
- 分组：`codex-pool-19-usd`
- 订阅状态：`active`
- 订阅到期：`2026-07-22 15:38:30.544797+08`

## Responses API 验证

使用该 Key 通过公网入口请求：

`POST https://api.aaccx.pw/v1/responses`

结果：

- HTTP：`200`
- 模型：`gpt-5.4-mini-2026-03-17`
- 响应内容：`pong`
- 响应头请求 ID：`f7bd03ac-048c-4ff7-89bc-aad072747599`

## 判断

如果 CCSwitch 仍显示 `request timed out`，当前更可能不是 Key 或 Base URL 填错，而是：

1. CCSwitch 当前保存的配置没有真正生效，需要保存后切换到该供应商再重试。
2. 请求使用了更大的上下文或 `gpt-5.5`，上游响应慢，客户端本地超时时间过短。
3. 客户端走了本地路由映射或代理，实际请求没有按截图配置走 `https://api.aaccx.pw/v1`。

建议下一步让用户提供 CCSwitch 的实际请求日志、模型名、请求时间和错误详情；不要再发送完整 API Key。
