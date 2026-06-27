# 运维日志可见性与当前慢请求排查

## 背景

2026-06-24 19:09 左右，用户截图显示 `/admin/ops` 最近 1 小时健康分约 `65`、TTFT P99 约 `28658ms`、请求时长 P99 约 `117843ms`。用户反馈朋友发请求时需要等很久，询问管理员路由下的日志是否能在数据库或后端看到。

## 日志来源

- 成功请求和慢请求明细在 PostgreSQL `usage_logs`，包含 `request_id`、`user_id`、`api_key_id`、`account_id`、`model/requested_model/upstream_model`、`duration_ms`、`first_token_ms`、token 数、入口和上游路径。
- 失败请求、业务限制和被恢复的上游错误在 PostgreSQL `ops_error_logs`，包含 `error_phase`、`error_type`、`error_owner`、`status_code`、`upstream_status_code`、`is_business_limited`、错误消息等。
- 管理后台请求明细接口 `/api/v1/admin/ops/requests` 后端实现会合并 `usage_logs` 和 `ops_error_logs`，对应代码在 `backend/internal/repository/ops_repo_request_details.go`。
- 系统指标、后台任务心跳等来自 `ops_system_metrics`、`ops_job_heartbeats`；并发/排队部分还会读 Redis/运行态状态。
- 后端容器日志不一定记录所有成功慢请求；定位“等很久但最终成功”的请求，数据库 `usage_logs` 更可靠。

## 最近 1 小时统计

窗口约 `2026-06-24 18:12:54 ~ 19:12:54 JST`：

- 成功请求 `320`，错误总数 `6`，SLA 错误 `0`，业务限制 `6`。
- Token 数约 `3865.2495 万`。
- TTFT：P50 `3492ms`，P90 `11007ms`，P95 `12723ms`，P99 `29222ms`，Avg `5301ms`，Max `33697ms`。
- 请求时长：P99 `118175ms`，Max `161170ms`。
- TTFT 超过 `8s`：`52/320`，约 `16.25%`。
- TTFT 超过 `15s`：`13/320`，约 `4.06%`。
- TTFT 超过 `30s`：`2/320`。

## 按模型

- `gpt-5.5`：147 次，TTFT P50 `5194ms`，P90 `12725ms`，P95 `19938ms`，P99 `31090ms`，Max `33697ms`，超过 8s `43` 次，超过 15s `12` 次。
- `gpt-5.4`：171 次，TTFT P50 `2824ms`，P90 `6223ms`，P95 `8159ms`，P99 `12405ms`，Max `18898ms`，超过 8s `9` 次，超过 15s `1` 次。
- `gpt-5.4-mini`：2 次，无慢尾问题。

## 按用户维度的慢请求线索

- 管理员/本机用户 `x***@gmail.com`：220 次请求，平均 token 约 `128350`，TTFT P90 `8252ms`，Max `32245ms`，超过 8s `24` 次，超过 15s `9` 次。
- 一个 `1***@qq.com` 用户：21 次请求，平均 token 约 `123592`，TTFT P90 `19428ms`，Max `20705ms`，超过 8s `16` 次，超过 15s `3` 次；这类形态符合“用户体感一直等很久”。
- 一个 `8***@qq.com` 用户：11 次请求，平均 token 约 `181577`，TTFT P90 `12181ms`，Max `14265ms`，超过 8s `2` 次。
- 一个 `l***@163.com` 用户：26 次请求，平均 token 约 `72719`，TTFT P90 `9786ms`，Max `12423ms`，超过 8s `5` 次。

## Token 规模与等待时间

- `<20k tokens`：6 次，TTFT P90 `3524ms`，无超过 8s。
- `20-80k tokens`：83 次，TTFT P90 `8416ms`，超过 8s `12` 次，超过 15s `4` 次。
- `80-150k tokens`：103 次，TTFT P90 `5745ms`，超过 8s `7` 次，超过 15s `1` 次。
- `150-220k tokens`：105 次，TTFT P90 `12715ms`，P95 `19188ms`，P99 `32054ms`，超过 8s `28` 次，超过 15s `8` 次。
- `>=220k tokens`：5 次，TTFT P90 `11916ms`，但请求时长 P95 `152619ms`。

## 错误明细

- 最近 1 小时错误主要不是 provider 502。
- `auth/api_error/client 401` 且 `is_business_limited=true`：6 条，来自 Gemini 路径，消息为 `Invalid API key`。
- 被恢复的 upstream 429：1 条，OpenAI `gpt-5.5`，最终客户端状态 200，消息为 `Recovered upstream error 429: The usage limit has been reached`。

## 当前判断

- 这些管理员页面日志可以在数据库看到；成功慢请求主要看 `usage_logs`，错误和恢复错误看 `ops_error_logs`。
- 朋友等待很久的主因更像“成功请求首 token 和总耗时慢”，不是当前 1 小时内的 502 爆发。
- 慢请求集中在 `gpt-5.5` 和大上下文请求，尤其 150k-220k tokens 桶；如果朋友是 `1***@qq.com`，他的近 1 小时体验确实很差。
- 下一步若要精确定位朋友请求，需要提供朋友邮箱或 API Key 掩码；可以按用户/API Key 过滤出全部 request_id、TTFT、duration、token 数和错误记录。
