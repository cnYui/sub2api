# `/v1/responses` 混入 `messages` 的服务端防御修复设计问答

## Q1：这次要解决的真实问题是什么？

客户端把 Chat Completions 请求体发到了 Responses endpoint：

```json
{
  "model": "gpt-5.5",
  "messages": [
    { "role": "user", "content": "你好" }
  ]
}
```

但请求路径是：

```http
POST /v1/responses
```

Responses endpoint 应接收 `input`，不是 `messages`。当前 Sub2API 会把这类 body 原样转发给上游，最终上游返回 `400 Unsupported parameter: messages`，Sub2API 对外表现为 502，错误语义不清晰。

## Q2：为什么不是修改 Codex Desktop？

Codex Desktop 正确配置下本来就应该走：

```toml
base_url = "https://api.aaccx.pw/v1"
wire_api = "responses"
```

并发送 Responses 格式的 `input`。前面成功日志也证明大量 Codex Desktop 请求是 `/v1/responses -> /v1/responses` 且成功落库。

所以这次服务端修复不是为了兼容 Codex Desktop 的错误行为，而是为了在其它客户端、调试脚本或兼容 SDK 混用 path/body 时返回清晰 400，避免误判为上游不可用。

## Q3：为什么不在 `/v1/responses` 自动把 `messages` 转成 `input`？

不建议这么做。

原因：

- `/v1/responses` 和 `/v1/chat/completions` 是两种协议面，自动转换会掩盖客户端配置错误。
- Chat Completions 的 `messages` 与 Responses 的 `input` 不是所有字段都一一等价，工具调用、reasoning、previous_response_id、stream 语义都可能被隐式转换放大风险。
- 当前代码已有明确转换入口：`/v1/chat/completions` 通过 `ForwardAsChatCompletions()` 调 `apicompat.ChatCompletionsToResponses()`，不应把这套兼容逻辑复制到 Responses handler。

结论：不做兼容转换，只做防御性拒绝。

## Q4：推荐的服务端行为是什么？

当正式 OpenAI `/v1/responses` 请求体里：

- 存在 `messages`
- 不存在 `input`

直接返回 HTTP 400，错误类型使用现有 OpenAI gateway 风格：

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "/v1/responses expects input; use /v1/chat/completions for messages"
  }
}
```

不进入账号选择、不转发上游、不扣费、不写成功 usage。

## Q5：如果同时带了 `messages` 和 `input` 怎么处理？

本轮最小防御只拦截“有 `messages` 且没有 `input`”的典型混用。

同时带 `messages` 和 `input` 属于更模糊的非法 body，但这类请求不在当前已复现问题的最小闭环内。为了避免误伤可能带无关 metadata 的客户端，本轮不扩大拦截面；后续如果日志里出现这种形态，再单独收敛。

## Q6：改哪里？

只改正式 OpenAI 网关入口：

- `backend/internal/handler/openai_gateway_handler.go`

位置建议放在：

1. 已读取并解析 body
2. 已完成基础 JSON / model / stream 校验
3. 内容审核、账号选择、计费准入、上游转发之前

这样可以最早返回清晰协议错误，同时不影响鉴权、body 大小、空 body、JSON parse、model required 等已有错误优先级。

不改：

- `backend/internal/service/openai_gateway_chat_completions.go`
- `backend/internal/pkg/apicompat/chatcompletions_to_responses.go`
- `GatewayHandler.Responses()` 的 Claude/Anthropic 兼容链路
- Nginx 配置
- DB、Redis、容器运行态

## Q7：怎么测试？

先写失败测试，再实现。

目标测试建议放在：

- `backend/internal/handler/openai_gateway_handler_test.go`

新增用例：

1. `/v1/responses` body 只有 `messages`、没有 `input` 时返回 400。
2. 响应 message 明确提示 Responses 要用 `input`，Chat 格式要走 `/v1/chat/completions`。
3. 通过 stub/spy 确认不会调用 `gatewayService.Forward()`；如果现有测试结构不方便 spy，就用最小 handler 依赖让请求在转发前返回，避免引入大范围重构。

回归验证：

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler
git diff --check
```

如果目标包因既有非相关测试过重或需要更多依赖，再收窄到具体测试名运行，并记录原因。

## Q8：成功标准是什么？

- `/v1/responses` + `input` 仍按原逻辑转发。
- `/v1/chat/completions` + `messages` 仍按原逻辑转换并转发。
- `/v1/responses` + `messages` 不再变成上游 400 后的 502，而是在 Sub2API 本地返回 400。
- 错误提示能让用户明确改客户端配置，而不是误以为套餐、账号池或上游故障。

## Q9：后续实施顺序是什么？

1. 按 TDD 先写 handler 级失败测试。
2. 在 `OpenAIGatewayHandler.Responses()` 增加最小 shape guard。
3. 跑目标测试和 `git diff --check`。
4. 新建 result 文档，记录没有改 DB、nginx、容器。
