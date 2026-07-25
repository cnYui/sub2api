# OpenAI 透传分支计费预授权修复结果

## 背景

- `luzhiyuan2026@163.com` 的套餐已撤销，但 API Key 仍 active，且用户仍有 OpenAI 流量卡。
- 继续可用的直接原因不是套餐，而是请求命中 `OpenAI 自动透传` 分支。
- 该分支此前没有走请求前 `authorizeOpenAIForward`，成功响应后才退回旧的流量卡判断；流量卡不足时，请求已经成功返回，只能把 usage fact 记为 `debt`。

## 修复

- 在 `backend/internal/service/openai_gateway_service.go` 的 `forwardOpenAIPassthrough` 中补齐通用计费预授权。
- 请求上游前调用 `authorizeOpenAIForward`，使用 body 中最终透传 model 做预算模型。
- 预授权失败时直接返回 402/503，不触达上游。
- 取 token 或构造上游请求失败时释放尚未 dispatched 的 reservation。
- 真正发送上游请求前调用 `markOpenAIBillingDispatched`。
- transport error 发生在上游请求发出后，标记 reservation 为 unknown。
- 成功构造 `OpenAIForwardResult` 时携带 `BillingAuthorization`，后续 `BuildUsageFact` 使用固定的订阅权益段或流量卡 reservation。

## 测试

- `go test ./internal/service -run "OpenAIGatewayService.*Passthrough|OpenAIBillingAuthorization"`
- `go test -tags unit ./internal/service -run "TestOpenAIBillingAuthorization"`
- `go test ./internal/service`

以上测试均通过。

## 运行态处理

- 已另行禁用 `luzhiyuan2026@163.com`：`users.id=35` 为 `disabled`，`api_keys.id=41 / codex_used` 为 `inactive`。
- 禁用前备份：`backups/20260724-180154-luzhiyuan-disable-account-prechange.sql`。
- 禁用结果见 `docs/ai/context/20260724-180544-luzhiyuan-disable-account-result_CN.md`。

## 后续注意

- 8 条历史过期但仍为 `dispatched` 的 reservation 没有在本次代码修复中直接修改，应单独按账务运维任务处理。
- 本次修复没有针对单个用户写特判。
