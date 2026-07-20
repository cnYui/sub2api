# 全项目错误契约实现计划

## 目标

建立 Sub2API 面向客户端的统一错误契约，并与 CLIProxyAPI 约定稳定的内部失败类别，修复账号池限流在多次聚合后被错误输出为 502 的问题。

## 已确认决策

- 采用双码：`S2A-xxxx` 作为稳定序号，英文大写符号码作为程序分支代码。
- 保留 OpenAI、Anthropic、Google 和通用 API 的既有响应外壳，只增加兼容字段与响应头。
- 前端默认显示本地化短提示；错误详情可复制 `S2A-xxxx`、英文代码和 request ID。
- 错误透传规则只用于识别并分类上游错误，不得改写公开 HTTP 状态、英文消息或重试语义。
- CLIProxyAPI 只输出内部失败类别；Sub2API 负责转换为最终公开 `S2A` 错误。

## 实施顺序

1. 新建错误目录、结构化错误事实和 HTTP/协议 renderer，覆盖 OpenAI 网关的 failover 终点。
2. 扩展 failover 错误，保留上游分类、`Retry-After` 和尝试摘要；删除 5xx 统一映射为 502 的行为。
3. 升级 CLIProxyAPI：全部可用账号均冷却时明确输出 429 和 `all_accounts_rate_limited`；其他无凭据状态输出 503 类别。
4. 将通用 API、中间件、Anthropic、Google、SSE 和 WebSocket 收敛到相同错误事实；迁移透传规则为受枚举约束的分类规则。
5. 前端统一归一化错误体和响应头，使用 i18n 提示并提供可复制错误引用。
6. 按 CLIProxyAPI、Sub2API、前端顺序发布；Sub2API 保持对未升级 CLIProxyAPI 响应的状态码回退兼容。

## 验收

- 全账号模型冷却返回 `S2A-5004 / UPSTREAM_RATE_LIMITED / HTTP 429`，并保留 `Retry-After`。
- 无可用上游账号返回 503，不再被转换为 502；上游超时为 504；连接和无效响应为 502。
- 所有公开错误不回显上游原文、账号、内部地址或 Go 错误详情。
- OpenAI、Anthropic、Google、通用 JSON、SSE、WebSocket 和前端归一化测试均覆盖规范字段和兼容字段。

## 风险与边界

- 当前工作区仅包含 Sub2API；CLIProxyAPI 修改在其独立仓库以同一契约实施和验证。
- 现存的公开 `reason` 作为兼容字段保留，后续页面按批次迁移到 `error_code`。
- 运行态数据库、Redis、容器、Nginx 和错误透传规则数据不在本次本地代码变更中修改。
