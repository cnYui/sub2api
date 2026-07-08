# 东八区订阅日用量自动刷新 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除 API 入口 DB 写入式惰性刷新，改为东八区 00:00 后台主动推进 active 订阅窗口，并保证完成时扣费、`/v1/usage` 和前端展示都只认当天真实窗口。

**Architecture:** 订阅表继续作为准入快速事实源；`usage_logs.created_at` 是用量归属事实源。后台任务只推进 stale active 订阅，完成时扣费在同一账务事务内自愈窗口，准入和展示使用只读窗口归一化，不再在 API 入口写库清零。

**Tech Stack:** Go、PostgreSQL、Ent、Redis leader lock、Gin middleware、Wire DI、Go unit/integration tests。

---

## 文件结构

- Modify: `backend/internal/service/user_subscription_port.go`
  - 增加后台校准返回结构和 `UserSubscriptionRepository` 方法签名。
- Modify: `backend/internal/repository/user_subscription_repo.go`
  - 新增 active 订阅 daily window 批量校准 SQL。
  - 修改 `IncrementUsage` 的 legacy fallback 路径，使完成时扣费可自愈 daily/weekly/monthly 窗口。
- Modify: `backend/internal/repository/usage_billing_repo.go`
  - `UsageBillingRepository.Apply` 的订阅扣费使用 `UsageBillingCommand.CompletedAt` 推进窗口。
- Modify: `backend/internal/service/usage_billing.go`
  - 增加 `CompletedAt time.Time`，`Normalize()` 在零值时补 `time.Now()`，且不纳入 dedup fingerprint。
- Modify: `backend/internal/service/gateway_service.go`
  - `buildUsageBillingCommand()` 从 `usageLog.CreatedAt` 传递完成时间。
- Modify: `backend/internal/service/billing_cache_service.go`
  - 删除 `checkSubscriptionEligibility()` 中的 DB 写入式窗口刷新调用。
  - 增加只读窗口归一化，避免 00:00 后后台任务短暂未执行时误拒。
- Create: `backend/internal/service/subscription_usage_window_scheduler.go`
  - 新增东八区 daily window 后台任务，复用 `tryAcquireSingletonLeaderLock`。
- Create: `backend/internal/service/subscription_usage_window_scheduler_test.go`
  - 覆盖启动补偿、leader lock、stale 监控和批量推进。
- Modify: `backend/internal/service/wire.go`
  - 新增 provider，启动后台任务并注入 `LeaderLockCache`、`*sql.DB`、`BillingCacheService`。
- Modify: `backend/cmd/server/wire.go`
  - cleanup 中停止新后台任务。
- Modify: `backend/cmd/server/wire_gen.go`
  - 通过 Wire 重新生成。
- Modify: `backend/internal/server/middleware/effective_group.go`
  - 自动 Key endpoint policy 支持 `/v1/usage`。
- Modify: `backend/internal/handler/gateway_handler.go`
  - `/v1/usage` 订阅块返回前做窗口归一化。
- Test: `backend/internal/service/billing_cache_service_subscription_window_test.go`
- Test: `backend/internal/repository/user_subscription_repo_integration_test.go`
- Test: `backend/internal/repository/usage_billing_repo_integration_test.go`
- Test: `backend/internal/server/middleware/effective_group_test.go`
- Test: `backend/internal/handler/gateway_handler_usage_test.go` 或现有 gateway handler 测试文件。

---

### Task 1: 定义完成时间和后台校准接口

**Files:**
- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/service/user_subscription_port.go`

- [ ] **Step 1: 给 `UsageBillingCommand` 写完成时间字段**

在 `backend/internal/service/usage_billing.go` 的 `UsageBillingCommand` 中加入：

```go
CompletedAt time.Time
```

在 import 中加入 `time`。

- [ ] **Step 2: 修改 `Normalize()` 补完成时间**

将 `Normalize()` 改成：

```go
func (c *UsageBillingCommand) Normalize() {
	if c == nil {
		return
	}
	c.RequestID = strings.TrimSpace(c.RequestID)
	if c.CompletedAt.IsZero() {
		c.CompletedAt = time.Now()
	}
	if strings.TrimSpace(c.RequestFingerprint) == "" {
		c.RequestFingerprint = buildUsageBillingFingerprint(c)
	}
}
```

注意：不要把 `CompletedAt` 写进 `buildUsageBillingFingerprint()`。同一个 request_id 的重试如果完成时间不同，也不应该造成 fingerprint conflict。

- [ ] **Step 3: 定义后台校准返回类型**

在 `backend/internal/service/user_subscription_port.go` 中加入：

```go
type SubscriptionWindowCacheKey struct {
	SubscriptionID int64
	UserID         int64
	GroupID        int64
}

