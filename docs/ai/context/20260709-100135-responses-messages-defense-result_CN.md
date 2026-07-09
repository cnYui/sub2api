# `/v1/responses` 混入 `messages` 服务端防御修复结果

## 结论

已在本地修复正式 OpenAI `/v1/responses` 收到 Chat Completions `messages` body 时错误表现为上游 400 / 对外 502 的问题。

新行为：

- `POST /v1/responses` + 只有 `messages`、没有 `input`：Sub2API 本地返回 `400 invalid_request_error`。
- 错误消息固定为：`/v1/responses expects input; use /v1/chat/completions for messages`。
- `/v1/chat/completions` 的 Chat -> Responses 转换链路不变。
- `/v1/responses` 不自动把 `messages` 转 `input`，避免掩盖客户端协议错误。

## 代码改动

- `backend/internal/handler/openai_gateway_handler.go`
  - 新增 `openAIResponsesMessagesWithoutInputMessage`。
  - 新增 `openAIResponsesHasMessagesWithoutInput()`。
  - 在 `OpenAIGatewayHandler.Responses()` 的 model/stream 基础校验后、本地返回 400。

- `backend/internal/handler/openai_gateway_handler_test.go`
  - 新增 `TestOpenAIResponsesRejectsMessagesWithoutInput`，覆盖 `/v1/responses` + `messages` 的本地 400。

## TDD 记录

第一次 RED 先暴露测试夹具问题：context key 名和 `GroupID` 类型与现有代码不一致，已改为项目内通用写法：

- `string(middleware.ContextKeyAPIKey)`
- `string(middleware.ContextKeyUser)`
- `GroupID: &groupID`

修正夹具后，RED 命令：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run TestOpenAIResponsesRejectsMessagesWithoutInput
```

失败结果符合预期：当前无本地 shape guard，请求返回 `503` 而不是目标 `400`。

GREEN 命令：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run TestOpenAIResponsesRejectsMessagesWithoutInput
```

结果：通过。

## 验证

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler
```

结果：通过，耗时约 `24.346s`。

```bash
git diff --check
```

结果：通过，无输出。

## 未触碰项

- 未改 DB。
- 未改 nginx。
- 未改 Redis。
- 未构建镜像。
- 未替换或重启 18084 容器。
- 未改 `/v1/chat/completions` 转换逻辑。
- 未改 Claude/Anthropic 兼容 `GatewayHandler.Responses()`。
