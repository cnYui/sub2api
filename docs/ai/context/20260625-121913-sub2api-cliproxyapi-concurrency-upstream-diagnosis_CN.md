# Sub2API 与 CLIProxyAPI 并发/上游错误排查结果

## 排查目标

按此前约定先排查两件事：

1. Sub2API 是否真的因为并发太小、排队过长导致问题。
2. 上游/CLIProxyAPI 是否存在容量、限额、认证或 502/429 问题。

本次只读排查，没有修改配置、数据库或代码。

## 运行态

- 公网 Sub2API 容器：`sub2api`，监听 `127.0.0.1:18080 -> 8080`。
- CLIProxyAPI 进程监听 `*:8317`。
- Sub2API 主日志：`deploy/data/logs/sub2api.log`。
- CLIProxyAPI 错误日志：`/Users/wujianxiang/CodeSpace/CLIProxyAPI/logs/error-v1-*.log`。

## Sub2API 并发与排队证据

运行库中只有一个 OpenAI 上游账号：

| account_id | name | concurrency | pool_mode | retry_status |
|---|---|---:|---|---|
| 1 | `cliproxy-local-openai` | 3 | true | `[401, 403, 429]` |

四个分组都绑定同一个上游账号 `cliproxy-local-openai`：

- `codex-pool-19-usd`
- `codex-pool-29-usd`
- `codex-pool-49-usd`
- `codex-pool-local-unlimited`

相关用户的用户级并发均为 5。也就是说当前主要入口瓶颈不是用户并发，而是所有套餐共享的 Sub2API 账号并发 `3`。

09:00 后日志统计：

- 并发/排队异常信号：1 条。
- 唯一一条是 `user_id=13` 的 `gpt-5.4-mini`：`timeout waiting for user concurrency slot`。
- Redis 当前 `concurrency:account:1` 为 zset，当前占槽 2；`wait:account:1` 当前值为 0。

基于请求开始/结束日志估算，09:00 后 `/v1/responses` 峰值同时进入约 9 个请求，超过账号并发 3，说明确实存在排队/等待，但没有形成大面积队列满或并发超时。

## 上游/CLIProxyAPI 证据

09:00 后 Sub2API 文本日志中：

- `openai.forward_failed`：44 条。
- 这些失败消息都是 `upstream response failed: Our servers are currently overloaded. Please try again later.`。
- `pool_mode_error_skipped`：34 条，其中包含 503。
- `upstream_failover_switching`：30 条。
- `pool_mode_same_account_retry`：3 条。
- HTTP access 中 `/v1/responses` 多数仍显示 200，因为这类问题发生在 SSE 里，客户端已收到流式错误帧。

模型分布：

- `gpt-5.5`：22 条 forward failed。
- `gpt-5.4`：21 条 forward failed。

分组分布：

- `group_id=5`：18 条。
- `group_id=2`：12 条。
- `group_id=3`：9 条。
- `group_id=4`：5 条。

CLIProxyAPI 错误文件中还发现：

- `usage_limit_reached`，HTTP 429，消息为 `The usage limit has been reached`。
- 凌晨存在 `auth_unavailable: no auth available`、认证 token invalidated、503。

这些都是 CLIProxyAPI 内部账号池或官方上游侧问题，不是 Sub2API 本地排队能直接生成的错误。

## 性能观察

09:00 后 `usage_logs` 统计：

| model | n | p50 duration | p95 duration | max duration |
|---|---:|---:|---:|---:|
| `gpt-5.5` | 253 | 12.7s | 44.2s | 168.4s |
| `gpt-5.4` | 175 | 9.5s | 29.1s | 124.1s |
| `gpt-5.4-mini` | 9 | 7.1s | 153.4s | 153.4s |
| `codex-auto-review` | 6 | 3.8s | 4.9s | 4.9s |

TTFT 侧：

- `gpt-5.5` 本机无限分组 p95 TTFT 约 9.1s。
- `gpt-5.4` 19 元分组 p95 TTFT 约 11.5s。
- `gpt-5.5` 19 元分组 p95 TTFT 约 18.9s。

## 判断

当前不是“Sub2API 并发太小导致大量请求排队超时，然后显示 capacity/overloaded”。

更准确的结论是：

1. Sub2API 账号并发 `3` 对所有套餐共享，确实偏紧，会造成高峰时排队和 TTFT/总耗时变长。
2. 但当前主要错误来源在上游/CLIProxyAPI：官方 overloaded、CLIProxyAPI 内部账号 usage limit reached、auth unavailable/invalidated。
3. 直接把 Sub2API 并发大幅调高，可能减少本地排队，但会更快打穿到 CLIProxyAPI/官方上游，放大 overloaded/429/502。

## 建议

优先级从高到低：

1. 先清理/分层 CLIProxyAPI 内部账号池：处理 usage limit reached、auth unavailable、invalidated token 的账号。
2. 将 Sub2API 的一个聚合上游拆成按套餐或模型分层的多个聚合上游，避免所有分组抢同一个 account concurrency=3。
3. 若暂时不拆池，可小步把 Sub2API `cliproxy-local-openai.concurrency` 从 3 提到 4 或 5 试运行，并持续观察 overloaded/429/502 是否上升。
4. 不建议一次性调到很高；当前上游已经有明显容量/额度错误。
