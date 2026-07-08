# API /v1 与裸 /responses 路由核查

## 结论

- 标准 OpenAI 兼容写法应让客户端 Base URL 使用 `https://api.aaccx.pw/v1`，Responses 完整地址为 `https://api.aaccx.pw/v1/responses`。
- 当前并不是所有请求都在 `/v1` 下。今天 nginx access log 中有裸 `/responses` 和裸 `/models?client_version=...`，说明仍有客户端把 Base URL 配成不带 `/v1`。
- 当前公网不是把 `/responses` 做 301/302 跳转到 `/v1/responses`；实测 `https://api.aaccx.pw/responses`、`https://aaccx.pw/responses` 都无 `Location`，直接进入 Sub2API 返回缺 Key 的 `401`。本机 `127.0.0.1:8080` 和 `127.0.0.1:18084` 也一致。
- 从兼容性看，裸 `/responses` 应作为兼容别名保留，但不应作为推荐入口。若真做 HTTP redirect，POST/SSE 客户端可能不跟随、丢方法或断流；必须用 307/308 才能保留方法和请求体，但仍不如服务端内部 alias/proxy 稳。

## 证据

- 后端 `backend/internal/server/routes/gateway.go` 同时注册：
  - 标准 `/v1/responses`、`/v1/chat/completions`、`/v1/images/*`。
  - 裸 `/responses`、`/responses/*subpath`、`/chat/completions`、`/embeddings`、`/images/generations`、`/images/edits`。
  - `/backend-api/codex/responses` 兼容入口。
- 当前 nginx：
  - `api.aaccx.pw` 的 `location /` 全部代理到 `127.0.0.1:18084`。
  - `aaccx.pw` 明确把裸 `responses`、`chat/completions`、`embeddings`、`images/(generations|edits)` 代理到 `127.0.0.1:18084`，避免落到 yui.web。
- 今天 `2026-07-08` nginx access log 计数：
  - `/v1/responses`: 2674
  - `/responses`: 455
  - `/v1/chat/completions`: 474
  - `/models` 裸路径：194，其中 `/models?client_version=...` 为 171
  - `/v1/models`: 28
  - `/v1/images/edits`: 8
- 近 2 万行计数：
  - `/v1/responses`: 2448
  - `/responses`: 455
  - `/v1/chat/completions`: 474
  - `/models`: 194
- 裸 `/responses` 主要来源包含 `Codex Desktop/0.142.5`、`codex_vscode/0.142.5`、`codex-tui/0.143.0` 等。

## 判断

- 裸 `/responses` 的根因通常是客户端 Base URL 填成 `https://api.aaccx.pw` 或 `https://aaccx.pw`，客户端再拼 `/responses`。
- 官方 OpenAI API 的版本化入口是 `/v1`；Responses API 标准端点为 `/v1/responses`，Chat Completions 为 `/v1/chat/completions`。参考：`https://platform.openai.com/docs/api-reference/responses/create`、`https://platform.openai.com/docs/api-reference/chat/create`。
- 对外文档、用户配置、Codex 示例继续统一写 `base_url = "https://api.aaccx.pw/v1"`。裸路径只作为容错兼容，不在 UI/文档里推荐。

## 建议

1. 保留裸 `/responses`、裸 `/models` 兼容一段时间，降低已配置用户的中断风险。
2. 不建议用 301/302 把模型 POST 请求跳转到 `/v1/responses`。
3. 如果未来想清理裸路径，先在 access log/ops 日志按 User-Agent 和账号观察 7-14 天，再用响应头或控制台提示引导用户修正 Base URL。
4. 更直接的可选优化：给裸 `/models?client_version=...` 增加兼容 alias 到 `/v1/models`，因为当前裸 `/responses` 能跑，但同一类错误配置下模型列表会 404。
