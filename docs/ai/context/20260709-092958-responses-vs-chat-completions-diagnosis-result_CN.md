# /responses 与 /chat/completions 当前问题排查结果

## 结论

当前公网 18084 的正式模型 API 入口是 `/v1/*`，裸 `/responses` 与裸 `/chat/completions` 返回 `400 INVALID_BASE_URL` 是预期策略，不是故障。

真正导致 `/v1/responses` 非 200 的最新可复现原因是请求体混用了 Chat Completions 格式：把 `messages` 字段发到了 Responses API。上游返回 `400 Unsupported parameter: messages`，Sub2API 当前包装成 `502 Upstream request failed`。

## 复现与验证

- `POST /responses`：`400 INVALID_BASE_URL`。
- `POST /chat/completions`：`400 INVALID_BASE_URL`。
- `POST /v1/responses`，请求体使用 Responses 格式 `input`：HTTP 200。
- `POST /v1/chat/completions`，请求体使用 Chat Completions 格式 `messages`：HTTP 200。
- `POST /v1/responses`，请求体错误使用 `messages`：HTTP 502；后台 `ops_system_logs` 中真实原因是 `upstream error: 400 message=Unsupported parameter: messages`。

本轮使用 `api_keys.id=32` 的 active Key 做最小真实请求，未输出完整 Key：

- `usage_logs.id=71712`：`inbound_endpoint=/v1/responses`，`upstream_endpoint=/v1/responses`，HTTP 200。
- `usage_logs.id=71713`：`inbound_endpoint=/v1/chat/completions`，`upstream_endpoint=/v1/responses`，HTTP 200。
- 错误请求未落 `usage_logs`。

## 日志观察

近两小时成功请求汇总：

- `/v1/responses -> /v1/responses`：132 条成功用量记录。
- `/v1/chat/completions -> /v1/responses`：26 条成功用量记录。

近两小时失败主要分两类：

- 早些时候 `07:37-07:46 +08` 有一批 `/v1/responses` 502，错误为 `invalid base_url: invalid url scheme: http`；当前账号运行态已能 200，不是最新阻塞。
- 最新可复现失败为 `/v1/responses` 请求体带 `messages`，上游拒绝 `Unsupported parameter: messages`。

## 使用建议

- Codex / Responses 模式：`base_url=https://api.aaccx.pw/v1`，路径使用 `/responses`，请求体使用 Responses API 的 `input`。
- Chat Completions 客户端：`base_url=https://api.aaccx.pw/v1`，路径使用 `/chat/completions`，请求体使用 `messages`。
- 不要把 `messages` 发送到 `/v1/responses`；如果客户端只能生成 `messages`，就走 `/v1/chat/completions`。

## 后续可选优化

Sub2API 可以在 `/v1/responses` 入口本地检测 `messages` 字段，直接返回 `400 invalid_request_error`，避免把明显的客户端格式错误包装成 `502`。本轮只做诊断与真实请求验证，未改代码、未重启容器、未改 nginx/Redis；真实 200 探测正常产生了两条用量记录。
