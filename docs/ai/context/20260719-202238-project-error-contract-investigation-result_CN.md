# 全项目错误契约调查结果

- 时间：2026-07-19 20:22 JST
- 范围：Sub2API 后端、前端及 CLIProxyAPI 聚合边界的只读调查。
- 运行态：未修改数据库、Redis、容器、Nginx 或公网链路；未修改业务代码。

## 结论

当前问题不是单个 429 映射写错，而是全链路缺少统一的“错误语义事实”。上游错误经过账号池调度、failover、协议转换和前端解析后，只剩粗粒度 HTTP 状态或字符串，原始原因可能连续发生两次改写：

1. CLIProxyAPI 中，全部账号处于明确模型冷却时可返回 429；但无法选号且没有完整冷却事实时会聚合为 `auth_unavailable`/503，早先账号级 429 原因不再存在于最终错误对象。
2. Sub2API `OpenAIGatewayHandler.mapUpstreamError` 又把上游 500/502/503/504 统一改成 502。
3. 因此“账号池先发生 429，后续请求只得到无可用认证/503，再被 Sub2API 输出为 502”是当前代码允许且可解释的链路。

最近的 OpenAI failover 状态重构没有引入这个映射：`mapUpstreamError` 的 5xx -> 502 逻辑自 2025-12-27 已存在；近期重构主要收敛了循环状态。

## 主要证据

- `backend/internal/service/gateway_service.go` 的 `UpstreamFailoverError` 只有 `StatusCode`、响应体、响应头和重试标记，没有稳定错误码、错误来源、重试时间、尝试摘要或根因分类。
- `backend/internal/handler/openai_gateway_handler.go`：429 保持 429，但 500/502/503/504 全部输出 502；上游 401/403 也按网关语义输出 502。
- `backend/internal/handler/gateway_handler_responses.go` 与 `gateway_handler_chat_completions.go` 的另一套兼容 handler 会直接保留最后一个上游状态，同一公开协议因内部路由不同而出现不同状态策略。
- `backend/internal/service/openai_gateway_service.go` 同时存在原样透传、规则改写、未配置自定义错误码时返回 500、failover、最终状态映射等多条错误出口。
- 全局错误透传规则可以在运行态改写状态码和消息，导致错误行为不完全由版本化代码决定。
- CLIProxyAPI 当前选择器中，全部模型冷却返回带 `Retry-After` 的 429；普通 `auth_unavailable` 在 handler enrich 后默认返回 503。两者之间没有保留“此前所有失败是否均为 429”的聚合事实。

## 错误结构现状

项目同时存在至少六套对外结构：

1. 通用 API：`{code: number, message, reason, metadata, data}`。
2. 中间件 API：`{code: string, message}`。
3. OpenAI：`{error: {type, message, code?}}`。
4. Anthropic：`{type: "error", error: {type, message}}`。
5. Google：`{error: {code, status, message, details}}`。
6. 历史直接响应：顶层 `error`、`detail` 或 `message`。

`response.go` 和 `middleware.go` 都把自己的结构称为“标准错误响应”，但前者 `code` 是数字，后者 `code` 是字符串。前端 `ApiResponse.code` 又只声明为数字，实际运行时会接收字符串。

静态统计显示：

- 后端约有 188 处直接 `c.JSON(...)`，没有统一经过协议 renderer。
- 现有 `infraerrors`/中间件可识别到约 842 次错误码使用、409 个唯一符号码；`OAUTH_CONFIG_INVALID`、`INVALID_STATUS`、`NOT_FOUND` 等过宽码被大量复用。
- API Key 链路仍有中文公开消息，例如分组删除、分组停用、Key 到期和额度耗尽，不符合统一英文 fallback 的目标。
- 多处请求绑定失败直接把 `err.Error()` 拼到公开消息；安装向导还直接返回数据库/Redis 连接错误，存在不稳定文案和内部细节泄露风险。

## 前端问题

