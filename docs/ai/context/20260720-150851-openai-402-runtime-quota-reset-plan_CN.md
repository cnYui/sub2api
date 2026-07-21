# OpenAI 402 临时放行运行态处置计划

## 背景

2026-07-20 近 4 小时 `sub2api-candidate` 日志中存在多条 `/v1/responses` 402，错误为 `traffic credit is insufficient for request budget`。这些请求先命中 active OpenAI 订阅，但请求前预算没有被当前订阅剩余额度覆盖，随后落入流量卡预授权；受影响用户的流量卡余额为 0 或接近 0，因此直接返回 402。

已确认这是计量单位/预算估算问题导致的运行态影响，新版本已修复但当前需要先让用户可用。

## 影响用户与订阅

| user_id | email | subscription_id | group | 当前日用量 |
| --- | --- | ---: | --- | ---: |
| 19 | xunskyler@gmail.com | 21 | codex-pool-19-usd | 18.6844705000 |
| 35 | luzhiyuan2026@163.com | 53 | codex-pool-19-usd | 16.3569547000 |
| 55 | 853436957@qq.com | 70 | codex-pool-19-usd | 9.7158222000 |
| 60 | daleselaji@gmail.com | 77 | codex-pool-49-usd | 36.1494350000 |
| 88 | 3415991811@qq.com | 96 | codex-pool-19-usd | 0.6537950000 |

## 处置方案

1. 备份上述 `user_subscriptions` 行，以及相关 `user_traffic_credits` 和 Redis `billing:sub:<user_id>:<group_id>` 缓存状态。
2. 只重置上述 active 订阅的 `daily_usage_usd=0`，并把 `daily_window_start` 设置为今日零点；不改周/月用量，因为相关分组没有周/月限额。
3. 删除对应 Redis 订阅缓存 key，避免 L2 旧值继续参与计费判断。
4. 查询确认日用量已归零；继续观察 402 日志是否停止增长。

## 回滚边界

如需回滚，仅恢复这 5 行 `user_subscriptions` 的 `daily_usage_usd`、`daily_window_start`、`updated_at` 至备份值，并再次删除对应 Redis 订阅缓存。历史 `usage_logs`、流量卡账本、订单、API Key、订阅有效期不改动。

## 风险

- 这是临时恢复可用动作，会额外放出这些订阅今日剩余额度，不改变真实历史 usage 明细。
- 如果应用进程 L1 缓存中已有旧订阅对象，直接 SQL 无法主动删除 L1；其 TTL 约 5 分钟。删除 Redis 缓存后，新加载会走 DB。
- 根因仍需通过部署已修复版本解决。