type SubscriptionDailyWindowCalibrationResult struct {
	Updated        []SubscriptionWindowCacheKey
	UpdatedCount   int64
	StaleRemaining int64
}
```

- [ ] **Step 4: 扩展 `UserSubscriptionRepository` 接口**

在 `UserSubscriptionRepository` 中加入：

```go
CalibrateActiveDailyUsageWindows(ctx context.Context, dailyStart, upperBound, now time.Time, batchSize int) (*SubscriptionDailyWindowCalibrationResult, error)
CountStaleActiveDailyWindows(ctx context.Context, dailyStart, now time.Time) (int64, error)
```

- [ ] **Step 5: 运行接口层编译检查**

Run:

```bash
go test -count=1 -tags=unit ./internal/service
```

Expected: 当前会失败，提示仓储 stub 或实现尚未满足新增接口。这是本任务的预期红灯。

---

### Task 2: 完成时订阅扣费自愈窗口

**Files:**
- Modify: `backend/internal/repository/usage_billing_repo_integration_test.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Modify: `backend/internal/service/gateway_service.go`

- [ ] **Step 1: 写 integration 失败测试，覆盖 stale daily window**

在 `backend/internal/repository/usage_billing_repo_integration_test.go` 新增：

```go
func TestUsageBillingRepositoryApply_SubscriptionBillingAdvancesExpiredDailyWindowAtCompletedAt(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-sub-window-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-sub-window-" + uuid.NewString(),
		Platform:         service.PlatformOpenAI,
		SubscriptionType: service.SubscriptionTypeSubscription,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  user.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-billing-sub-window-" + uuid.NewString(),
		Name:    "billing-sub-window",
	})

	today := timezone.StartOfDay(timezone.Now())
	yesterday := today.Add(-24 * time.Hour)
	completedAt := today.Add(90 * time.Second)
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:             user.ID,
		GroupID:            group.ID,
		DailyUsageUSD:      19.5,
		WeeklyUsageUSD:     20.5,
		MonthlyUsageUSD:    21.5,
		DailyWindowStart:   &yesterday,
		WeeklyWindowStart:  &yesterday,
		MonthlyWindowStart: &yesterday,
	})

	cmd := &service.UsageBillingCommand{
		RequestID:        uuid.NewString(),
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		SubscriptionID:   &subscription.ID,
		SubscriptionCost: 0.75,
		CompletedAt:      completedAt,
	}
	result, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)

	var dailyUsage float64
	var dailyWindow time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT daily_usage_usd, daily_window_start
		FROM user_subscriptions
		WHERE id = $1
	`, subscription.ID).Scan(&dailyUsage, &dailyWindow))
	require.InDelta(t, 0.75, dailyUsage, 0.000001)
	require.WithinDuration(t, today, dailyWindow, time.Microsecond)
}
```

需要 import `github.com/Wei-Shaw/sub2api/internal/pkg/timezone`。

- [ ] **Step 2: 写 integration 测试，覆盖今天窗口继续累加**

同文件新增：

```go
func TestUsageBillingRepositoryApply_SubscriptionBillingAccumulatesCurrentDailyWindow(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-sub-current-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-sub-current-" + uuid.NewString(),
		Platform:         service.PlatformOpenAI,
		SubscriptionType: service.SubscriptionTypeSubscription,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  user.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-billing-sub-current-" + uuid.NewString(),
		Name:    "billing-sub-current",
	})

	today := timezone.StartOfDay(timezone.Now())
	completedAt := today.Add(2 * time.Minute)
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:             user.ID,
		GroupID:            group.ID,
		DailyUsageUSD:      1.25,
		DailyWindowStart:   &today,
		WeeklyWindowStart:  &today,
		MonthlyWindowStart: &today,
	})

	cmd := &service.UsageBillingCommand{
		RequestID:        uuid.NewString(),
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		SubscriptionID:   &subscription.ID,
		SubscriptionCost: 0.75,
		CompletedAt:      completedAt,
	}
	_, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)

	var dailyUsage float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id = $1", subscription.ID).Scan(&dailyUsage))
	require.InDelta(t, 2.0, dailyUsage, 0.000001)
}
```

- [ ] **Step 3: Run failing tests**

Run:

```bash
go test -count=1 -tags=integration ./internal/repository -run 'TestUsageBillingRepositoryApply_SubscriptionBilling'
```

Expected: 新 stale window 测试失败，当前实现会把 0.75 累加到 19.5。

- [ ] **Step 4: 修改 `buildUsageBillingCommand()` 传递完成时间**

在 `backend/internal/service/gateway_service.go` 的 `buildUsageBillingCommand()` 中加入：

```go
if usageLog != nil && !usageLog.CreatedAt.IsZero() {
	cmd.CompletedAt = usageLog.CreatedAt
}
```

- [ ] **Step 5: 修改 `incrementUsageBillingSubscription()`**

将签名改为：

```go
func incrementUsageBillingSubscription(ctx context.Context, tx *sql.Tx, subscriptionID int64, costUSD float64, completedAt time.Time) error
```

在 `applyUsageBillingEffects()` 中调用：

```go
if err := incrementUsageBillingSubscription(ctx, tx, *cmd.SubscriptionID, cmd.SubscriptionCost, cmd.CompletedAt); err != nil {
	return err
}
```

SQL 使用完成时间推进窗口：

```go
completedAt = completedAt.In(timezone.Location())
dailyStart := timezone.StartOfDay(completedAt)
weeklyStart := timezone.StartOfWeek(completedAt)

