# /responses 与 /chat/completions 当前状态排查计划

## 背景

用户反馈当前 `/responses` 请求不是 200，但 `/chat/completions` 可以 200，需要判断问题出在公网入口路径、后端路由、自动 Key endpoint 策略、请求体兼容，还是 CLIProxyAPI 上游。

## 排查原则

- 先复现并记录精确 HTTP 状态、响应体错误码和命中入口。
- 对比 `/responses`、`/v1/responses`、`/chat/completions`、`/v1/chat/completions` 四类路径，区分裸路径兼容问题和正式 `/v1/*` 问题。
- 优先查 Sub2API 当前运行容器、nginx 生效配置、后端路由和系统日志，再判断是否需要改代码。
- 不在文档、日志或回复中暴露完整 API Key、内部 token 或 HMAC secret。

## 初始假设

- 若裸 `/responses` 不是 200，而 `/v1/responses` 正常，则符合 2026-07-08 已确定的“只保留 `/v1/*` 为正式模型 API”策略，不是功能故障。
- 若 `/v1/responses` 也不是 200，而 `/v1/chat/completions` 正常，则需要重点排查自动 Key 白名单、请求体 schema、responses 转发链路或 CLIProxyAPI provider 支持。

## 待验证

1. 当前公网 nginx 与 18084 对四类路径的无鉴权状态码。
2. 使用可用 active Key 对 `/v1/responses` 与 `/v1/chat/completions` 的真实最小请求结果。
3. 对应 `ops_system_logs` / 容器日志中是否有 `INVALID_BASE_URL`、`AUTO_KEY_UNSUPPORTED_ENDPOINT`、上游 4xx/5xx 或 stream usage 终止异常。
4. 若确认是代码缺陷，再按 TDD 新增失败用例后修复。
