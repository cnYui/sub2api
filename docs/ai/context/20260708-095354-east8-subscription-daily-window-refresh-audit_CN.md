# 东八区订阅日窗口刷新只读核查

时间：2026-07-08 09:53:54（Asia/Shanghai 口径）

## 结论

- 当前代码确实实现了东八区自然日刷新，但不是 00:00 后全量后台任务，而是 API 计费入口的惰性刷新。
- 当前公网 18084 运行态没有让所有 active 订阅真实写库刷新到今天：52 个未过期 active 订阅中，38 个 `daily_window_start` 仍早于 `2026-07-08 00:00:00+08`，其中 25 个还保留非零 `daily_usage_usd`，这些是昨天残留。
- 今天真正发生订阅扣费的 active 订阅共 13 个，全部已刷新到 `2026-07-08 00:00:00+08`，且 `user_subscriptions.daily_usage_usd` 与今天 `usage_logs.total_cost` 聚合一致，没有发现“今天后端扣费但订阅表没加”的订阅错账。
- `/subscriptions` 和管理端订阅列表会在返回前做过期窗口展示归一化，旧窗口残留会显示为 0；因此这两个页面不会把昨天用量加到今天展示。
- `/v1/usage` / Key Usage 路径存在独立展示问题：当前公网 68 个 active API Key 全为自动 Key（`group_id=NULL`），自动 Key 的 endpoint policy 不支持 `/v1/usage`，会在有效分组解析层被拒绝或拿不到订阅额度块。即使固定分组 Key 走到 handler，该路径也不会触发订阅窗口刷新，直接使用上下文订阅数据，有展示旧窗口用量的风险。
- Redis 当前只有 5 个 `billing:sub:*` 订阅缓存，窗口字段均为今天 0 点，不是 stale 的来源。

## 代码路径

- 写库刷新：`backend/internal/service/billing_cache_service.go`
  - `checkSubscriptionEligibility()` 在限额判断前调用 `refreshExpiredSubscriptionWindowsIfNeeded()`。
  - 日窗口起点为 `timezone.StartOfDay(now)`，生产容器 `TZ=Asia/Shanghai`。
- SQL 刷新：`backend/internal/repository/user_subscription_repo.go`
  - `RefreshExpiredUsageWindows()` 对 `daily_window_start IS NULL OR daily_window_start < today_start` 清零 `daily_usage_usd` 并写入今天 0 点。
  - weekly/monthly 也有同类刷新。
- 列表展示归一化：`backend/internal/service/subscription_service.go`
  - `ListUserSubscriptions()`、`ListActiveUserSubscriptions()`、admin `List()` 均调用 `normalizeExpiredWindows()`。
  - `normalizeExpiredWindowForDisplay()` 仅修改返回数据，不写库。
- Key Usage 路径：
  - `backend/internal/server/middleware/api_key_auth.go` 明确让 `/v1/usage` 跳过计费执行。
  - `backend/internal/server/middleware/effective_group.go` 的 `DefaultAutomaticKeyEndpointPolicy()` 只支持 responses/chat/embeddings/images/messages 等实际调用路径，不包含 `/v1/usage`。
  - `backend/internal/handler/gateway_handler.go` 的 `/v1/usage` 在订阅模式下直接读取 context 中的 subscription 值。

## 运行态证据

容器与时间：

- 公网应用：`sub2api-candidate:20260708-092542-6f00a311a-rmb-balance-affiliate`
- Postgres 当前时间：`2026-07-08 08:49:36+08`
- 上海日窗口起点：`2026-07-08 00:00:00+08`

active 订阅日窗口：

```text
active_unexpired=52
daily_null=0
daily_before_today=38
stale_usage_gt0=25
min_daily_window=2026-07-07 00:00:00+08
max_daily_window=2026-07-08 00:00:00+08
```

订阅表与今日日志聚合：

```text
active_count=52
mismatch_count=25
mismatch_abs_sum=431.9114542000
table_daily_sum=526.3737563500
logs_today_sum=94.4623021500
```

今天真实扣订阅的用户：

```text
todays_logs_with_active_subscription=13
today_usage_display_aligned=13
today_usage_but_old_window=0
today_usage_mismatch=0
```

stale 分类：

```text
correct_today_window_count=14
stale_yesterday_only_count=25
stale_window_zero_usage_count=13
refreshed_zero_usage_count=1
```

自动 Key 状态：

```text
api_keys active=68
group_id NULL=68
stale_sub_users=25
stale_users_with_active_key=25
```

用户平台 quota 表另有类似陈旧窗口，但这属于 user platform quota，不是订阅套餐 daily quota：

```text
user_platform_quotas total_quota_rows=197
daily_null=177
daily_before_today=19
stale_usage_gt0=19
```

## 风险判断

1. “所有用户真实刷新”：否。只有发生过 API 计费入口请求、或被人工校准过的订阅才会写库刷新。
2. “后端扣费了前端却不展示”：订阅表未发现今天扣费不展示；但 Key Usage `/v1/usage` 对自动 Key 不支持，是另一个展示入口风险。
3. “前端是否展示今天真实用量”：`/subscriptions` 与管理订阅页会把旧窗口隐藏为 0，不会把昨天加进今天；今天真实扣费的 13 个订阅与日志一致。Key Usage 需要单独修 `/v1/usage` 自动 Key 解析和窗口归一化。

## 后续建议

- 如要让数据库也在 00:00 后全量真实刷新，需要增加定时任务或每日校准任务；当前设计不是全量 cron。
- 修复 `/v1/usage`：把自动 Key 的 usage 查询纳入 effective group 支持，或者在 handler 内显式解析 effective group；同时对返回的 subscription 做与 `SubscriptionService` 一致的窗口归一化。
- 若要立刻消除当前公网表内昨天残留，可按 2026-07-07 的校准方案重新用今天 `usage_logs` 聚合校准 active 订阅，并清理 `billing:sub:*` 缓存；执行前必须备份 Postgres/Redis。