const updateSQL = `
	UPDATE user_subscriptions us
	SET
		daily_usage_usd = CASE
			WHEN us.daily_window_start IS NULL OR us.daily_window_start < $3 THEN $1
			ELSE us.daily_usage_usd + $1
		END,
		daily_window_start = CASE
			WHEN us.daily_window_start IS NULL OR us.daily_window_start < $3 THEN $3
			ELSE us.daily_window_start
		END,
		weekly_usage_usd = CASE
			WHEN us.weekly_window_start IS NULL OR us.weekly_window_start < $4 THEN $1
			ELSE us.weekly_usage_usd + $1
		END,
		weekly_window_start = CASE
			WHEN us.weekly_window_start IS NULL OR us.weekly_window_start < $4 THEN $4
			ELSE us.weekly_window_start
		END,
		monthly_usage_usd = CASE
			WHEN us.monthly_window_start IS NULL OR us.monthly_window_start + INTERVAL '30 days' <= $5 THEN $1
			ELSE us.monthly_usage_usd + $1
		END,
		monthly_window_start = CASE
			WHEN us.monthly_window_start IS NULL OR us.monthly_window_start + INTERVAL '30 days' <= $5 THEN $5
			ELSE us.monthly_window_start
		END,
		updated_at = $5
	FROM groups g
	WHERE us.id = $2
		AND us.deleted_at IS NULL
		AND us.group_id = g.id
		AND g.deleted_at IS NULL
`
res, err := tx.ExecContext(ctx, updateSQL, costUSD, subscriptionID, dailyStart, weeklyStart, completedAt)
```

需要 import `time` 和 `github.com/Wei-Shaw/sub2api/internal/pkg/timezone`。

- [ ] **Step 6: 修改 legacy `IncrementUsage()`**

在 `backend/internal/repository/user_subscription_repo.go` 的 `IncrementUsage()` 中使用同样 CASE 逻辑。完成时间用 `now := timezone.Now()`，因为 legacy fallback 没有 `UsageBillingCommand.CompletedAt`。

- [ ] **Step 7: Run tests**

Run:

```bash
go test -count=1 -tags=integration ./internal/repository -run 'TestUsageBillingRepositoryApply_SubscriptionBilling'
go test -count=1 -tags=unit ./internal/service
```

Expected: repository integration 新增测试通过；service unit 仍可能因为后续接口 stub 未补齐而失败，记录具体失败进入下一任务。

---

### Task 3: 仓储实现 active daily window 后台校准

**Files:**
- Modify: `backend/internal/repository/user_subscription_repo_integration_test.go`
- Modify: `backend/internal/repository/user_subscription_repo.go`

- [ ] **Step 1: 写 integration 失败测试，聚合今天日志推进 stale active 订阅**

在 `backend/internal/repository/user_subscription_repo_integration_test.go` 新增：

```go
func (s *UserSubscriptionRepoSuite) TestCalibrateActiveDailyUsageWindows_UsesTodayUsageLogs() {
	user := s.mustCreateUser("calibrate-daily@test.com", service.RoleUser)
	group := s.mustCreateGroup("g-calibrate-daily")
	today := timezone.StartOfDay(timezone.Now())
	yesterday := today.Add(-24 * time.Hour)
	upperBound := today.Add(10 * time.Minute)
	now := upperBound

	sub := s.mustCreateSubscription(user.ID, group.ID, func(c *dbent.UserSubscriptionCreate) {
		c.SetDailyWindowStart(yesterday)
		c.SetDailyUsageUsd(99)
		c.SetStatus(service.SubscriptionStatusActive)
		c.SetExpiresAt(now.Add(24 * time.Hour))
	})
	apiKey := s.mustCreateAPIKey(user.ID, group.ID, "sk-calibrate-daily-"+uuid.NewString())

	s.mustCreateUsageLog(apiKey.ID, user.ID, group.ID, sub.ID, 1.25, today.Add(30*time.Second))
	s.mustCreateUsageLog(apiKey.ID, user.ID, group.ID, sub.ID, 2.75, today.Add(2*time.Minute))
	s.mustCreateUsageLog(apiKey.ID, user.ID, group.ID, sub.ID, 100, yesterday.Add(23*time.Hour))
	s.mustCreateUsageLog(apiKey.ID, user.ID, group.ID, sub.ID, 50, upperBound.Add(time.Second))

	result, err := s.repo.CalibrateActiveDailyUsageWindows(s.ctx, today, upperBound, now, 100)
	s.Require().NoError(err)
	s.Require().Equal(int64(1), result.UpdatedCount)
	s.Require().Len(result.Updated, 1)
	s.Require().Equal(sub.ID, result.Updated[0].SubscriptionID)
	s.Require().Equal(user.ID, result.Updated[0].UserID)
	s.Require().Equal(group.ID, result.Updated[0].GroupID)

	got, err := s.repo.GetByID(s.ctx, sub.ID)
	s.Require().NoError(err)
	s.Require().InDelta(4.0, got.DailyUsageUSD, 0.000001)
	s.Require().NotNil(got.DailyWindowStart)
	s.Require().WithinDuration(today, *got.DailyWindowStart, time.Microsecond)
}
```

若测试文件缺少 `mustCreateAPIKey` 或 `mustCreateUsageLog` helper，先新增 helper，直接用 Ent 创建 `api_keys` 和 `usage_logs`，只填必需字段。

- [ ] **Step 2: 写 integration 测试，今天窗口不覆盖**

新增：

```go
func (s *UserSubscriptionRepoSuite) TestCalibrateActiveDailyUsageWindows_DoesNotOverwriteCurrentWindow() {
	user := s.mustCreateUser("calibrate-current@test.com", service.RoleUser)
	group := s.mustCreateGroup("g-calibrate-current")
	today := timezone.StartOfDay(timezone.Now())
	now := today.Add(15 * time.Minute)

	sub := s.mustCreateSubscription(user.ID, group.ID, func(c *dbent.UserSubscriptionCreate) {
		c.SetDailyWindowStart(today)
		c.SetDailyUsageUsd(7.5)
		c.SetStatus(service.SubscriptionStatusActive)
		c.SetExpiresAt(now.Add(24 * time.Hour))
	})

	result, err := s.repo.CalibrateActiveDailyUsageWindows(s.ctx, today, now, now, 100)
	s.Require().NoError(err)
	s.Require().Equal(int64(0), result.UpdatedCount)

	got, err := s.repo.GetByID(s.ctx, sub.ID)
	s.Require().NoError(err)
	s.Require().InDelta(7.5, got.DailyUsageUSD, 0.000001)
}
```

- [ ] **Step 3: Run failing tests**

Run:

```bash
go test -count=1 -tags=integration ./internal/repository -run 'TestUserSubscriptionRepoSuite/TestCalibrateActiveDailyUsageWindows'
```

Expected: build fails because repository methods are not implemented yet.

- [ ] **Step 4: 实现 `CalibrateActiveDailyUsageWindows()`**

在 `backend/internal/repository/user_subscription_repo.go` 新增方法。核心 SQL：

```go
if batchSize <= 0 {
	batchSize = 200
}

