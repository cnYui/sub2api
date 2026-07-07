# 订阅 daily 窗口 NULL 导致用量不清零问题说明

## 问题

2026-07-07 管理端订阅列表中，部分用户的“每日用量”在东八区 0 点后仍显示昨天累计值。用户担心东八区 0 点刷新没有生效。

实际排查结论：不是东八区时间计算错误，而是老订阅的窗口字段为 `NULL` 时，当前刷新 SQL 只补窗口、不清旧用量。

## 影响表现

典型表现：

- `daily_window_start` 仍为 `NULL`，但 `daily_usage_usd` 有昨天累计值。
- 或 `daily_window_start` 已变成今天 0 点，但 `daily_usage_usd` 仍等于昨天 + 今天。
- 管理端列表直接读取 `user_subscriptions.daily_usage_usd`，不会主动触发刷新，所以没再发请求的用户会持续显示旧值。

截图排查中的例子：

- `897858381@qq.com`：2026-07-07 无成功请求，但页面显示昨天 `16.4470019500`。
- `qixiaocheng777@gmail.com`：2026-07-07 今日实际 `0.2609524500`，但页面曾显示昨天 + 今天 `19.0097972000`。
- `3056163754@qq.com`：窗口已经是 `2026-07-07 00:00:00+08`，但 daily 用量仍包含昨天 `9.3394234000`。

## 根因

刷新入口在 `BillingCacheService.refreshExpiredSubscriptionWindowsIfNeeded()`，会使用 `timezone.StartOfDay(now)` 计算今天 0 点。这个时间本身是正确的。

问题在 `RefreshExpiredUsageWindows` 的 SQL：

```sql
daily_usage_usd = CASE
  WHEN daily_window_start IS NOT NULL AND daily_window_start < $2 THEN 0
  ELSE daily_usage_usd
END,
daily_window_start = CASE
  WHEN daily_window_start IS NULL OR daily_window_start < $2 THEN $2
  ELSE daily_window_start
END
```

这段逻辑对不同状态的行为：

- `daily_window_start < 今天 0 点`：清零 `daily_usage_usd`，正确。
- `daily_window_start IS NULL`：只把 `daily_window_start` 补成今天 0 点，但保留旧 `daily_usage_usd`，错误。

因此，如果历史订阅的 `daily_window_start` 为空且 `daily_usage_usd` 已有累计值，刷新后会出现“窗口字段看起来刷新了，用量却没清”的状态。

weekly/monthly 当前也有同类模式，需要一并审查：

- `weekly_window_start IS NULL` 时只补窗口，不清 `weekly_usage_usd`。
- `monthly_window_start IS NULL` 时只补窗口，不清 `monthly_usage_usd`。

## 为什么管理端更容易暴露

API 计费入口会在请求前尝试刷新窗口；管理端订阅列表是读模型字段展示，不会调用计费入口刷新逻辑。

所以：

- 继续发请求的用户，可能在请求路径中触发窗口补齐。
- 不再发请求的用户，管理端仍会看到旧 `daily_usage_usd`。
- 如果老数据是 `NULL` 窗口，即使触发刷新，也可能只补窗口不清旧值。

## 已执行的运行态校准

2026-07-07 已对当前公网 18084 候选库执行一次性校准：

- 执行前备份：`deploy/backups/20260707-101105-sub2api-candidate-before-daily-usage-calibration.dump`
- 范围：所有 active、未删除、未过期订阅
- 事实源：今天 0 点到明天 0 点之间的 `usage_logs.total_cost` 聚合值
- 更新字段：
  - `daily_window_start = 2026-07-07 00:00:00+08`
  - `daily_usage_usd = 今日 usage_logs 聚合值`
  - `updated_at = now()`
- 更新数量：38 条 active 订阅
- 修复后全量复查：`remaining_mismatch_count = 0`
- 当时已有 `billing:sub:*` Redis 缓存已删除，让后续请求回源重建

详细结果见：

- `docs/ai/context/20260707-101401-active-subscriptions-daily-usage-calibration-result_CN.md`

## 后续代码修复建议

需要修复代码，避免后续再次出现：

1. `daily_window_start IS NULL` 且窗口需要初始化时，`daily_usage_usd` 不应保留旧累计值。
2. 推荐行为：以 usage window 为事实边界，初始化窗口时将对应 usage 清零，或在迁移/后台任务中按 `usage_logs` 聚合回填。
3. weekly/monthly 窗口要同步确认业务语义，避免只修 daily。
4. 管理端订阅列表不应直接信任陈旧窗口字段；可在列表查询前做轻量窗口刷新，或通过后台任务定时校准。
5. 修复后需要新增回归测试，覆盖：
   - `daily_window_start IS NULL` 且 `daily_usage_usd > 0`
   - `daily_window_start < today_start`
   - weekly/monthly 的 NULL 窗口初始化行为

## 风险提醒

当前运行态数据已校准，但代码尚未修复。只要还有窗口字段为 `NULL` 的老数据、导入数据或特殊创建路径，就可能再次出现“窗口补了但用量没清”的问题。
