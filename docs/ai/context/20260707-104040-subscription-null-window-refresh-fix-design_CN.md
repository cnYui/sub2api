# 订阅 NULL 窗口用量残留彻底修复设计

## 背景

本地 `main` 当前 HEAD 为 `ab218d068`。`docs/ai/context/20260707-101847-subscription-daily-window-null-root-cause_CN.md` 已确认：订阅窗口字段为 `NULL` 且 usage 字段已有历史累计时，当前代码会出现“窗口被补上，但旧用量没有清掉”的状态。

这不是东八区自然日计算错误。`timezone.StartOfDay(now)` 和 `usagewindow.QuotaDailyExpired(nil, currentStart)` 的语义是正确的；问题集中在两个边界：

- 写路径：`backend/internal/repository/user_subscription_repo.go` 的 `RefreshExpiredUsageWindows` SQL 对 `*_window_start IS NULL` 只补窗口，不清 `*_usage_usd`。
- 读路径：`backend/internal/service/subscription_service.go` 的 `normalizeExpiredWindows` 只处理过期的非空窗口；`UserSubscription.NeedsDailyReset()` 对 `NULL` 窗口返回 false，所以管理端和用户订阅列表仍可能展示 `NULL + 历史 usage`。

## 目标

- API 计费入口在限额判断前刷新窗口时，`NULL` 窗口和过期窗口都按“当前窗口重新开始”处理。
- 订阅列表和管理端列表即使没有触发 API 请求，也不能展示 `NULL` 窗口下的历史 usage。
- daily、weekly、monthly 三个窗口语义一致，避免只修 daily。
- 用回归测试锁住 `NULL + usage > 0`、过期窗口、当前窗口三类状态。

## 非目标

- 不改变订阅、流量卡、余额三套计费优先级。
- 不把 `usage_logs` 聚合查询塞进每次 API 计费热路径。
- 不引入新的定时任务作为本次根因修复前提。
- 不在源码、文档或日志中写入任何 API Key、内部 token、HMAC secret、SMTP 密码。

## 方案取舍

### 方案 A：只修仓储 SQL

把 `daily_usage_usd` 的条件从 `daily_window_start IS NOT NULL AND daily_window_start < $2` 改成 `daily_window_start IS NULL OR daily_window_start < $2`，weekly/monthly 同理。

优点是改动最小，能修 API 请求路径。缺点是管理端列表仍可能在用户没有发请求时看到陈旧数据。

### 方案 B：每次刷新都按 `usage_logs` 聚合回填

`RefreshExpiredUsageWindows` 在 `NULL` 或过期时查询当前 daily/weekly/monthly 窗口内的 `usage_logs.total_cost`，再写回订阅 usage。

优点是事实源最精确。缺点是把大表聚合放进请求热路径，且需要处理 usage log 写入和订阅 usage 写入的时序，风险大于收益。

### 推荐方案 C：写路径清零 + 读路径惰性归零 + 上线校准检查

写路径中，`NULL` 窗口和过期窗口统一清零并设置当前窗口；读路径中，订阅列表对 `NULL + usage > 0` 也归零展示；上线前后用一次性 SQL 按 `usage_logs` 聚合校准运行态数据。

这是推荐方案。它把请求热路径保持简单、原子、低成本，同时确保管理端展示不会再泄漏旧值；运行态已有错账用校准解决，不让通用请求路径背负历史修复成本。

## 设计

### 1. 仓储层刷新语义

修改 `backend/internal/repository/user_subscription_repo.go` 的 `RefreshExpiredUsageWindows`：

- daily：`daily_window_start IS NULL OR daily_window_start < $2` 时，`daily_usage_usd = 0` 且 `daily_window_start = $2`。
- weekly：`weekly_window_start IS NULL OR weekly_window_start < $3` 时，`weekly_usage_usd = 0` 且 `weekly_window_start = $3`。
- monthly：`monthly_window_start IS NULL OR monthly_window_start + INTERVAL '30 days' <= $5` 时，`monthly_usage_usd = 0` 且 `monthly_window_start = $4`。

`WHERE` 条件保持“至少一个窗口需要刷新才更新”，避免当前窗口内重复请求频繁写 DB。这个 SQL 仍是单条 `UPDATE`，不引入读改写竞态。

### 2. Service 层列表展示语义