const updateSQL = `
	WITH candidates AS (
		SELECT id, user_id, group_id
		FROM user_subscriptions
		WHERE deleted_at IS NULL
			AND status = $4
			AND expires_at > $3
			AND (daily_window_start IS NULL OR daily_window_start < $1)
		ORDER BY id
		LIMIT $5
		FOR UPDATE SKIP LOCKED
	),
	usage_today AS (
		SELECT ul.subscription_id, COALESCE(SUM(ul.total_cost), 0) AS total_cost
		FROM usage_logs ul
		JOIN candidates c ON c.id = ul.subscription_id
		WHERE ul.created_at >= $1
			AND ul.created_at < $2
		GROUP BY ul.subscription_id
	),
	updated AS (
		UPDATE user_subscriptions us
		SET daily_usage_usd = COALESCE(ut.total_cost, 0),
			daily_window_start = $1,
			updated_at = $3
		FROM candidates c
		LEFT JOIN usage_today ut ON ut.subscription_id = c.id
		WHERE us.id = c.id
		RETURNING us.id, us.user_id, us.group_id
	)
	SELECT id, user_id, group_id
	FROM updated
	ORDER BY id
`
rows, err := client.QueryContext(ctx, updateSQL, dailyStart, upperBound, now, service.SubscriptionStatusActive, batchSize)
```

读取 `updated` 列表后调用 `CountStaleActiveDailyWindows(ctx, dailyStart, now)` 得到 `StaleRemaining`。

- [ ] **Step 5: 实现 `CountStaleActiveDailyWindows()`**

SQL：

```go
const countSQL = `
	SELECT COUNT(*)
	FROM user_subscriptions
	WHERE deleted_at IS NULL
		AND status = $3
		AND expires_at > $2
		AND (daily_window_start IS NULL OR daily_window_start < $1)
`
err := client.QueryRowContext(ctx, countSQL, dailyStart, now, service.SubscriptionStatusActive).Scan(&count)
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test -count=1 -tags=integration ./internal/repository -run 'TestUserSubscriptionRepoSuite/TestCalibrateActiveDailyUsageWindows'
```

Expected: 新增校准测试通过。

---

### Task 4: 删除 API 入口写库刷新，改为只读归一化准入

**Files:**
- Modify: `backend/internal/service/billing_cache_service_subscription_window_test.go`
- Modify: `backend/internal/service/billing_cache_service.go`

- [ ] **Step 1: 替换旧刷新测试为不写库测试**

把 `TestCheckBillingEligibility_RefreshesExpiredDailyWindowBeforeLimitCheck` 改名为：

```go
func TestCheckBillingEligibility_NormalizesExpiredDailyWindowWithoutRepositoryWrite(t *testing.T)
```

断言从 `refreshCalls == 1` 改为：

```go
require.NoError(t, err)
require.Zero(t, repo.refreshCalls)
require.InDelta(t, 19.5, repo.sub.DailyUsageUSD, 0.000001)
require.True(t, repo.sub.DailyWindowStart.Equal(yesterday))
```

该测试表达：昨天窗口用满不影响今天准入，但 API 入口不写库。

- [ ] **Step 2: 替换 weekly/monthly 旧刷新测试**

把 `TestCheckBillingEligibility_RefreshesExpiredWeeklyAndMonthlyWindowsBeforeLimitCheck` 改成只读归一化测试，断言：

```go
require.NoError(t, err)
require.Zero(t, repo.refreshCalls)
require.InDelta(t, 20.5, repo.sub.WeeklyUsageUSD, 0.000001)
require.InDelta(t, 51.0, repo.sub.MonthlyUsageUSD, 0.000001)
```

- [ ] **Step 3: 保留今天窗口真实超限测试**

`TestCheckBillingEligibility_CurrentDailyWindowStillRejectsRealExhaustion` 保持语义不变，确保今天窗口 `DailyUsageUSD >= limit` 仍拒绝。

- [ ] **Step 4: Run failing tests**

Run:

```bash
go test -count=1 -tags=unit ./internal/service -run 'TestCheckBillingEligibility_.*Window'
```

Expected: 至少新的“不写库”测试失败，因为当前实现仍调用 `RefreshExpiredUsageWindows`。

- [ ] **Step 5: 删除写库刷新调用**

在 `backend/internal/service/billing_cache_service.go` 的 `checkSubscriptionEligibility()` 删除对 `refreshExpiredSubscriptionWindowsIfNeeded()` 的调用块。

保留函数 `refreshExpiredSubscriptionWindowsIfNeeded()` 可先不删，降低本次改动面；实施后用 `rg "refreshExpiredSubscriptionWindowsIfNeeded"` 确认无调用，再决定是否清理私有函数。

- [ ] **Step 6: 增加只读归一化 helper**

在同文件新增：

```go
func normalizeSubscriptionCacheForEligibility(subData *subscriptionCacheData, now time.Time) subscriptionCacheData {
	normalized := *subData
	dailyStart := timezone.StartOfDay(now)
	weeklyStart := timezone.StartOfWeek(now)

	if normalized.DailyWindowStart == nil || normalized.DailyWindowStart.Before(dailyStart) {
		normalized.DailyUsage = 0
	}
	if normalized.WeeklyWindowStart == nil || normalized.WeeklyWindowStart.Before(weeklyStart) {
		normalized.WeeklyUsage = 0
	}
	if normalized.MonthlyWindowStart == nil || !normalized.MonthlyWindowStart.Add(30*24*time.Hour).After(now) {
		normalized.MonthlyUsage = 0
	}
	return normalized
}
```

在限额检查前调用：

```go
normalized := normalizeSubscriptionCacheForEligibility(subData, timezone.Now())
subData = &normalized
```

- [ ] **Step 7: Run tests**

Run:

```bash
go test -count=1 -tags=unit ./internal/service -run 'TestCheckBillingEligibility_.*Window'
```

Expected: 窗口准入相关 unit tests 通过。

---

### Task 5: 新增东八区后台任务服务

**Files:**
- Create: `backend/internal/service/subscription_usage_window_scheduler.go`
- Create: `backend/internal/service/subscription_usage_window_scheduler_test.go`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/cmd/server/wire.go`
- Modify: `backend/cmd/server/wire_gen.go`

