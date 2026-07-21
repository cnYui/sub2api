# 套餐每日额度调整计划

## 背景

2026-07-21 用户决定把当前在售订阅套餐的每日可用额度调整为一组手工确定的整数额度：

| 套餐 | 分组 | 新每日额度 |
|---|---|---:|
| 29 元订阅池 | codex-pool-19-usd | 15 USD |
| 39 元订阅池 | codex-pool-29-usd | 25 USD |
| 59 元订阅池 | codex-pool-49-usd | 39 USD |
| 79 元订阅池 | codex-pool-69-usd | 53 USD |
| 99 元订阅池 | codex-pool-89-usd | 66 USD |
| 149 元订阅池 | codex-pool-135-usd | 100 USD |
| 199 元订阅池 | codex-pool-179-usd | 133 USD |

目标是从今天开始更新每天刷新可用额度，不修改套餐价格、有效天数、订阅起止时间、用户余额、订单、流量卡或 API Key。

## 当前只读核对

运行态容器为 `sub2api-candidate`，数据库容器为 `sub2api-candidate-postgres`。

当前在售套餐与分组日限额：

| plan_id | 套餐 | 价格 | group_id | 分组 | 当前每日额度 |
|---:|---|---:|---:|---|---:|
| 1 | 29 元订阅池 | 29 | 2 | codex-pool-19-usd | 19 |
| 2 | 39 元订阅池 | 39 | 3 | codex-pool-29-usd | 29 |
| 3 | 59 元订阅池 | 59 | 4 | codex-pool-49-usd | 49 |
| 7 | 79 元订阅池 | 79 | 9 | codex-pool-69-usd | 69 |
| 6 | 99 元订阅池 | 99 | 8 | codex-pool-89-usd | 89 |
| 8 | 149 元订阅池 | 149 | 11 | codex-pool-135-usd | 135 |
| 9 | 199 元订阅池 | 199 | 12 | codex-pool-179-usd | 179 |

active 订阅数：29 元池 31，39 元池 8，59 元池 12，79 元池 3，99 元池 8，149 元池 0，199 元池 3。

当前 active `subscription_entitlement_periods` 精确权益快照只有 29 元池 1 条，额度为 19。

## 变更方案

1. 先执行 PostgreSQL custom dump 备份到 `deploy/candidate/dumps/`。
2. 用 `pg_restore --list` 验证备份可读。
3. 在单事务中更新：
   - `groups.daily_limit_usd`
   - 当前 active 的 `subscription_entitlement_periods.daily_limit_usd`
4. 不更新：
   - `subscription_plans.price`
   - `subscription_plans.validity_days`
   - `user_subscriptions.starts_at/expires_at/status`
   - 余额、订单、流量卡、API Key 字段
5. 更新后验证目标字段、active 快照、非目标字段。
6. 清理 API Key 认证缓存，让请求热路径尽快读取新分组额度。

## 回滚边界

可用备份完整恢复数据库；若只需回滚本次额度变更，也可按旧映射把 `groups.daily_limit_usd` 和当前 active `subscription_entitlement_periods.daily_limit_usd` 改回：

`19 / 29 / 49 / 69 / 89 / 135 / 179`

本次不重启 Docker，不替换镜像，不改 PostgreSQL/Redis volume。
