# API Key 并发替换实现结果

时间：2026-07-10 11:48 JST

## 结论

本地已实现“每个 API Key 最多 5 并发，用户创建多把 Key 可以扩大并发”。

实现方式为替换 Sub2API 模型请求入口的第一层调用方并发槽：从原来的 `user_id` 维度改为 `api_key_id` 维度。每把 Key 的并发上限仍复用现有 `subject.Concurrency`，默认值为 5；因此同一用户多把 Key 会得到多组独立槽位。

上游账号级并发槽位未改，仍用于保护 CLIProxyAPI/OpenAI 账号池。

## 主要改动

- `service.ConcurrencyCache` 新增 API Key 槽位和等待队列接口。
- `repository.concurrencyCache` 新增 Redis key：
  - `concurrency:api_key:{apiKeyID}`
  - `concurrency:wait:api_key:{apiKeyID}`
- `ConcurrencyService` 新增 `AcquireAPIKeySlot`、`IncrementAPIKeyWaitCount`、`DecrementAPIKeyWaitCount`。
- `ConcurrencyHelper` 新增 `TryAcquireAPIKeySlot`、`AcquireAPIKeySlotWithWait`，等待和 SSE ping 行为与原用户槽一致。
- 以下模型入口已从 user slot 切到 api key slot：
  - Claude Messages：`gateway_handler.go`
  - OpenAI-compatible Responses：`gateway_handler_responses.go`
  - OpenAI-compatible Chat Completions：`gateway_handler_chat_completions.go`
  - Gemini native：`gemini_v1beta_handler.go`
  - OpenAI direct Responses/Messages/WebSocket：`openai_gateway_handler.go`
  - OpenAI direct Chat/Embeddings/Images：`openai_chat_completions.go`、`openai_embeddings.go`、`openai_images.go`
- 新增静态回归测试，防止网关入口重新按 `subject.UserID` 抢 user slot。

## 验证

已通过：

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler
```

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/repository
```

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/...
```

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
```

```bash
git diff --check
```

## 未做

- 未构建镜像。
- 未替换或重启公网 18084 容器。
- 未改数据库 schema、订阅、额度、计费、nginx、Redis 配置或 CLIProxyAPI 配置。

公网生效需要后续构建并发布 Sub2API 应用容器。
