# Active 订阅日用量校准计划

## 背景

截图中部分用户的“每日用量”包含 2026-07-06 残留。只读排查已确认根因不是东八区 0 点计算错误，而是老订阅 `daily_window_start IS NULL` 时刷新 SQL 只补窗口、不清零旧 `daily_usage_usd`。

## 目标

- 当前公网 18084 候选库中，所有 active 且未过期订阅只保留今天的日用量。
- 修复后全量检查是否仍有 `daily_window_start` 不是今天 0 点，或 `daily_usage_usd` 不等于今天 `usage_logs.total_cost` 聚合值的订阅。

## 执行约束

- 执行前备份 `sub2api-candidate-postgres` 到 `deploy/backups/`。
- 不修改 nginx、Docker 应用容器、Redis 容器、Postgres 容器或代码运行版本。
- 只更新 `user_subscriptions` 的 `daily_window_start`、`daily_usage_usd`、`updated_at`。
- 使用当前 DB 会话时区下的 `date_trunc('day', now())`，当前 DB `now()` 已显示为 `+08`。

## SQL 策略

- 以 active 且未删除、未过期订阅为范围。
- 对每个订阅聚合今天 0 点到明天 0 点之间的 `usage_logs.total_cost`。
- 如果 `daily_window_start` 不是今天 0 点，或 `daily_usage_usd` 不等于今日聚合值，则更新为今日窗口和今日聚合值。
- 更新后删除对应 `billing:sub:*` Redis 缓存，让后续请求回源重建。
