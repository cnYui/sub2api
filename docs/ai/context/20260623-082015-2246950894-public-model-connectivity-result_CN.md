# 2246950894 公网模型连通性排查结果

## 结论

`2246950894@qq.com` 的账号、API Key、订阅和公网 GPT 模型连通性正常。用户反馈的 `502 Bad Gateway` 确实出现过，根因不是 Key 失效或订阅额度不足，而是请求了当前公网模型列表之外的模型，主要是 `MiniMax-M3` 和 `glm-5.2`。

## 当前状态

- 用户：`users.id=15`，`2246950894@qq.com`，状态 `active`。
- API Key：`api_keys.id=36`，掩码 `sk-907dc...e83aec`，状态 `active`，绑定 `groups.id=2`。
- 分组：`codex-pool-19-usd`，状态 `active`，日限额 `19 USD`。
- 订阅：`user_subscriptions.id=9`，状态 `active`，有效期 `2026-06-17 00:00:00+08` 到 `2026-07-17 00:00:00+08`。
- 当前日用量：`0.6132017500 / 19 USD`。

## 公网验证

使用数据库中的 active API Key，仅在本机命令内发起请求，未在文档记录完整 Key。

- `GET https://api.aaccx.pw/v1/models`
  - HTTP `200`
  - 请求 ID：`4173c41a-73ec-4ca1-907b-f8f21039e0b2`
  - 返回 10 个模型，前 8 个为 `gpt-5.5`、`gpt-5.4`、`gpt-5.4-mini`、`gpt-5.3-codex`、`gpt-5.3-codex-spark`、`codex-auto-review`、`gpt-5.2`、`gpt-image-1`
- `POST https://api.aaccx.pw/v1/chat/completions`，模型 `gpt-5.4-mini`
  - HTTP `200`
  - 请求 ID：`78418949-6abd-42d5-81f9-1b8100795946`
  - 响应内容：`pong`
  - 落库：`usage_logs.id=9524`
- `POST https://api.aaccx.pw/v1/responses`，模型 `gpt-5.4-mini`
  - HTTP `200`
  - 请求 ID：`effb0bde-01d8-436f-898e-f994b331c5e0`
  - 落库：`usage_logs.id=9525`
- `POST https://api.aaccx.pw/v1/responses`，模型 `MiniMax-M3`
  - HTTP `502`
  - 响应体：`error code: 502`
  - 落库错误：`ops_error_logs.id=1972`
  - 错误详情：`unknown provider for model MiniMax-M3`
- `GET https://aaccx.pw/v1/models`
  - HTTP `200`
  - 请求 ID：`a2ff2bda-575f-4949-8113-8e85973004fe`
  - 返回 10 个模型

## 历史错误

该用户/API Key 的 `502` 记录按模型聚合：

- `glm-5.2`：106 次，时间 `2026-06-23 00:19:35+08` 到 `2026-06-23 00:25:58+08`，错误为 `unknown provider for model glm-5.2`。
- `MiniMax-M3`：49 次，时间 `2026-06-23 01:32:55+08` 到本次复现 `2026-06-23 07:19:22+08`，错误为 `unknown provider for model MiniMax-M3`。
- `gpt-5.5`：59 次，时间 `2026-06-22 11:44:51+08` 到 `2026-06-22 11:48:21+08`，错误为 `auth_unavailable: no auth available (providers=codex, model=gpt-5.5)`，属于当时上游认证/账号池不可用信号，不是当前本次复现的主因。

本次排查中 `2026-06-23 07:18:52+08` 附近有 3 条 `400 Request body is empty`，这是第一次手工测试脚本漏传 JSON body 造成的排查噪声，不代表用户请求问题。

## 判断

当前推荐用户使用 `/v1/models` 返回的模型，例如 `gpt-5.5`、`gpt-5.4`、`gpt-5.4-mini`。`MiniMax-M3` 和 `glm-5.2` 不在当前公网可用模型列表中，客户端如果选择这些模型，会在 Sub2API 上游路由阶段找不到可用 provider/account，最终表现为 `502 Bad Gateway`。
