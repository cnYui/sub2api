# 订阅 NULL 窗口用量残留修复结果

## 改动

- `RefreshExpiredUsageWindows` 已让 daily/weekly/monthly 的 `NULL` 窗口初始化同步清零 usage，并继续保持当前窗口内不重复写 DB。
- 订阅列表读路径已抽出 `normalizeExpiredWindowForDisplay`，`NULL + usage > 0` 不再返回历史 usage。
- `GetSubscriptionProgress` 在计算 progress 前复用展示归一化，避免 progress 与列表展示不一致。
- `billing_cache_service_subscription_window_test.go` 的测试 stub 已与真实仓储语义保持一致。

## 验证

- RED：`go test -count=1 -tags=integration ./internal/repository -run 'TestUserSubscriptionRepoSuite/TestRefreshExpiredUsageWindows_(NullWindowsResetUsage|CurrentWindowsKeepUsage)'` 在旧实现下失败，`DailyUsageUSD` 仍为 `9.9`。
- RED：`go test -count=1 ./internal/service -run 'TestNormalizeExpiredWindows_|TestGetSubscriptionProgress_NormalizesExpiredWindowBeforeProgress'` 在旧实现下失败，列表保留 `9.9` 且 progress 返回非空 Daily。
- GREEN：`go test -count=1 -tags=integration ./internal/repository -run 'TestUserSubscriptionRepoSuite/TestRefreshExpiredUsageWindows_(NullWindowsResetUsage|CurrentWindowsKeepUsage)'` 通过。
- GREEN：`go test -count=1 ./internal/service -run 'TestNormalizeExpiredWindows_|TestGetSubscriptionProgress_NormalizesExpiredWindowBeforeProgress'` 通过。
- 全量相关 unit：`go test -count=1 -tags=unit ./internal/repository ./internal/service` 通过。
- 新增仓储 integration：`go test -count=1 -tags=integration ./internal/repository -run 'TestUserSubscriptionRepoSuite/TestRefreshExpiredUsageWindows_(NullWindowsResetUsage|CurrentWindowsKeepUsage)'` 通过。
- 格式检查：`git diff --check` 通过。

## 运行态提醒

源码修复不自动重写历史行。发布前后仍需对公网库执行只读检查：

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

如果结果大于 0，应按当前窗口用 `usage_logs.total_cost` 聚合校准，并删除 `billing:sub:*` Redis 缓存。