- [ ] **Step 1: 写后台任务 unit 测试**

创建 `backend/internal/service/subscription_usage_window_scheduler_test.go`，定义 fake repo：

```go
type subscriptionUsageWindowRepoStub struct {
	userSubRepoNoop
	calibrateCalls int
	countCalls     int
	result         *SubscriptionDailyWindowCalibrationResult
	err            error
}

func (r *subscriptionUsageWindowRepoStub) CalibrateActiveDailyUsageWindows(ctx context.Context, dailyStart, upperBound, now time.Time, batchSize int) (*SubscriptionDailyWindowCalibrationResult, error) {
	r.calibrateCalls++
	if r.err != nil {
		return nil, r.err
	}
	if r.result != nil {
		return r.result, nil
	}
	return &SubscriptionDailyWindowCalibrationResult{}, nil
}

func (r *subscriptionUsageWindowRepoStub) CountStaleActiveDailyWindows(ctx context.Context, dailyStart, now time.Time) (int64, error) {
	r.countCalls++
	return 0, nil
}
```

新增测试：

```go
func TestSubscriptionUsageWindowScheduler_RunOnceSkipsWhenNotLeader(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	_, _ = cache.TryAcquireLeaderLock(context.Background(), subscriptionUsageWindowLeaderLockKey, "peer", time.Minute)
	repo := &subscriptionUsageWindowRepoStub{}
	svc := NewSubscriptionUsageWindowScheduler(repo, nil, 200)
	svc.SetLeaderLock(cache, nil)

	svc.runOnce(context.Background(), timezone.StartOfDay(timezone.Now()).Add(time.Minute))

	require.Zero(t, repo.calibrateCalls)
}

func TestSubscriptionUsageWindowScheduler_RunOnceCalibratesBatches(t *testing.T) {
	repo := &subscriptionUsageWindowRepoStub{
		result: &SubscriptionDailyWindowCalibrationResult{
			Updated: []SubscriptionWindowCacheKey{{SubscriptionID: 1, UserID: 2, GroupID: 3}},
		},
	}
	cache := &fakeLeaderLockCache{}
	svc := NewSubscriptionUsageWindowScheduler(repo, nil, 200)
	svc.SetLeaderLock(cache, nil)

	svc.runOnce(context.Background(), timezone.StartOfDay(timezone.Now()).Add(time.Minute))

	require.Equal(t, 1, repo.calibrateCalls)
}
```

