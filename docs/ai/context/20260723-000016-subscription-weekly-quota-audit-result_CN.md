# 订阅档位周额度只读排查结果

## 查询时间

- 2026-07-23 00:00:16 JST

## 查询口径

- 只读查询本地当前项目运行库：`sub2api-postgres-dev / sub2api`。
- 公开 Codex 订阅档位以 `subscription_plans` 关联的 `groups.subscription_type='subscription'` 为准。
- 每周可用额度以 `groups.weekly_limit_usd` 为配置源；新权益段会写入 `subscription_entitlement_periods.weekly_limit_usd`。
- 周窗口按订阅锚点滚动，每 7 天刷新；28 天有效期等于 4 个周窗口。

## 档位表

| 档位 | 分组 | 售价 CNY | 每用户每周额度 USD | 28 天总额度 USD | 当前有效用户数 |
|---|---|---:|---:|---:|---:|
| 29 元订阅池 | `codex-pool-19-usd` | 29 | 58 | 232 | 28 |
| 39 元订阅池 | `codex-pool-29-usd` | 39 | 78 | 312 | 9 |
| 59 元订阅池 | `codex-pool-49-usd` | 59 | 118 | 472 | 12 |
| 79 元订阅池 | `codex-pool-69-usd` | 79 | 158 | 632 | 3 |
| 99 元订阅池 | `codex-pool-89-usd` | 99 | 198 | 792 | 8 |
| 149 元订阅池 | `codex-pool-135-usd` | 149 | 299 | 1196 | 0 |
| 199 元订阅池 | `codex-pool-179-usd` | 199 | 400 | 1600 | 5 |

## 额外发现

- 另有本地分组 `codex-pool-local-unlimited`，没有公开套餐价，没有周额度限制，当前有效用户数为 1。
- 代码中的周窗口实现会按 `weekly_anchor_at` 或订阅开始时间推进；最后一个不足 7 天的窗口会按实际天数折算额度。
