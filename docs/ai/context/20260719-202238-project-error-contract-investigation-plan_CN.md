# 全项目错误契约调查计划

- 时间：2026-07-19 20:22 JST
- 目标：只读梳理 Sub2API 全部错误出口，定位 429 被展示为 502 的状态丢失链路，并识别统一错误规范需要修改的模块。

## 调查范围

- Sub2API 后端：通用 API 响应、API Key/JWT 中间件、OpenAI/Anthropic/Gemini 兼容入口、failover、错误透传规则、流式错误、运维错误日志。
- Sub2API 前端：Axios 响应拦截器、错误提取 helper、各页面手写错误解析和提示。
- CLIProxyAPI 边界：只读核对账号池冷却、无可用认证和最终 HTTP 错误输出，确认原始 429 是否在进入 Sub2API 前已经被聚合。
- 官方兼容性参考：OpenAI Error codes 文档及图片错误处理建议。

## 关键问题

1. 原始上游状态、failover 聚合原因和最终客户端状态分别在哪一层产生或改写。
2. 同一类错误在不同协议、流式/非流式和不同 handler 中是否保持相同语义。
3. 当前错误结构、英文消息、机器码、重试提示和 request ID 是否稳定。
4. 前端是否能可靠提取所有后端错误结构，而不是退化为 `Request failed with status code 502`。
5. 最小根治范围应限于 Sub2API，还是必须包含 Sub2API 与 CLIProxyAPI 的内部错误契约。

## 约束

- 不修改业务代码、数据库、Redis、容器、Nginx 或公网运行态。
- 不输出 API Key、内部 token 或完整上游敏感错误体。
- 先确认根因和契约边界，再进入设计与实现。
