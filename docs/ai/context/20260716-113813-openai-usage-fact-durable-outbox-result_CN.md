# OpenAI Usage Fact 与 Durable Outbox 阶段一结果

## 结果

本地分支 `codex/fix-traffic-credit-billing-gap` 已完成第一阶段：OpenAI 成功响应的用量先同步写入 PostgreSQL `usage_facts`，再由 durable worker 幂等结算并投影 `usage_logs`。未部署，未修改运行态数据库、Redis、容器或公网服务。

## 关键改动

- 新增 migration `164_usage_facts_durable_outbox.sql`。原计划编号 `163` 已被并行完成的支付宝余额组合支付占用，因此本分支为后续合并预留 `163`，第二阶段 reservation migration 顺延为 `165`。
- `usage_facts` 同时承担不可变用量事实和 durable outbox，状态为 `pending/settling/settled/debt/failed`。
- claim 使用 `FOR UPDATE SKIP LOCKED` 和显式 `lease_until`，租约由 worker 任务超时计算，避免多个 worker 同时结算同一 fact。
- worker 默认配置为 `enabled=true`、`poll_interval_ms=250`、`batch_size=100`、`task_timeout_seconds=10`，失败按最多 256 秒指数退避，无固定最大重试次数。
- `UsageFactSettlementService` 使用现有 `UsageBillingRepository.Apply()` 幂等扣费；`ErrInsufficientBalance` 时仍正式写入 `usage_logs`，随后把 fact 标记为 `debt`。
- settlement 成功后按 fact 中的 ID 重建用户、API Key 和账号上下文，复用现有 post-billing 缓存、last-used、配额失效和通知副作用。
- OpenAI Responses、Chat Completions、Anthropic Messages 兼容入口、Images、Embeddings 使用响应屏障：非流式 body 在 fact 持久化前不可见；SSE 只暂存终止帧及之后的数据，普通 delta 继续实时透传。
- fact 写入失败时，非流式丢弃上游成功 body 并返回 `503 billing_persistence_error`；SSE 丢弃成功终止帧并发送同类错误事件。
- OpenAI WebSocket turn 改为同步持久化 fact，不再提交可能 drop/sample 的内存任务。

## 验证

以下命令均按用户要求串行执行，统一使用 `GOMAXPROCS=2` 和 `-p=1`：

```bash
go test -count=1 -tags=unit ./internal/config ./internal/service ./internal/handler -run 'UsageFact|OpenAIGatewayService|UsageRecord'
go test -count=1 -tags=integration ./internal/repository -run 'TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate|TestUsageFactRepository|TestUsageBillingRepository'
go test -count=1 -tags=unit ./internal/service
go test -count=1 -tags=unit ./internal/handler
go test -count=1 ./cmd/server
```

结果全部通过；Wire 已重新生成，`git diff --check` 无错误。

## 未完成范围

第一阶段只保证成功用量事实不可丢失，尚未阻止流量卡并发超卖。下一阶段必须实现 migration `165`、请求前保守预算、事务预留、唯一计费来源和 debt gate，才能在进入上游前拒绝无法支付的请求。
