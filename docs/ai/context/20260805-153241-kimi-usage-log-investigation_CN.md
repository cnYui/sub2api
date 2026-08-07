# Kimi 用量日志停止与 API 直连链路调查

## 调查时间

2026-08-05 15:32:41（Asia/Tokyo）

## 结论

1. Kimi 的 `/usage` 页面停在 `2026/08/05 13:17:04` 不是前端分页或查询截断。
2. 数据库中 Kimi 最新成功用量记录为 `2026-08-05 12:17:04.911896+08`，换算东京时间为 `13:17:04`。
3. 该时间之后，账号 `accounts.id=5` 的 Kimi 请求只发现 3 次完整请求，全部为 `POST /v1/chat/completions`、模型 `kimi-k3`、HTTP `502`：
   - `12:34:21.486+08`
   - `12:34:41.105+08`
   - `12:40:45.724+08`
4. 因为请求没有成功完成，正常 `RecordUsage` 不会执行，所以没有新增 Kimi 用量行。日志同时显示 Kimi 账号被上游错误和模型级临时状态标记，随后选择器报告没有可用账号。
5. 未发现可以不认证直接调用 Kimi 上游的公开网关路由。API 直连是系统设计的正常入口，网关路由统一经过 API Key 鉴权；前端不是计费边界。

## 路由与计费链路核对

- `backend/internal/server/routes/gateway.go` 中 `/v1` 网关在所有业务路由前挂载 `apiKeyAuth`；OpenAI 兼容的 `/v1/chat/completions`、`/v1/responses`、别名路由及 Codex 直连路由也均挂载该中间件。
- `backend/internal/server/middleware/api_key_auth.go` 仅对 `/v1/usage`、`/v1/sub2api/billing` 和异步图片任务查询跳过计费检查，这些端点只读数据或查询已有任务，不会转发模型请求。
- `backend/internal/handler/openai_chat_completions.go` 在上游转发成功后异步提交 `RecordUsage`；转发失败直接返回，不记录成功用量。

## 发现的通用风险

当前实现存在“后扣计费失败但请求已成功返回”的缺口：

- 请求先完成上游转发并返回 HTTP `200`。
- 之后由 worker 异步执行 `RecordUsage`，失败只写 `record_usage_failed` 日志，不改变已经发送给客户端的响应。
- `backend/internal/service/gateway_usage_billing.go` 只有 `applyUsageBilling` 成功后才写 `usage_logs`；计费错误直接返回，因此失败请求可能没有余额扣减，也没有用量行。
- 生产日志中已观察到大量 `record_usage_failed`，错误为 `INSUFFICIENT_BALANCE`，同时对应 OpenAI 请求的 HTTP 状态为 `200`。这不是 Kimi 本次停止的原因，但并发/流量卡余额竞争时可能形成“成功响应、未计费、无日志”的真实缺口。

## 建议

- 不要把 Kimi 本次日志停止判定为前端绕过；应先恢复/核验 Kimi 上游 `502` 的原因。
- 将成功响应与计费确认解耦改为可审计的账单状态：至少持久化一条待结算用量事件，后扣失败进入重试/异常队列，禁止仅靠异步错误日志兜底。
- 对 `record_usage_failed` 按模型、账号、用户和请求 ID 建立告警，并补做成功请求与 `usage_logs` 的对账。
