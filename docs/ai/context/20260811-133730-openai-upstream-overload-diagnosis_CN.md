# OpenAI 中转上游过载排查

## 现象

客户端出现：

`stream disconnected before completion: Our servers are currently overloaded. Please try again later.`

这是 OpenAI Responses 流式请求已经建立后，上游在流中返回失败事件或提前断开时，客户端对失败流的统一提示，不等价于本地网关进程崩溃。

## 线上证据（2026-08-11 13:37 +08:00 前后）

- `sub2api-official-18082`、PostgreSQL、Redis 均 healthy；本地 `18082`、Nginx `8080`、`aaccx.pw`、`api.aaccx.pw` 的 `/health` 均返回 200。
- 应用容器 CPU 约 4.46%，内存约 113 MiB；没有本机资源耗尽证据。
- 最近 6 小时应用日志有 174 次 `openai.forward_failed`；按账号/错误聚合：
  - 账号 `1128`、分组 `9`：131 次原样出现 `Our servers are currently overloaded. Please try again later.`。
  - 账号 `1129`、分组 `10`：19 次 `Upstream temporarily unavailable; please retry`，12 次 `Service temporarily unavailable, please retry later`，另有 10 次同样的过载文案。
  - 账号 `1128` 与 `1129` 的 `base_url` 主机均为 `api.ai-genesis.app`，说明两个分组共享同一上游服务端容量池的可能性很高。
- 最近 30 分钟仍有 22 次转发失败，且 1128/1129 两条线路都在持续失败。
- 分组 `9` 只有账号 `1128`，分组 `10` 只有账号 `1129`；没有同组备用账号可完成切换。两账号本地并发均为 `100`，状态仍为 active/schedulable。
- 代码在收到上游 HTTP 200 后继续解析 SSE；流内 `response.failed` 或读流异常会记录 `openai.forward_failed`。响应头已经写出后不能再改成普通 HTTP 502，只能结束流或写终止事件，因此客户端看到的是 `stream disconnected before completion`。相关逻辑位于 `backend/internal/service/openai_gateway_response_handling.go`、`backend/internal/service/openai_gateway_passthrough.go` 和 `backend/internal/handler/openai_gateway_handler.go`。

## 结论

根因在上游 `api.ai-genesis.app` 的过载/暂时不可用，当前证据不支持把问题归因于 Docker、Nginx、PostgreSQL、Redis 或本地 CPU/内存。此前将上游账号并发从 50 调到 100，可能放大了共享上游容量压力，但仅凭现有日志不能证明单一因果；应在峰值窗口对调低并发后的失败率进行对照验证。

## 建议

1. 不要继续提高这两条线路的本地并发；先将 `1128`、`1129` 临时降到 30~50，观察 10~15 分钟的过载错误、首 token 延迟和成功率。
2. 为分组 `9`、`10` 增加不同上游主机的备用账号，并确认模型映射一致；仅复制同一 `api.ai-genesis.app` 的第二把 Key，可能仍受同一服务端容量限制。
3. 运维监控必须同时看 `openai.forward_failed`/上游错误事件和流式成功率，不能只看健康端点或 HTTP 200，因为流式失败可能发生在 200 响应之后。
4. 本次为只读诊断，未修改数据库、账号状态、并发值或容器。
