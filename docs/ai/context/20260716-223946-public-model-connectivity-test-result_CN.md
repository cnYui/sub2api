# 公网 GPT-5 模型真实请求验证结果

时间：2026-07-16 22:39（Asia/Tokyo）

## 范围

- 使用用户提供的 LOCAL API Key 访问 `https://api.aaccx.pw/v1`。
- 先请求 `GET /v1/models`，确认目标模型：`gpt-5.5`、`gpt-5.6-luna`、`gpt-5.6-sol`、`gpt-5.6-terra`。
- 对每个模型调用一次最小非流式 `POST /v1/chat/completions`，请求内容为要求仅返回 `OK`，输出上限为 16 token。
- Key 未写入本文件、代码、配置或日志。

## 结果

| 模型 | HTTP | 实际响应 | 耗时 | 结论 |
| --- | --- | --- | --- | --- |
| `gpt-5.5` | 200 | `OK` | 5 秒 | 可用 |
| `gpt-5.6-luna` | 200 | `OK` | 1 秒 | 可用 |
| `gpt-5.6-sol` | 429 | `Upstream rate limit exceeded, please retry later` | 2 秒 | 当前上游限流，不可用 |
| `gpt-5.6-terra` | 200 | `OK` | 1 秒 | 可用 |

对 `gpt-5.6-sol` 间隔 8 秒后以相同最小请求独立重试一次，仍返回 HTTP 429 和相同上游限流信息（1 秒）。因此该结论不是单次偶发请求失败；本次未修改服务、数据库、Redis、配置或用户数据。