- `frontend/src/api/client.ts` 只优先读取顶层 `message/detail`；OpenAI 风格的 `error.message` 会被保存在 `error` 对象中，但公共提取器把 `error` 当字符串处理。
- `frontend/src/utils/apiError.ts`、`authError.ts` 和页面手写逻辑并存；静态搜索有约 121 处直接读取 `response.data.message/detail/error` 或 `error.message`。
- 现有前端 15 个相关测试通过，但没有覆盖 `{error: {message, code, type}}`，因此会退化为 Axios 的 `Request failed with status code 502`。

## 可复用基础

- 请求入口已经生成并返回 `X-Request-ID`、`X-Client-Request-ID`。
- 运维日志已经区分客户端状态、上游状态、有效状态和多次上游尝试。
- 429 的本地额度/RPM 路径已有 `Retry-After`。
- Responses 流已经支持规范 `response.failed` 终止事件。

这些能力应被统一错误模型复用，不需要另起一套观测系统。

## 修改优先级

### P0：先解决状态语义丢失

- 在 Sub2API 建立单一内部错误模型，至少包含稳定序号、符号码、HTTP 状态、协议类型、英文公开消息、是否可重试、`Retry-After`、request ID、来源、原始上游状态/码和安全 metadata。
- 将 `UpstreamFailoverError` 改为携带结构化原因和所有尝试摘要；最终错误按聚合事实决定，禁止只看最后状态。
- 为 Sub2API 与 CLIProxyAPI 定义内部错误契约。CLIProxyAPI 必须区分 `all_accounts_rate_limited`、`credentials_unavailable`、`model_unavailable`、`upstream_overloaded`、`transport_failure`，并保留 `Retry-After`。
- 明确 HTTP 语义：429 表示可退避的限流/额度；503 表示当前无容量或无可用上游；504 表示上游超时；502 表示连接、协议、凭据或无效上游响应；500 只表示 Sub2API 自身未处理错误。

### P1：统一协议输出和前端消费

- 内部只产生一种错误事实，由 OpenAI、Anthropic、Google、通用 API、SSE、WebSocket renderer 分别输出协议兼容结构。
- OpenAI 错误统一补齐稳定 `error.code`；流式错误使用同一个码，HTTP 已提交时通过协议终止事件表达。
- 通用 API 停止让 `code` 同时承担 HTTP 数字和业务字符串；保留兼容字段并分阶段迁移。
- 前端只保留一个错误 normalizer，支持所有已知结构，并统一按符号码做 i18n、按序号做客服定位。
- 限制错误透传规则：不得覆盖已分类错误的 HTTP 语义，只允许处理未知上游错误的公开消息或显式兼容例外。

### P2：迁移存量业务错误

- 清理过宽、重复和大小写不一致的 409 个符号码。
- 将公开消息统一为稳定英文 fallback；中文展示交给前端 i18n。
- 将动态细节放入安全 metadata，不再拼接到主消息。
- 为每个公开错误补充契约测试，覆盖 HTTP、JSON/SSE/WS、`Retry-After`、request ID 和敏感信息脱敏。

## 规范方向（待用户确认）

推荐采用“双码”：`S2A-四位数字` 作为客服/文档序号，英文大写符号码作为程序分支，例如 `S2A-5301 / UPSTREAM_RATE_LIMITED`。不建议只用 HTTP 状态或只用纯数字。

完整英文消息表和序号分段应在用户确认后形成设计文档，再进入实现计划。

## 验证

- Sub2API：目标 handler 测试通过，确认当前上游 server error 经 failover 后输出 502。
- CLIProxyAPI：目标 selector/handler 测试通过，确认全部冷却为 429、普通 `auth_unavailable` 为 503。
- 前端：`client.spec.ts` 与 `authError.spec.ts` 共 15 个测试通过；同时确认缺少嵌套 OpenAI 错误结构测试。
- 官方参考：OpenAI Error codes 文档明确区分 429、500、503，并建议程序化处理；图片错误指南建议以稳定 `error.code` 分支、记录 request ID、只对 429/5xx 等瞬态错误重试。
