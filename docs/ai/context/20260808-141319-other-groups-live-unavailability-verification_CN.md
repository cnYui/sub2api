# 其他分组真实不可用性验证

## 测试方法

- 使用管理员指定的测试 API Key；该 Key 一次只能绑定一个分组。
- 依次临时绑定各活跃分组，等待认证缓存失效后经公网 `https://api.aaccx.pw/v1` 发送一次最小实际请求。
- 每次请求均设置极小输出上限；测试结束后无条件恢复 Key 到新 GPT 0.15 分组 `13`。
- 生图分组单独使用正确的 `POST /v1/images/generations` 接口验证。

## 结果

| 分组 ID | 分组 | 接口 | HTTP 结果 |
| --- | --- | --- | --- |
| 3 | XAI Grok 官方API | chat/completions | 404 |
| 4 | Claude - Max | chat/completions | 404 |
| 5 | Claude - Kiro | chat/completions | 503 |
| 6 | 智谱 GLM | chat/completions | 404 |
| 7 | 月之暗面 Kimi | chat/completions | 404 |
| 8 | DeepSeek | chat/completions | 404 |
| 9 | Codex（日常） | chat/completions | 502 |
| 10 | Codex（生产） | chat/completions | 503 |
| 11 | Codex（特惠） | chat/completions | 502 |
| 12 | OpenAI 生图 | images/generations | 502 |

所有请求均未成功；本轮没有新增本地 `usage_logs`，因此没有产生本地用户扣费。

## 结论与注意事项

- 从调用方视角，除新 GPT 0.15 分组 `13` 外，其他已测分组当前均不可调用。
- 其中 `503` 表示本地没有可用容量；`404` 与 `502` 表示本地仍尝试路由到账号，但上游拒绝或不可达。它们虽然不可用，却不是“本地已彻底关闭”的同一种状态。
- 若目标是让其他分组在本地直接拒绝、完全不再向上游发请求，应单独将对应分组或账号设为 `inactive` / `schedulable=false`；本次仅验证，未改动这些渠道状态。
- 测试结束后，API Key `227` 已恢复为 `group_id=13`；公网健康检查保持 HTTP `200`。
