# GPT-0.15 503 与并发诊断

## 诊断时间

2026-08-05 21:17（Asia/Tokyo）

## 结论

- 容量配置已修复：生产实例 `sub2api-official-18082` 的 10 个账号均为 `concurrency=50`。
- GPT-0.15 分组 `groups.id=9` 当前只有账号 `1128`，因此该分组实际是单账号池，理论本地容量为 50。
- 当前 503 的主因不是本地并发槽位耗尽，而是上游 5xx 触发本地的“账号+模型”临时摘除；单账号池没有备用账号时，本地调度器返回 `no available accounts`，最终转换为平台侧 503。
- 上游也确实存在少量真实 503/502，但日志不足以证明这是明确的并发超限；更像上游或出站代理的瞬时不可用，随后被本地保护逻辑放大。

## 生产证据

### 账号与容量

数据库核验结果：

```text
accounts 总数=10，concurrency=50 的记录数=10，非 50=0
group 9 GPT-0.15 account_count=1
group 9 唯一账号=1128
```

Redis 当前快照：

```text
concurrency:account:1128 ZCARD=2
```

即当前约为 `2/50`，没有达到本地账号并发上限。

### 503/502 分类

最近 6 小时 `gpt-5.6-luna` 的 `ops_error_logs`：

```text
platform  503  536  Service temporarily unavailable
provider  503   31  Upstream service temporarily unavailable
provider  502   46  Upstream service temporarily unavailable
```

上游 503/502 记录均关联账号 `1128`。其中一部分包含 `upstream_status_code=503` 和上游 `api_error`，另一部分只有 `error code: 502`，未包含上游 request ID，无法据此认定是上游模型并发限制；后者也可能来自出站代理或中间层。

### 临时摘除链路

生产日志多次出现：

```text
openai_model_transient_state
account_id=1128 model=gpt-5.6-luna failure_streak=3 cooldown_ms=45000 block_scope=account_model
openai.account_select_failed error=no available accounts
```

代码在 `backend/internal/service/openai_account_runtime_block_fastpath.go` 中会将 500/502/503/504/520-524 视为可触发临时冷却的上游瞬态错误；同一分钟内第 2 次冷却 10 秒，第 3 次及以后冷却 45 秒。账号选择器随后在 `backend/internal/service/openai_account_scheduler.go` 过滤 `runtime_blocked` 账号，单账号池因此没有候选，处理器在 `backend/internal/handler/openai_gateway_handler.go` 将无可用账号返回 HTTP 503。

### 无计费基础探测

使用账号 `1128` 的认证，对 `https://api.ai-genesis.app` 的以下接口各并发 40 次：

```text
/v1/models          40/40 HTTP 200
/v1/sub2api/billing 40/40 HTTP 200
/v1/usage           40/40 HTTP 200
```

这只能证明基础接口和认证链路正常，不能证明 `gpt-5.6-luna` 推理端点在高并发下不会返回 5xx。

## 两个 GPT-0.15 API Key 的可行性

可行，且当前架构已经支持，无需新增“按 Key 分流”的代码：

1. 新建第二个 OpenAI API Key 账号。
2. 将两个账号都加入 GPT-0.15 分组 `id=9`。
3. 两个账号都映射相同的 GPT-0.15 模型，并分别设置 `concurrency=50`。
4. 保持两个账号同优先级、均可调度。

生产设置 `openai_advanced_scheduler_enabled=false`，但关闭高级调度并不会关闭基础负载均衡：当前回退选择器仍读取每个账号的 Redis `LoadRate`，按优先级和当前负载排序后抢占并发槽位。开启高级调度后才会额外使用评分、错误率、TTFT 等因子。无论哪种模式，都是“按当前负载尽量均衡”，不是严格每个请求 50/50 的轮询；带会话/`previous_response_id` 的请求还可能因粘性会话继续命中同一账号。

两个 Key 能否真正把上游容量翻倍，取决于中转站的限流维度：

- 若中转站按 API Key 独立限制，两个 Key 通常能分摊并发和瞬时故障影响。
- 若中转站按账号、IP、模型或全局出口限制，两个 Key 仍会共享同一个上游瓶颈，不能保证消除 503。

因此建议先增加第二账号并观察 `ops_error_logs.account_id`、上游状态码和请求延迟分布，再决定是否调整本地临时摘除策略；不建议继续单纯提高单账号 `concurrency`。