- [ ] **Step 2: Run failing tests**

Run:

```bash
go test -count=1 -tags=unit ./internal/service -run 'TestSubscriptionUsageWindowScheduler'
```

Expected: build fails because scheduler does not exist.

- [ ] **Step 3: 创建 scheduler 服务**

创建 `backend/internal/service/subscription_usage_window_scheduler.go`，核心结构：

```go
const (
	subscriptionUsageWindowLeaderLockKey = "subscription:usage_window:daily:leader"
	subscriptionUsageWindowLeaderLockTTL = 10 * time.Minute
	subscriptionUsageWindowTickInterval  = 1 * time.Minute
	subscriptionUsageWindowBatchSize     = 200
)

type SubscriptionUsageWindowScheduler struct {
	userSubRepo  UserSubscriptionRepository
	billingCache *BillingCacheService
	batchSize    int
	lockCache    LeaderLockCache
	db           *sql.DB
	instanceID   string
	stopCh       chan struct{}
	stopOnce     sync.Once
	wg           sync.WaitGroup
}
```

构造函数：

```go
func NewSubscriptionUsageWindowScheduler(userSubRepo UserSubscriptionRepository, billingCache *BillingCacheService, batchSize int) *SubscriptionUsageWindowScheduler {
	if batchSize <= 0 {
		batchSize = subscriptionUsageWindowBatchSize
	}
	return &SubscriptionUsageWindowScheduler{
		userSubRepo:  userSubRepo,
		billingCache: billingCache,
		batchSize:    batchSize,
		instanceID:   uuid.NewString(),
		stopCh:       make(chan struct{}),
	}
}
```

增加 `SetLeaderLock`、`Start`、`Stop`、`loop`、`runOnce`。

- [ ] **Step 4: 实现 lock 和批处理**

在 `runOnce` 中：

```go
release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, subscriptionUsageWindowLeaderLockKey, s.instanceID, subscriptionUsageWindowLeaderLockTTL)
if !ok {
	return
}
defer release()

dailyStart := timezone.StartOfDay(now)
upperBound := now
for {
	result, err := s.userSubRepo.CalibrateActiveDailyUsageWindows(ctx, dailyStart, upperBound, now, s.batchSize)
	if err != nil {
		logger.LegacyPrintf("service.subscription_usage_window", "[SubscriptionUsageWindow] calibrate failed: %v", err)
		return
	}
	for _, key := range result.Updated {
		if s.billingCache != nil {
			_ = s.billingCache.InvalidateSubscription(ctx, key.UserID, key.GroupID)
		}
	}
	if result.UpdatedCount == 0 || result.StaleRemaining == 0 {
		break
	}
}
```

日志只写 subscription id、user id、group id 和数量，不写 API Key。

- [ ] **Step 5: 注册 Wire provider**

在 `backend/internal/service/wire.go` 加：

```go
func ProvideSubscriptionUsageWindowScheduler(userSubRepo UserSubscriptionRepository, billingCache *BillingCacheService, lockCache LeaderLockCache, db *sql.DB) *SubscriptionUsageWindowScheduler {
	svc := NewSubscriptionUsageWindowScheduler(userSubRepo, billingCache, subscriptionUsageWindowBatchSize)
	svc.SetLeaderLock(lockCache, db)
	svc.Start()
	return svc
}
```

并加入 `ProviderSet`。

- [ ] **Step 6: 注册 cleanup**

在 `backend/cmd/server/wire.go` 的 `provideCleanup` 参数中加入：

```go
subscriptionUsageWindow *service.SubscriptionUsageWindowScheduler,
```

在 `parallelSteps` 中加入：

```go
{"SubscriptionUsageWindowScheduler", func() error {
	if subscriptionUsageWindow != nil {
		subscriptionUsageWindow.Stop()
	}
	return nil
}},
```

- [ ] **Step 7: 重新生成 Wire**

Run:

```bash
go generate ./cmd/server
```

Expected: `backend/cmd/server/wire_gen.go` 更新并编译通过。

- [ ] **Step 8: Run tests**

Run:

```bash
go test -count=1 -tags=unit ./internal/service -run 'TestSubscriptionUsageWindowScheduler|TestTryAcquireSingletonLeaderLock'
go test -count=1 ./cmd/server
```

Expected: scheduler unit 和 server wire 测试通过。

---

### Task 6: 修 `/v1/usage` 自动 Key 与订阅窗口展示

**Files:**
- Modify: `backend/internal/server/middleware/effective_group_test.go`
- Modify: `backend/internal/server/middleware/effective_group.go`
- Modify: `backend/internal/handler/gateway_handler.go`
- Test: `backend/internal/handler/gateway_handler_usage_test.go`

