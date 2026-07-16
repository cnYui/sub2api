# 当前 524 日志只读诊断

## 范围

- 核对时间：2026-07-15 12:27-12:31 JST。
- 链路：Cloudflare -> cloudflared -> Nginx `:8080` -> Sub2API `:18084` -> CLIProxyAPI `:8317` -> OpenAI Codex 上游。
- 本轮仅查看日志、只读查询 PostgreSQL 和检查进程状态；未修改配置、数据库、Redis、容器或服务。

## 结论

当前日志中存在两层含义不同的 524：

1. 用户侧看到的 524，主要是 `/v1/responses` 在约 120 秒内没有收到首个响应字节，Cloudflare 先断开连接。Nginx 因下游已断开而记录 `499`，cloudflared 记录 `Incoming request ended abruptly: context canceled`。
2. CLIProxyAPI 在 11:49:08 JST 记录的单条 `error status: 524`，来自 OpenAI Codex 上游的 Cloudflare。该请求在上游等待约 30 分钟后才收到 524 HTML 错误页，不是本机 Nginx 或 Sub2API 主动生成。

本机服务没有整体宕机。健康检查当时分别约为：Sub2API 10ms、Nginx 17ms、公网 101ms，均为 HTTP 200。

## 精确时间线

目标请求为流式 `gpt-5.6-luna`：

- 11:19:07：CLIProxyAPI 开始使用第一份 OAuth 凭据请求上游。
- 约 11:21:11：公网连接等待约 124 秒仍无首包，Cloudflare 断开；Nginx 记录 `/v1/responses` 499，cloudflared 记录请求被取消。用户侧此时会看到 524。
- 11:49:08：第一条上游请求等待约 30 分钟后返回 524，CLIProxyAPI 将其识别为可切换错误并更换凭据。
- 11:49:10、11:49:11：后续两份凭据分别返回 401，均为 token 已失效。
- 11:49:30：再次切换凭据后，上游最终成功；CLIProxyAPI 记录 HTTP 200，总耗时 30 分 22 秒。
- Sub2API `usage_logs` 同步记录成功，`duration_ms=1822940`、`first_token_ms=1808809`，即首 token 约 30 分 08.8 秒才出现。

因此“CLIProxyAPI 最终 200”和“用户已经收到 524”可以同时成立：Sub2API 对流式上游使用了脱离客户端取消信号的 context，客户端断开后后台请求仍会继续执行并最终记账。

## 近期影响范围

最近 40 分钟快照：

- 完成并落库的模型请求：466 条。
- 首 token 不少于 120 秒：22 条，涉及 10 个用户。
- 同期 Nginx `/v1/responses` 499：19 条。
- 慢首包分布于 `gpt-5.6-sol`、`gpt-5.6-luna`、`gpt-5.5`、`gpt-5.6-terra`、`gpt-5.4` 和 `codex-auto-review`，不是单一模型或单一客户端问题。

典型对应关系：

- 12:16:39 左右开始的请求在 12:18:43 被公网断开，CLIProxyAPI 到 12:20:44 才成功，总耗时约 4 分 05 秒。
- 12:16:44 左右开始的请求在 12:18:49 被公网断开，CLIProxyAPI 到 12:21:04 才成功，总耗时约 4 分 19 秒。
- 12:18:28 左右开始的请求在 12:20:32 被公网断开，CLIProxyAPI 到 12:22:51 才成功，总耗时约 4 分 23 秒。

## 根因判断

直接原因是上游在 Cloudflare 公网等待窗口内没有产生首包或首个 SSE 事件。当前链路又同时具备以下条件：

- Nginx `proxy_read_timeout=86400`，不会在 Cloudflare 前主动结束慢首包请求。
- Sub2API 的流式上游 context 使用 `context.WithoutCancel`，公网断开后不会及时取消上游请求。
- CLIProxyAPI/上游缺少能在公网窗口前生效的首响应硬期限，实测单次请求可等待约 30 分钟。
- OAuth 池中存在大量失效或停用凭据。当前日志窗口内有较多 401/402，虽然不是本次 30 分钟等待的起点，但会在重试阶段继续增加尾延迟。

当前进程资源不支持“本机已被打满”这一假设：检查时 CLIProxyAPI 约 192 个打开文件、104 条 ESTABLISHED TCP，Sub2API/数据库/Redis 健康，CPU 和内存未出现整体阻塞证据。

## 后续建议

1. 把客户端取消信号传递到上游，只给扣费和审计使用独立短 context，避免用户已经失败后请求仍长期占用连接、账号槽并产生费用。
2. 给上游首响应设置小于公网 Cloudflare 窗口的明确期限，超时后返回可分类的结构化错误并释放资源；若业务必须支持超过 120 秒的推理，应设计服务端 SSE 心跳，而不是无限等待首包。
3. 清理或自动隔离持续返回 401/402 的 OAuth 凭据，减少失败后的账号切换链长度。
4. 给 Nginx access log 增加 `$request_time`、`$upstream_response_time` 和关联请求 ID；当前 `ops_error_logs` 因 `OPS_ENABLED=false` 在该时间窗没有记录，跨层关联主要依赖时间、User-Agent 和 usage 时延反推。
