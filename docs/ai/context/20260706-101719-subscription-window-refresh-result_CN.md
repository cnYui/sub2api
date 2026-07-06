# 订阅窗口刷新本地实现结果

## 结论

- 已按 `docs/ai/context/20260706-095853-subscription-daily-window-refresh-requirements_CN.md` 修改本地 `main` 代码。
- 本次只改源码和测试，未连接、迁移、重启、替换任何公网或本地运行态容器。

## 代码改动

- `BillingCacheService.checkSubscriptionEligibility()` 在套餐限额判断前统一检查订阅窗口是否过期。
- 新增 request-scoped `singleflight`，同一用户同一分组跨日并发请求只合并执行一次窗口刷新。
- `SubscriptionCacheData` 和 Redis `billing:sub:{userID}:{groupID}` hash 新增：
  - `daily_window_start`
  - `weekly_window_start`
  - `monthly_window_start`
- 旧 Redis 订阅缓存缺窗口字段时视为不可用于窗口判断；有仓储时回源 DB 并重建缓存，无仓储测试场景继续兼容旧 cache。
- 新增 `UserSubscriptionRepository.RefreshExpiredUsageWindows()`：
  - `daily_window_start < timezone.StartOfDay(now)` 时清零日用量并推进日窗口。
  - `weekly_window_start < timezone.StartOfWeek(now)` 时清零周用量并推进周窗口。
  - `monthly_window_start + 30 days <= now` 时清零月用量并以当前时间作为新月窗口起点。
  - SQL `WHERE` 带过期条件，避免并发请求重复清零或覆盖新窗口用量。

## 测试覆盖

- 新增 `backend/internal/service/billing_cache_service_subscription_window_test.go`：
  - 旧日窗口超限在请求入口刷新后放行。
  - 旧 Redis cache 缺窗口字段时回源 DB。
  - 旧周/月窗口超限在请求入口刷新后放行。
  - 当前日窗口真实超限仍返回 `ErrDailyLimitExceeded`。
- 新增 `backend/internal/repository/user_subscription_repo_window_test.go`：
  - 条件刷新 SQL 包含 daily/weekly/monthly 过期条件。
  - 无过期窗口时返回 `updated=false`。
- 更新 `backend/internal/repository/billing_cache_test.go`：
  - 旧订阅缓存缺窗口字段时解析失败。

## 验证

已通过：

```bash
cd backend
go test -count=1 -tags=unit ./internal/service ./internal/repository
```

结果：

- `github.com/Wei-Shaw/sub2api/internal/service` 通过，用时约 90 秒。
- `github.com/Wei-Shaw/sub2api/internal/repository` 通过，用时约 4 秒。

## 后续上线提示

- 上线只需替换应用代码；不需要新增 migration。
- 发布后旧 Redis 订阅缓存会因缺窗口字段被回源 DB 自愈，无需手动清 Redis。
- 如果线上仍有过期窗口旧用量，第一次 API 资格检查会先刷新 DB 再判断限额。
