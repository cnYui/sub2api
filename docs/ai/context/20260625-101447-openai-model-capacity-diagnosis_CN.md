# OpenAI 模型容量报错归因记录

## 问题

用户询问以下报错来源：

`Selected model is at capacity. Please try a different model.`

需要判断这是 Sub2API 拦截导致，还是 OpenAI 官方/上游返回。

## 代码证据

- `backend/internal/service/openai_gateway_service.go` 中 `isOpenAITransientProcessingError` 明确匹配 `selected model is at capacity`，且只在上游状态码为 `400 Bad Request` 时识别为 OpenAI 临时处理错误。
- Responses SSE 流里的 `response.failed` 如果包含该消息，会通过 `openAIStreamFailedEventShouldFailover` 进入 failover，而不是直接作为 Sub2API 自造错误。
- OpenAI Responses、Chat Completions、Messages 兼容路径在上游返回该错误时，会构造 `UpstreamFailoverError`，并在池模式下标记 `RetryableOnSameAccount`。
- Handler 层对 `RetryableOnSameAccount` 会先做同账号重试，超过重试次数后再切换账号；账号切换耗尽后才返回默认映射错误或透传规则匹配后的错误。

## 测试证据

已执行：

```bash
go test ./internal/service -run 'TestIsOpenAITransientProcessingError|TestOpenAIStreamingResponseFailedBeforeOutputCapacityErrorReturnsFailover|TestOpenAIGatewayService_Forward_ModelCapacityErrorTriggersFailoverAndSameAccountRetry'
```

结果：

```text
ok github.com/Wei-Shaw/sub2api/internal/service 0.670s
```

相关测试验证了：

- 该文案会被识别为 OpenAI transient processing error。
- 流式 `response.failed` 中出现该文案时会返回 `UpstreamFailoverError`。
- 非流式上游 `400` 返回该文案时，在 pool mode 下会触发同账号重试/后续 failover。

## 结论

这句报错的原始来源不是 Sub2API 拦截或 Sub2API 自己生成，而是 OpenAI 官方/上游返回的模型容量不足错误。

Sub2API 的作用是识别该上游错误，并把它当作临时容量错误处理：优先同账号重试，再进行账号 failover。若所有可用账号都遇到该错误或重试耗尽，客户端可能看到通用的上游失败错误；如果有错误透传规则或兼容路径暴露了原始消息，也可能直接看到这句英文。

## 排查建议

- 在 ops 错误日志里看对应请求的 `upstream_status`、`upstream_request_id`、`account_id`、`model`。
- 如果同一模型持续出现，应优先切换可用模型或降级模型，而不是改 Sub2API 拦截逻辑。
- 如果只有某个账号池频繁出现，应检查该池上游账号是否集中遇到官方容量限制。
