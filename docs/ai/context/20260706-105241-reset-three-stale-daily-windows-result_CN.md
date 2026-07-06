# 三个 stale 日额度用户手动清零结果

## 背景

2026-07-06 发现订阅套餐 API 请求入口没有稳定触发过期日窗口刷新，导致部分用户跨过东八区 00:00 后仍沿用旧 `daily_usage_usd`。

本次按用户要求先手动处理三位 stale 超限用户：

- `859591608@qq.com`
- `cnfoxian@gmail.com`
- `milesyang987@gmail.com`

## 操作前状态

三位用户均为 active 19 USD 套餐，且 `daily_window_start` 早于 2026-07-06 00:00+08：

| email | subscription_id | user_id | group_id | daily_limit_usd | daily_usage_usd | daily_window_start | OpenAI 流量卡 |
|---|---:|---:|---:|---:|---:|---|---:|
| `859591608@qq.com` | 72 | 57 | 2 | 19 | 19.1392950000 | 2026-07-05 00:00:00+08 | 0 |
| `cnfoxian@gmail.com` | 64 | 40 | 2 | 19 | 19.0282566500 | 2026-06-27 00:00:00+08 | 19.8934673500 |
| `milesyang987@gmail.com` | 2 | 3 | 2 | 19 | 19.0075800000 | 2026-07-05 00:00:00+08 | 19.2678460000 |

## 备份

执行前已备份公网候选库：

- `deploy/backups/20260706-105157-sub2api-candidate-before-reset-three-stale-daily-windows.dump`

## 执行内容

只更新三条 active 订阅的日窗口字段：

- `daily_usage_usd=0`
- `daily_window_start=2026-07-06 00:00:00+08`
- `updated_at=NOW()`

保留不变：

- `weekly_usage_usd`
- `monthly_usage_usd`
- 套餐状态、到期时间、分组绑定
- GPT/OpenAI 流量卡余额

同时删除 Redis billing subscription cache：

- `DEL billing:sub:57:2 billing:sub:40:2 billing:sub:3:2`
- 返回 `0`，表示当时没有这些缓存 key。

## 操作后状态

| email | subscription_id | user_id | group_id | daily_limit_usd | daily_usage_usd | daily_window_start |
|---|---:|---:|---:|---:|---:|---|
| `859591608@qq.com` | 72 | 57 | 2 | 19 | 0.0000000000 | 2026-07-06 00:00:00+08 |
| `cnfoxian@gmail.com` | 64 | 40 | 2 | 19 | 0.0000000000 | 2026-07-06 00:00:00+08 |
| `milesyang987@gmail.com` | 2 | 3 | 2 | 19 | 0.0000000000 | 2026-07-06 00:00:00+08 |

全局统计：

- 带日额度 active 订阅：43 条
- 过期日窗口且有旧用量：34 条
- 过期日窗口且超过日限额：0 条

## 说明

- 本次没有发起真实 API 请求，避免刚清零后立刻产生新日用量。
- 源码根因仍需按需求文档修复：`BillingCacheService.CheckBillingEligibility()` 的订阅资格检查路径需要在限额判断前刷新过期窗口。