- [ ] **Step 1: 写自动 Key policy 测试**

在 `backend/internal/server/middleware/effective_group_test.go` 新增：

```go
func TestDefaultAutomaticKeyEndpointPolicyAllowsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/usage", nil)

	platform, ok := DefaultAutomaticKeyEndpointPolicy(c)

	require.True(t, ok)
	require.Equal(t, service.PlatformOpenAI, platform)
}
```

- [ ] **Step 2: Run failing test**

Run:

```bash
go test -count=1 -tags=unit ./internal/server/middleware -run TestDefaultAutomaticKeyEndpointPolicyAllowsUsage
```

Expected: 当前失败，policy 不支持 `/v1/usage`。

- [ ] **Step 3: 修改 policy**

在 `DefaultAutomaticKeyEndpointPolicy` 的支持列表加入：

```go
strings.EqualFold(path, "/v1/usage")
```

保持 `/v1beta` 和 `/antigravity/` 拒绝逻辑优先。

- [ ] **Step 4: 给 usage 响应加窗口归一化 helper**

在 `backend/internal/handler/gateway_handler.go` 增加：

```go
func normalizedSubscriptionForUsageResponse(sub *service.UserSubscription, now time.Time) service.UserSubscription {
	cp := *sub
	today := timezone.StartOfDay(now)
	thisWeek := timezone.StartOfWeek(now)
	if cp.DailyWindowStart == nil || cp.DailyWindowStart.Before(today) {
		cp.DailyUsageUSD = 0
		cp.DailyWindowStart = nil
	}
	if cp.WeeklyWindowStart == nil || cp.WeeklyWindowStart.Before(thisWeek) {
		cp.WeeklyUsageUSD = 0
		cp.WeeklyWindowStart = nil
	}
	if cp.MonthlyWindowStart == nil || !cp.MonthlyWindowStart.Add(30*24*time.Hour).After(now) {
		cp.MonthlyUsageUSD = 0
		cp.MonthlyWindowStart = nil
	}
	return cp
}
```

在 `usageUnrestricted()` 中，拿到 subscription 后先执行：

```go
normalized := normalizedSubscriptionForUsageResponse(subscription, timezone.Now())
subscription = &normalized
```

再计算 remaining 和响应字段。

- [ ] **Step 5: 写 handler unit 测试**

创建或补充 `backend/internal/handler/gateway_handler_usage_test.go`：

```go
func TestNormalizedSubscriptionForUsageResponse_ZerosStaleDailyWindow(t *testing.T) {
	today := timezone.StartOfDay(timezone.Now())
	yesterday := today.Add(-24 * time.Hour)
	sub := &service.UserSubscription{
		DailyUsageUSD:      12.5,
		WeeklyUsageUSD:     13.5,
		MonthlyUsageUSD:    14.5,
		DailyWindowStart:   &yesterday,
		WeeklyWindowStart:  &today,
		MonthlyWindowStart: &today,
	}

	got := normalizedSubscriptionForUsageResponse(sub, today.Add(time.Hour))

	require.Zero(t, got.DailyUsageUSD)
	require.Nil(t, got.DailyWindowStart)
	require.InDelta(t, 13.5, got.WeeklyUsageUSD, 0.000001)
	require.InDelta(t, 14.5, got.MonthlyUsageUSD, 0.000001)
}
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test -count=1 -tags=unit ./internal/server/middleware -run 'TestDefaultAutomaticKeyEndpointPolicyAllowsUsage|TestResolveEffectiveGroupMiddleware'
go test -count=1 -tags=unit ./internal/handler -run 'TestNormalizedSubscriptionForUsageResponse'
```

Expected: 新增 tests 通过，已有 unsupported endpoint tests 仍通过。

---

### Task 7: 补充整体回归测试与缓存失效

**Files:**
- Modify: `backend/internal/service/subscription_usage_window_scheduler_test.go`
- Modify: `backend/internal/service/billing_cache_service_subscription_window_test.go`
- Modify: `backend/internal/repository/user_subscription_repo_integration_test.go`

- [ ] **Step 1: scheduler 测试覆盖缓存失效**

在 scheduler test 中增加 fake billing cache，或直接用 `BillingCacheService` 包装一个 stub cache，断言更新了 `(userID, groupID)` 的订阅缓存会被失效。

测试断言：

```go
require.Equal(t, []string{"2:3"}, cache.invalidated)
```

- [ ] **Step 2: repository 测试覆盖 batch size**

新增两个 stale active 订阅，调用：

```go
result, err := s.repo.CalibrateActiveDailyUsageWindows(s.ctx, today, now, now, 1)
```

断言：

```go
s.Require().NoError(err)
s.Require().Equal(int64(1), result.UpdatedCount)
s.Require().Equal(int64(1), result.StaleRemaining)
```

- [ ] **Step 3: billing cache 测试覆盖 stale 准入不写库**

确认窗口测试中所有 stale 场景都断言：

```go
require.Zero(t, repo.refreshCalls)
```

- [ ] **Step 4: Run focused test set**

Run:

