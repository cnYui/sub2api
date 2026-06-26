# Sub2API 与 CLIProxyAPI 并发权衡评估

## 背景

当前链路为：

`Cloudflare Tunnel -> nginx -> Sub2API -> CLIProxyAPI -> OpenAI/Codex 上游账号池`

Sub2API 是公网入口、用户 Key、计费和用量事实源。CLIProxyAPI 是内网上游账号池、OAuth、协议转换和轮询上游。对 Sub2API 来说，`cliproxy-local-openai` 不是单个静态 OpenAI Key，而是一个聚合上游。

## 核心判断

Sub2API 的账号并发不是越大越好。它本质上是放在 CLIProxyAPI 前面的入口节流阀：

- 过小：用户在 Sub2API 层排队，TTFT 变长，客户端可能超时重试，排队错误增加。
- 过大：请求更快打穿到 CLIProxyAPI/官方上游，可能放大 OpenAI `Selected model is at capacity`、429、流式 502 和慢尾。

所以并发数要按“稳定吞吐”调，不按“峰值贪大”调。

## 需要同时看的指标

Sub2API 层：

- `waiting_in_queue`：是否长期大于 0。
- `openai.account_wait_queue_full`：等待队列是否经常满。
- `openai.account_slot_acquire_failed`：是否经常等槽超时。
- 请求 TTFT / 总耗时 p50、p95、p99。
- 502、503、429 按阶段区分：concurrency、routing、upstream。

上游/CLIProxyAPI 层：

- OpenAI 上游 `400 Selected model is at capacity` 频率。
- 429 频率和是否集中在同一模型/同一账号池。
- 流式 `missing terminal event`、`stream data interval timeout`、502 频率。
- CLIProxyAPI 内部账号池是否有慢账号、坏账号、OAuth 失效账号、地区/代理异常。

## 调参原则

1. 若 Sub2API 队列长期堆积，但上游 429/400/502 不高，可以小步提高 Sub2API 账号并发。
2. 若 Sub2API 队列不高，但上游 `is at capacity`、429、流式 502 高，不能继续加 Sub2API 并发，应先处理 CLIProxyAPI 内部池容量、模型降级或请求平滑。
3. 若排队错误和上游容量错误同时高，说明入口流量超过整体能力，应优先做分层限速和用户侧公平性，而不是简单扩大并发。
4. 若只有某个模型高发，例如大模型流式长任务，应按模型拆并发或降级策略，不要用全局并发硬扛。

## 建议的压测方法

按阶梯调参，而不是一次性调大：

- 选择低峰期。
- 以当前账号并发为基线。
- 每轮只提高 20%～30%，观察 15～30 分钟。
- 保持同样请求模型、stream 比例、请求体大小和用户来源。
- 每轮记录队列深度、TTFT p95、成功率、上游 400/429/502。

停止加并发的信号：

- TTFT p95 不再下降。
- 成功率开始下降。
- `Selected model is at capacity` 或 429 明显上升。
- 流式 502 / 缺终止事件明显上升。
- CLIProxyAPI 单机 CPU、内存、连接数或内部账号失败率上升。

## 推荐方向

短期不建议盲目把 Sub2API 并发调很大。更稳的策略是：

- Sub2API 负责保护用户体验和公平性，让排队可控。
- CLIProxyAPI 负责内部账号池选择、OAuth 和上游容错。
- 根据上游错误率反推 Sub2API 并发上限。
- 对长流式任务和图片任务单独限流，避免占满普通文本请求槽位。

若当前只有一个 `cliproxy-local-openai` 聚合上游绑定到多个套餐分组，那么 Sub2API 层再多分组也不会产生真实上游隔离；真实隔离需要 CLIProxyAPI 内部池按套餐或模型有可观测的容量边界。
