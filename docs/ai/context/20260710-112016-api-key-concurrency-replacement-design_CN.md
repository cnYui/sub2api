# API Key 并发替换设计

时间：2026-07-10 11:20 JST

## 背景

当前 Sub2API 的模型请求入口先按 `users.concurrency` 抢用户级 Redis 槽位。默认值为 5，因此同一用户创建多把 API Key 仍共享 5 并发。这和目标“每把 API Key 最多 5 并发，用户创建多把 Key 可以扩大并发”相反。

此前尝试在 CLIProxyAPI `inbound-limits` 做 `per-api-key=5` 会按 Sub2API 共用的上游 Key 聚合计数，把整条 Sub2API -> CLIProxyAPI 链路限制成 5 并发，已经确认不可用。本次限制必须落在 Sub2API 认证后的用户 API Key 层。

## 目标

- 每把 Sub2API API Key 独立最多 5 个正在处理的模型请求。
- 同一用户创建多把 active API Key 后，并发上限按 Key 数自然扩展。
- 上游账号级并发限制保留，继续保护 CLIProxyAPI/OpenAI 账号池。
- 不新增数据库字段，不改订阅、额度、计费、账号选择和 CLIProxyAPI 配置。

## 方案比较

推荐方案：把请求入口第一层并发槽从 `user_id` 替换为 `api_key_id`。复用现有 `subject.Concurrency` 作为每把 Key 的上限，默认仍是 5。优点是改动小、无需迁移，能直接满足“多 Key 扩并发”。缺点是后台“用户并发”字段的命名暂时不够精确，实际含义变成“单 Key 并发上限”。

备选方案一：保留用户级并发，再新增 Key 级并发。这样单 Key 不能超过 5，但同一用户多 Key 仍会被用户级 5 卡住，不满足目标。

备选方案二：给 `api_keys` 新增 `concurrency` 字段。长期表达更准确，但需要 DB migration、管理端字段、认证缓存版本和前端表单改造；本次只需要固定每 Key 5，不值得扩大范围。

## 设计

新增 Redis 槽位维度：

- 活跃槽：`concurrency:api_key:{apiKeyID}`，Sorted Set，成员仍为进程级 requestID。
- 等待计数：`concurrency:wait:api_key:{apiKeyID}`，整数计数器。

在 `service.ConcurrencyCache` 增加 API Key 槽位与等待队列接口；`repository.concurrencyCache` 复用现有 Lua 脚本实现 acquire/release/count/wait。启动清理需要扫描并清理 `concurrency:api_key:*`，并删除 `concurrency:wait:api_key:*`。

在 `service.ConcurrencyService` 增加 `AcquireAPIKeySlot`、`IncrementAPIKeyWaitCount`、`DecrementAPIKeyWaitCount` 等方法。`maxConcurrency <= 0` 仍表示不限流，保持原行为。

在 `handler.ConcurrencyHelper` 增加 `TryAcquireAPIKeySlot` 与 `AcquireAPIKeySlotWithWait`，等待、SSE ping、超时和释放策略与原用户槽一致。

所有模型网关入口把第一层并发替换为 API Key 槽：

- `GatewayHandler.Messages`
- `GatewayHandler.Responses`
- `GatewayHandler.ChatCompletions`
- `GatewayHandler.GeminiV1BetaModels`
- `OpenAIGatewayHandler` HTTP Responses/Chat 辅助入口
- `OpenAIGatewayHandler` WebSocket 每 turn 抢槽逻辑

账号级槽位、账号等待队列、图片全局并发限制和用量扣费不变。

## 错误行为

API Key 槽位满时沿用现有并发错误路径，返回 429 `rate_limit_error`。错误文案中的 slot type 应使用 `api_key`，避免再写成 user。等待队列满时仍返回 `Too many pending requests, please retry later`。

## 测试策略

- Service unit：API Key 槽位成功、失败、不限流、释放调用正确。
- Handler helper unit：`TryAcquireAPIKeySlot` 调用 API Key 维度，不误用用户槽；`AcquireAPIKeySlotWithWait` 满槽后进入 API Key 等待队列。
- Handler 回归：典型 OpenAI/Claude/Gemini 入口应调用 API Key 槽，不再调用 user 槽。
- 全量目标：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler`，以及 `git diff --check`。

## 上线注意

这是代码行为变更，需要构建并替换 18084 应用容器后才会对公网生效。发布后可用同一用户两把 Key 做并发实测：单 Key 第 6 个请求应等待或 429，多 Key 各 5 个请求不应被同一个用户级槽卡住。