```bash
go test -count=1 -tags=unit ./internal/service -run 'TestCheckBillingEligibility_.*Window|TestSubscriptionUsageWindowScheduler'
go test -count=1 -tags=integration ./internal/repository -run 'TestUserSubscriptionRepoSuite/TestCalibrateActiveDailyUsageWindows|TestUsageBillingRepositoryApply_SubscriptionBilling'
```

Expected: focused tests 全部通过。

---

### Task 8: 运行全量相关验证

**Files:**
- No source edits in this task.

- [ ] **Step 1: 后端核心 unit**

Run:

```bash
go test -count=1 -tags=unit ./internal/service ./internal/server/middleware ./internal/handler
```

Expected: 退出码 0。

- [ ] **Step 2: 仓储 integration**

Run:

```bash
go test -count=1 -tags=integration ./internal/repository
```

Expected: 退出码 0。如果本机没有 integration DB，记录实际错误，不把它写成通过。

- [ ] **Step 3: server 编译与 wire**

Run:

```bash
go test -count=1 ./cmd/server
```

Expected: 退出码 0。

- [ ] **Step 4: 前端类型检查**

只有 Task 6 修改了前端相关响应契约但未改前端代码时，也跑一次：

```bash
cd frontend && pnpm typecheck
```

Expected: 退出码 0。

- [ ] **Step 5: diff 检查**

Run:

```bash
git diff --check
git status --short
```

Expected: `git diff --check` 退出码 0；`git status --short` 中只包含本计划相关实现、测试和文档，以及实施前已经存在的无关改动。

---

### Task 9: 上线前运行态验收脚本

**Files:**
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-east8-subscription-auto-refresh-result_CN.md`

- [ ] **Step 1: 备份公网数据**

Run on deployment host:

```bash
docker exec sub2api-candidate-postgres pg_dump -U postgres -Fc sub2api > deploy/backups/$(date +%Y%m%d-%H%M%S)-sub2api-candidate-before-east8-window-scheduler.dump
docker exec sub2api-candidate-redis redis-cli SAVE
```

Expected: Postgres dump 文件存在且非 0 字节；Redis `SAVE` 返回 `OK`。

- [ ] **Step 2: 发布应用容器**

沿用当前项目发布习惯：只替换 `sub2api-candidate` 应用容器，保留 `sub2api-candidate-postgres`、`sub2api-candidate-redis`、nginx 和 Cloudflare Tunnel。

- [ ] **Step 3: 验证 stale 数量**

Run:

```sql
SELECT COUNT(*)
FROM user_subscriptions
WHERE deleted_at IS NULL
  AND status = 'active'
  AND expires_at > NOW()
  AND (daily_window_start IS NULL OR daily_window_start < date_trunc('day', NOW() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai');
```

Expected: 后台任务运行后为 `0`。如果刚发布时间不在午夜附近，启动补偿扫描也应把 stale 推进为 `0`。

- [ ] **Step 4: 验证 `/v1/usage` 自动 Key**

使用一个自动 Key 请求：

```bash
curl -sS https://api.aaccx.pw/v1/usage \
  -H 'Authorization: Bearer sk-***'
```

Expected: HTTP 200；响应包含 `subscription` 或余额模式字段；自动 Key 不再返回 `AUTO_KEY_UNSUPPORTED_ENDPOINT`。

- [ ] **Step 5: 验证展示不混昨天**

对一个历史 stale 用户查询：

```sql
SELECT us.id, us.daily_window_start, us.daily_usage_usd,
       COALESCE(SUM(ul.total_cost), 0) AS today_usage_logs
FROM user_subscriptions us
LEFT JOIN usage_logs ul
  ON ul.subscription_id = us.id
 AND ul.created_at >= date_trunc('day', NOW() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai'
WHERE us.deleted_at IS NULL
  AND us.status = 'active'
GROUP BY us.id;
```

Expected: `daily_usage_usd` 与今天日志聚合一致，或今天窗口内因并发完成时扣费只大于等于查询瞬间已聚合结果；不得出现昨天窗口用量展示为今天。

- [ ] **Step 6: 写结果文档**

在 `docs/ai/context/` 新增结果文档，记录：

```text
镜像 tag：
DB migration 数：
发布前备份：
health 检查：
stale active subscription 数：
/v1/usage 自动 Key 结果：
未完成项：
```

不要写完整 API Key、内部 token、HMAC secret、SMTP 密码。

---

## 自检清单

- 设计覆盖：删除 API 入口写库刷新由 Task 4 覆盖；后台任务由 Task 3/5 覆盖；完成时记账由 Task 2 覆盖；`/v1/usage` 自动 Key 与展示由 Task 6 覆盖；重试/锁/监控由 Task 5/9 覆盖。
- 并发边界：后台任务只更新 stale 行；完成时扣费先推进到今天时，后台任务不覆盖今天行。
- 时间口径：生产扣费使用 `usageLog.CreatedAt -> UsageBillingCommand.CompletedAt`；后台校准使用 `usage_logs.created_at` 聚合。
- 敏感信息：计划和验收只允许脱敏 API Key。
- 实施方式：默认不提交 git；只有用户明确要求提交时才执行提交动作。