修改 `normalizeExpiredWindows`，让读模型也遵循 quota 视角：

- 非空且过期：继续把对应 usage 置 0。
- `NULL` 且 usage 大于 0：把对应 usage 置 0。
- 当前窗口：保留 usage。

这个函数仍只影响返回数据，不写数据库。这样管理端和用户订阅列表不会因为用户没有再发 API 请求而展示历史累计值。

不建议在列表接口中批量调用 `RefreshExpiredUsageWindows`，原因是管理端分页列表属于读接口，批量写入会放大数据库写压力，也会让普通查看行为改变运行态数据。

### 3. Progress 计算

`GetSubscriptionProgress` 当前从 DB 读单条订阅并调用 `calculateProgress`。本次应在进度计算前复用同一套展示归一化逻辑，避免进度接口和列表接口对 `NULL + usage` 给出不同答案。

实现上优先抽一个小函数：

```go
func normalizeExpiredWindowForDisplay(sub *UserSubscription, now time.Time)
```

列表用循环调用，进度计算前对单条副本调用。函数只改内存对象，不持久化。

### 4. 测试策略

先写失败测试，再改实现。

仓储层新增集成测试，直接落真实测试库，验证 SQL 结果而不是只用 sqlmock 匹配字符串：

- `daily_window_start=NULL, daily_usage_usd=9.9` 刷新后 daily usage 为 0，daily window 为当前日 0 点。
- `weekly_window_start=NULL, weekly_usage_usd=19.9` 刷新后 weekly usage 为 0，weekly window 为当前周起点。
- `monthly_window_start=NULL, monthly_usage_usd=29.9` 刷新后 monthly usage 为 0，monthly window 为传入 `monthlyStart`。
- 当前窗口内的 usage 不被清零。

Service 层新增单元测试：

- 列表归一化对 `NULL + usage > 0` 返回 usage 0。
- 列表归一化对当前窗口保留 usage。
- progress 计算前归一化，避免 progress 仍展示 `NULL + usage`。

现有 `backend/internal/service/billing_cache_service_subscription_window_test.go` 的 stub 也要同步改成 NULL 时清 usage，防止测试假实现继续复刻旧 bug。

### 5. 上线与数据校准

源码修复不会自动重写所有历史行。上线前后需要做只读检查：

```sql
SELECT COUNT(*)
FROM user_subscriptions
WHERE deleted_at IS NULL
  AND status = 'active'
  AND (
    (daily_window_start IS NULL AND daily_usage_usd > 0)
    OR (weekly_window_start IS NULL AND weekly_usage_usd > 0)
    OR (monthly_window_start IS NULL AND monthly_usage_usd > 0)
  );
```

如果数量大于 0，按部署当天窗口用 `usage_logs.total_cost` 聚合校准 active 订阅，并删除 `billing:sub:*` Redis 缓存。公网 18084 在 2026-07-07 已做过 daily 校准；本次发布时仍要复查 weekly/monthly 和新产生的数据。

## 风险与缓解

- 风险：简单清零会低估同一窗口内、但窗口字段仍为 NULL 的真实新用量。缓解：运行态上线前使用 `usage_logs` 聚合校准；请求热路径不承担历史修复。
- 风险：读路径归一化与写路径刷新语义分叉。缓解：把展示归一化集中到单个函数，列表和 progress 复用。
- 风险：monthly 是 30 天滚动窗口，不是自然月。缓解：继续使用现有 `monthly_window_start + INTERVAL '30 days' <= now` 规则，不改业务语义。

## 验收标准

- `go test -count=1 -tags=unit ./internal/repository ./internal/service` 通过。
- 新增集成测试能证明 `NULL + usage > 0` 被清零，旧实现下会失败。
- 管理端订阅列表、用户订阅列表和 progress 接口不再返回 `NULL` 窗口下的历史 usage。
- 发布前只读检查中 `NULL window + usage > 0` 计数为 0，或已按 `usage_logs` 聚合完成校准。

## 后续实施顺序

1. 写仓储集成失败测试。
2. 写 service 展示归一化失败测试。
3. 修改 `RefreshExpiredUsageWindows` SQL。
4. 抽取并复用展示归一化函数。
5. 修正相关测试 stub。
6. 跑后端相关测试。
7. 新建实施结果文档，并更新 `AGENTS.md`。
