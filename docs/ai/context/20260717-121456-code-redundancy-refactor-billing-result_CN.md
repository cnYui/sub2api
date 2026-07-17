# 可靠计费与旧队列清理结果

## 完成内容

- 通用 `GatewayService` 新增 `BuildUsageFact`、`PersistUsageFact`，只构建并持久化计费事实，不再同步扣费或直接写用量日志。
- Gemini 长上下文字段并入 `RecordUsageInput`，删除重复的长上下文入口。
- OpenAI、Anthropic、Gemini 流式响应按协议识别终态；Gemini 以非空 `finishReason` 为终态。非流式响应和流式最后一帧均在 usage fact 持久化成功后放行。
- OpenAI cyber 失败产生真实 Token 时同步持久化计费事实；风控日志、邮件和会话封禁仍异步处理。
- `OpenAIUsageSettlementEffects` 改为通用 `UsageSettlementEffectsHandler`。
- 删除 `UsageRecordWorkerPool`、Handler 注入、Wire 生命周期、专用测试和全部 `gateway.usage_record.*` 配置。
- 删除 `usage_fact_worker.enabled`；usage fact 结算 worker 现在必须启动，轮询间隔、批量大小和超时继续可配置。

## 验证

- `go test -p 1 ./internal/config`
- `go test -p 1 ./internal/handler`
- `go test -p 1 ./internal/service`
- `go test -p 1 ./cmd/server`
- `go test -p 1 -tags=unit ./internal/config`
- `go test -p 1 -tags=unit ./internal/handler`
- `go test -p 1 -tags=unit ./internal/service`
- `go test -p 1 -tags=unit ./cmd/server`
- 搜索确认生产代码和配置中不再存在 `UsageRecordWorkerPool`、`gateway.usage_record.*`、`usage_fact_worker.enabled`。
- `git diff --check` 通过。

## 提交与回滚

- `5fe143f32 refactor: persist gateway usage as facts`
- `d2e2370c0 refactor: gate responses on durable usage facts`
- 旧队列清理单独提交，可按相反顺序 `git revert`。

本阶段未新增数据库迁移，未操作运行态数据库、Redis、容器、Nginx 或公网链路。
