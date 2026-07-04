# 管理员 LOCAL API Key 延迟排查结果

## 结论

- 当前慢响应主要不在 Sub2API，也不是 Sub2API 账号并发满导致。
- 实际链路为 `api.aaccx.pw/aaccx.pw -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`。
- 管理员 `xiaobianfuai@gmail.com` 在 18084 公网库中只有 1 个 active LOCAL Key：`api_keys.id=32`，分组 `codex-pool-local-unlimited`，上游账号为 `accounts.id=1 cliproxy-local-openai`。
- Sub2API 账号并发配置为 10；2026-07-04 10:00-10:45 JST 期间该账号和 LOCAL Key 的重算最大并发均为 3，所有超过 60 秒的慢请求开始时并发均为 1。
- CLIProxyAPI usage 日志能与 Sub2API 慢请求时间精确对齐，说明长耗时发生在 `CLIProxyAPI -> OpenAI/Codex 内部账号` 这一段。

## 关键证据

- 18084 公网库最近 24 小时 LOCAL Key 请求数 984，`p50=10608ms`、`p90=24583ms`、最大 `566436ms`。
- 2026-07-04 10:00-10:45 JST 内，LOCAL Key 有 6 条超过 60 秒的请求：
  - `usage_logs.id=42831`：10:17:22-10:26:49，`duration_ms=566436`，`first_token_ms=28295`。
  - `usage_logs.id=42837`：10:27:04-10:33:47，`duration_ms=402667`，`first_token_ms=21087`。
  - `usage_logs.id=42810`：09:58:23-10:02:44，`duration_ms=260955`，`first_token_ms=26106`。
  - `usage_logs.id=42812`：10:03:26-10:06:06，`duration_ms=160102`，`first_token_ms=19167`。
  - `usage_logs.id=42844`：10:38:36-10:40:08，`duration_ms=91773`，`first_token_ms=10573`。
  - `usage_logs.id=42829`：10:15:40-10:16:56，`duration_ms=75604`，`first_token_ms=3043`。
- 这些慢请求在 CLIProxyAPI `usage-events-2026-07.jsonl` 中对应为真实上游请求耗时，且其中两条最长请求的 CLIProxyAPI `latency_ms` 分别约 566 秒、402 秒。
- CLIProxyAPI 错误详情文件显示部分内部账号返回 `usage_limit_reached` 429；同一逻辑请求会在 2-3 个不同内部账号之间快速重试。
- CLIProxyAPI 当前配置为 `routing.strategy=round-robin`、`request-retry=1`、`max-retry-credentials=3`。它会在上游错误前首包阶段重试，但对“成功但很慢”的流式响应不会自动切走。
- 2026-07-04 10:00-10:45 JST CLIProxyAPI `/v1/responses` 成功请求 50 条，`p50=16960ms`、`p90=75505ms`、最大 `566339ms`，平均输入约 97343 tokens。
- 同窗口内输入超过 100k tokens 的成功请求 21 条，`p50=19221ms`、`p90=91673ms`、最大 `566339ms`，平均输入约 132605 tokens。大上下文是基础耗时来源，个别内部账号/上游状态会把耗时放大到数分钟。
- 容器和进程资源正常：Sub2API、Postgres、Redis、nginx、CLIProxyAPI CPU/内存均未满；`api.aaccx.pw/health` 返回 200，约 0.14 秒。

## 判断

- Sub2API：负责鉴权、计费、并发槽、转发；当前没有并发满、额度拦截、健康异常或资源瓶颈证据。
- CLIProxyAPI：存在内部账号池健康不均；部分账号 429，会触发快速重试；部分账号成功但流式输出极慢，不会触发重试。
- Codex 请求：当前上下文经常达到 100k+ tokens，长上下文会显著拉高首 token 和总耗时。

## 建议

1. 先清理或停用最近频繁 429 的内部 Codex auth，并重点观察两类异常账号：`usage_limit_reached` 账号、成功但 `latency_ms` 长期超过 60 秒的账号。
2. 如需临时改善体验，可先从 CLIProxyAPI auth 池中停用 10:00-10:45 JST 出现 400 秒、566 秒成功慢请求的账号，再观察 p90。
3. 可以把 CLIProxyAPI 的 `max-retry-credentials` 从 3 提高到 5-8，降低连续碰到 429 账号的概率；这只能改善错误重试，不会解决成功但超慢的账号。
4. 长期方案应给 CLIProxyAPI 增加基于近期 latency 和 429 的健康评分/冷却策略，避免 round-robin 持续打到慢账号。
