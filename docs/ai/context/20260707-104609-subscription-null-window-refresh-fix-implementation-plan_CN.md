# 订阅 NULL 窗口用量残留修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复订阅 `NULL` 窗口下历史 usage 残留的问题，让 API 计费写路径、订阅列表读路径和 progress 返回保持一致。

**Architecture:** 在仓储层用单条原子 `UPDATE` 统一刷新 `NULL` 和过期窗口；在 service 层抽出只影响返回数据的展示归一化函数，列表和 progress 复用。运行态历史数据不放进请求热路径聚合，发布前后单独用只读检查和必要校准处理。

**Tech Stack:** Go、Ent、PostgreSQL、sqlmock、testify、项目现有 `usagewindow` 窗口策略。

---

## 文件结构

- Modify: `backend/internal/repository/user_subscription_repo.go`
  - 负责订阅窗口刷新 SQL。只改 `RefreshExpiredUsageWindows` 的 `CASE` 条件，不改变方法签名。
- Modify: `backend/internal/repository/user_subscription_repo_integration_test.go`
  - 增加真实数据库集成测试，证明 `NULL + usage > 0` 会被清零，当前窗口内 usage 不被误清。
- Modify: `backend/internal/service/subscription_service.go`
  - 抽出 `normalizeExpiredWindowForDisplay(sub *UserSubscription, now time.Time)`，让列表和 progress 共享读模型归一化。
- Create: `backend/internal/service/subscription_window_display_test.go`
  - 增加 service 层读路径单元测试，不依赖数据库。
- Modify: `backend/internal/service/billing_cache_service_subscription_window_test.go`
  - 修正测试 stub 的 `RefreshExpiredUsageWindows` 行为，使它和仓储层真实语义一致。
- Create during implementation: `docs/ai/context/${TS}-subscription-null-window-refresh-fix-result_CN.md`
  - 记录实际改动、验证命令、运行态校准检查结果。
- Modify during implementation: `AGENTS.md`
  - 追加本次修复结果的长期记忆。

## Task 1: 写仓储层失败测试

**Files:**
- Modify: `backend/internal/repository/user_subscription_repo_integration_test.go`

- [ ] **Step 1: 在 `TestResetMonthlyUsage` 后追加 NULL 窗口回归测试**

```go
func (s *UserSubscriptionRepoSuite) TestRefreshExpiredUsageWindows_NullWindowsResetUsage() {
	user := s.mustCreateUser("refresh-null-windows@test.com", service.RoleUser)
	group := s.mustCreateGroup("g-refresh-null-windows")
	sub := s.mustCreateSubscription(user.ID, group.ID, func(c *dbent.UserSubscriptionCreate) {
		c.SetDailyUsageUsd(9.9)
		c.SetWeeklyUsageUsd(19.9)
		c.SetMonthlyUsageUsd(29.9)
	})

	dailyStart := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)
	weeklyStart := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	monthlyStart := time.Date(2026, 7, 7, 10, 30, 0, 0, time.UTC)
	now := monthlyStart

	updated, err := s.repo.RefreshExpiredUsageWindows(s.ctx, sub.ID, dailyStart, weeklyStart, monthlyStart, now)
	s.Require().NoError(err)
	s.Require().True(updated)

	got, err := s.repo.GetByID(s.ctx, sub.ID)
	s.Require().NoError(err)
	s.Require().InDelta(0.0, got.DailyUsageUSD, 1e-6)
	s.Require().InDelta(0.0, got.WeeklyUsageUSD, 1e-6)
	s.Require().InDelta(0.0, got.MonthlyUsageUSD, 1e-6)
	s.Require().NotNil(got.DailyWindowStart)
	s.Require().NotNil(got.WeeklyWindowStart)
	s.Require().NotNil(got.MonthlyWindowStart)
	s.Require().WithinDuration(dailyStart, *got.DailyWindowStart, time.Microsecond)
	s.Require().WithinDuration(weeklyStart, *got.WeeklyWindowStart, time.Microsecond)
	s.Require().WithinDuration(monthlyStart, *got.MonthlyWindowStart, time.Microsecond)
}
```

- [ ] **Step 2: 在同一文件追加当前窗口不清零测试**

```go
func (s *UserSubscriptionRepoSuite) TestRefreshExpiredUsageWindows_CurrentWindowsKeepUsage() {
	user := s.mustCreateUser("refresh-current-windows@test.com", service.RoleUser)
	group := s.mustCreateGroup("g-refresh-current-windows")
	dailyStart := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)
	weeklyStart := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	monthlyStart := time.Date(2026, 7, 7, 10, 30, 0, 0, time.UTC)
	now := monthlyStart.Add(2 * time.Hour)
	sub := s.mustCreateSubscription(user.ID, group.ID, func(c *dbent.UserSubscriptionCreate) {
		c.SetDailyWindowStart(dailyStart)
		c.SetWeeklyWindowStart(weeklyStart)
		c.SetMonthlyWindowStart(monthlyStart)
		c.SetDailyUsageUsd(1.1)
		c.SetWeeklyUsageUsd(2.2)
		c.SetMonthlyUsageUsd(3.3)
	})

	updated, err := s.repo.RefreshExpiredUsageWindows(s.ctx, sub.ID, dailyStart, weeklyStart, monthlyStart, now)
	s.Require().NoError(err)
	s.Require().False(updated)

	got, err := s.repo.GetByID(s.ctx, sub.ID)
	s.Require().NoError(err)
	s.Require().InDelta(1.1, got.DailyUsageUSD, 1e-6)
	s.Require().InDelta(2.2, got.WeeklyUsageUSD, 1e-6)
	s.Require().InDelta(3.3, got.MonthlyUsageUSD, 1e-6)
}
```

- [ ] **Step 3: 运行新仓储测试并确认旧实现失败**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/backend
go test -count=1 -tags=integration ./internal/repository -run 'TestUserSubscriptionRepoSuite/TestRefreshExpiredUsageWindows_(NullWindowsResetUsage|CurrentWindowsKeepUsage)'
```

Expected:

```text
FAIL
DailyUsageUSD remains 9.9 in TestRefreshExpiredUsageWindows_NullWindowsResetUsage
```

旧实现下第二个测试应通过，第一个测试应失败。

## Task 2: 写 service 读路径失败测试

**Files:**
- Create: `backend/internal/service/subscription_window_display_test.go`

- [ ] **Step 1: 创建 service 展示归一化测试文件**

```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeExpiredWindows_NullWindowsWithUsageAreHidden(t *testing.T) {
	subs := []UserSubscription{{
		ID:              1,
		DailyUsageUSD:   9.9,
		WeeklyUsageUSD:  19.9,
		MonthlyUsageUSD: 29.9,
	}}

	normalizeExpiredWindows(subs)

	require.InDelta(t, 0.0, subs[0].DailyUsageUSD, 1e-9)
	require.InDelta(t, 0.0, subs[0].WeeklyUsageUSD, 1e-9)
	require.InDelta(t, 0.0, subs[0].MonthlyUsageUSD, 1e-9)
	require.Nil(t, subs[0].DailyWindowStart)
	require.Nil(t, subs[0].WeeklyWindowStart)
	require.Nil(t, subs[0].MonthlyWindowStart)
}

func TestNormalizeExpiredWindows_CurrentWindowsKeepUsage(t *testing.T) {
	now := time.Now()
	subs := []UserSubscription{{
		ID:                 2,
		DailyWindowStart:   ptrTime(now),
		WeeklyWindowStart:  ptrTime(now),
		MonthlyWindowStart: ptrTime(now),
		DailyUsageUSD:      1.1,
		WeeklyUsageUSD:     2.2,
		MonthlyUsageUSD:    3.3,
		StartsAt:           now.Add(-time.Hour),
		ExpiresAt:          now.Add(24 * time.Hour),
	}}

	normalizeExpiredWindows(subs)

	require.InDelta(t, 1.1, subs[0].DailyUsageUSD, 1e-9)
	require.InDelta(t, 2.2, subs[0].WeeklyUsageUSD, 1e-9)
	require.InDelta(t, 3.3, subs[0].MonthlyUsageUSD, 1e-9)
}

type subscriptionProgressWindowRepoStub struct {
	userSubRepoNoop
	sub *UserSubscription
}

func (r *subscriptionProgressWindowRepoStub) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	if r.sub == nil || r.sub.ID != id {
		return nil, ErrSubscriptionNotFound
	}
	cp := *r.sub
	return &cp, nil
}

func TestGetSubscriptionProgress_NormalizesExpiredWindowBeforeProgress(t *testing.T) {
	pastStart := time.Now().Add(-48 * time.Hour)
	limit := 10.0
	repo := &subscriptionProgressWindowRepoStub{sub: &UserSubscription{
		ID:               3,
		GroupID:          4,
		StartsAt:         time.Now().Add(-72 * time.Hour),
		ExpiresAt:        time.Now().Add(24 * time.Hour),
		Status:           SubscriptionStatusActive,
		DailyWindowStart: ptrTime(pastStart),
		DailyUsageUSD:    9.9,
	}}
	group := &Group{ID: 4, Name: "Pro", DailyLimitUSD: &limit}
	svc := NewSubscriptionService(&subscriptionGroupRepoStub{group: group}, repo, nil, nil, nil)
	t.Cleanup(svc.Stop)

	progress, err := svc.GetSubscriptionProgress(context.Background(), 3)

	require.NoError(t, err)
	require.Nil(t, progress.Daily)
}
```

- [ ] **Step 2: 运行新 service 测试并确认旧实现失败**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/backend
go test -count=1 ./internal/service -run 'TestNormalizeExpiredWindows_|TestGetSubscriptionProgress_NormalizesExpiredWindowBeforeProgress'
```

Expected:

```text
FAIL
TestNormalizeExpiredWindows_NullWindowsWithUsageAreHidden keeps usage values
TestGetSubscriptionProgress_NormalizesExpiredWindowBeforeProgress returns non-nil Daily
```

## Task 3: 修仓储层真实刷新语义

**Files:**
- Modify: `backend/internal/repository/user_subscription_repo.go:344-366`
- Modify: `backend/internal/service/billing_cache_service_subscription_window_test.go:43-64`

- [ ] **Step 1: 修改 `RefreshExpiredUsageWindows` SQL 的三个 usage CASE**

Replace the existing `daily_usage_usd` / `weekly_usage_usd` / `monthly_usage_usd` CASE blocks with:

```go
				daily_usage_usd = CASE
					WHEN daily_window_start IS NULL OR daily_window_start < $2 THEN 0
					ELSE daily_usage_usd
				END,
				daily_window_start = CASE
					WHEN daily_window_start IS NULL OR daily_window_start < $2 THEN $2
					ELSE daily_window_start
				END,
				weekly_usage_usd = CASE
					WHEN weekly_window_start IS NULL OR weekly_window_start < $3 THEN 0
					ELSE weekly_usage_usd
				END,
				weekly_window_start = CASE
					WHEN weekly_window_start IS NULL OR weekly_window_start < $3 THEN $3
					ELSE weekly_window_start
				END,
				monthly_usage_usd = CASE
					WHEN monthly_window_start IS NULL OR monthly_window_start + INTERVAL '30 days' <= $5 THEN 0
					ELSE monthly_usage_usd
				END,
				monthly_window_start = CASE
					WHEN monthly_window_start IS NULL OR monthly_window_start + INTERVAL '30 days' <= $5 THEN $4
					ELSE monthly_window_start
				END,
```

- [ ] **Step 2: 同步修正 service 测试 stub**

In `backend/internal/service/billing_cache_service_subscription_window_test.go`, replace the three window blocks inside `RefreshExpiredUsageWindows` with:

```go
	if r.sub.DailyWindowStart == nil || r.sub.DailyWindowStart.Before(dailyStart) {
		r.sub.DailyUsageUSD = 0
		r.sub.DailyWindowStart = timePtrForWindowTest(dailyStart)
		changed = true
	}
	if r.sub.WeeklyWindowStart == nil || r.sub.WeeklyWindowStart.Before(weeklyStart) {
		r.sub.WeeklyUsageUSD = 0
		r.sub.WeeklyWindowStart = timePtrForWindowTest(weeklyStart)
		changed = true
	}
	if r.sub.MonthlyWindowStart == nil || !r.sub.MonthlyWindowStart.Add(30*24*time.Hour).After(now) {
		r.sub.MonthlyUsageUSD = 0
		r.sub.MonthlyWindowStart = timePtrForWindowTest(monthlyStart)
		changed = true
	}
```

- [ ] **Step 3: 跑仓储新测试确认通过**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/backend
go test -count=1 -tags=integration ./internal/repository -run 'TestUserSubscriptionRepoSuite/TestRefreshExpiredUsageWindows_(NullWindowsResetUsage|CurrentWindowsKeepUsage)'
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/repository
```

## Task 4: 修 service 读路径展示语义

**Files:**
- Modify: `backend/internal/service/subscription_service.go:703-724`
- Modify: `backend/internal/service/subscription_service.go:973-987`

- [ ] **Step 1: 抽出单条订阅展示归一化函数**

Replace `normalizeExpiredWindows` with:

```go
func normalizeExpiredWindows(subs []UserSubscription) {
	now := time.Now()
	for i := range subs {
		normalizeExpiredWindowForDisplay(&subs[i], now)
	}
}

func normalizeExpiredWindowForDisplay(sub *UserSubscription, now time.Time) {
	if sub == nil {
		return
	}
	if sub.DailyWindowStart == nil {
		if sub.DailyUsageUSD > 0 {
			sub.DailyUsageUSD = 0
		}
	} else if sub.NeedsDailyResetAt(now) {
		sub.DailyWindowStart = nil
		sub.DailyUsageUSD = 0
	}
	if sub.WeeklyWindowStart == nil {
		if sub.WeeklyUsageUSD > 0 {
			sub.WeeklyUsageUSD = 0
		}
	} else if usagewindow.WeeklyExpired(sub.WeeklyWindowStart, now) {
		sub.WeeklyWindowStart = nil
		sub.WeeklyUsageUSD = 0
	}
	if sub.MonthlyWindowStart == nil {
		if sub.MonthlyUsageUSD > 0 {
			sub.MonthlyUsageUSD = 0
		}
	} else if usagewindow.MonthlyExpired(sub.MonthlyWindowStart, now) {
		sub.MonthlyWindowStart = nil
		sub.MonthlyUsageUSD = 0
	}
}
```

- [ ] **Step 2: 在 `GetSubscriptionProgress` 计算前归一化单条订阅**

Change:

```go
	return s.calculateProgress(sub, group), nil
```

to:

```go
	normalizeExpiredWindowForDisplay(sub, time.Now())
	return s.calculateProgress(sub, group), nil
```

- [ ] **Step 3: 跑 service 新测试确认通过**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/backend
go test -count=1 ./internal/service -run 'TestNormalizeExpiredWindows_|TestGetSubscriptionProgress_NormalizesExpiredWindowBeforeProgress'
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service
```

## Task 5: 全量相关验证

**Files:**
- Validate: `backend/internal/repository`
- Validate: `backend/internal/service`

- [ ] **Step 1: 格式化 Go 文件**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/backend
gofmt -w \
  internal/repository/user_subscription_repo.go \
  internal/repository/user_subscription_repo_integration_test.go \
  internal/service/subscription_service.go \
  internal/service/subscription_window_display_test.go \
  internal/service/billing_cache_service_subscription_window_test.go
```

Expected: command exits with code 0.

- [ ] **Step 2: 跑 unit 测试**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/backend
go test -count=1 -tags=unit ./internal/repository ./internal/service
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/repository
ok  	github.com/Wei-Shaw/sub2api/internal/service
```

- [ ] **Step 3: 跑新增仓储集成测试**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/backend
go test -count=1 -tags=integration ./internal/repository -run 'TestUserSubscriptionRepoSuite/TestRefreshExpiredUsageWindows_(NullWindowsResetUsage|CurrentWindowsKeepUsage)'
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/repository
```

- [ ] **Step 4: 检查补丁格式**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
git diff --check
```

Expected: command exits with code 0 and prints no whitespace errors.

## Task 6: 归档结果与运行态校准检查

**Files:**
- Create during implementation: `docs/ai/context/${TS}-subscription-null-window-refresh-fix-result_CN.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: 生成结果文档时间戳**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
TS=$(date '+%Y%m%d-%H%M%S')
RESULT_DOC="docs/ai/context/${TS}-subscription-null-window-refresh-fix-result_CN.md"
printf '%s\n' "$RESULT_DOC"
```

Expected: prints a path under `docs/ai/context/`.

- [ ] **Step 2: 创建结果文档**

Create `$RESULT_DOC` with this structure and fill it from actual command output:

```markdown
# 订阅 NULL 窗口用量残留修复结果

## 改动

- `RefreshExpiredUsageWindows` 已让 daily/weekly/monthly 的 `NULL` 窗口初始化同步清零 usage。
- 订阅列表和 progress 已复用展示归一化，`NULL + usage > 0` 不再返回历史 usage。
- service 测试 stub 已与真实仓储语义保持一致。

## 验证

- `go test -count=1 -tags=unit ./internal/repository ./internal/service`：通过
- `go test -count=1 -tags=integration ./internal/repository -run 'TestUserSubscriptionRepoSuite/TestRefreshExpiredUsageWindows_(NullWindowsResetUsage|CurrentWindowsKeepUsage)'`：通过
- `git diff --check`：通过

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
```

- [ ] **Step 3: 更新 `AGENTS.md`**

Add a new bullet near the existing 2026-07-07 subscription window entries:

```markdown
- 2026-07-07 已修复订阅 NULL 窗口用量残留：`RefreshExpiredUsageWindows` 对 daily/weekly/monthly 的 `NULL` 窗口初始化会同步清零 usage，订阅列表和 progress 读路径也会隐藏 `NULL + usage > 0` 的历史值；相关 unit 与新增仓储 integration 测试通过。结果见 `docs/ai/context/${TS}-subscription-null-window-refresh-fix-result_CN.md`。
```

- [ ] **Step 4: 检查文档与格式**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
UNFINISHED_PATTERN='TB'"D"'|待'"补"'|占'"位"'|未'"完成"
rg -n "$UNFINISHED_PATTERN" "$RESULT_DOC" AGENTS.md
git diff --check
```

Expected:

```text
rg exits with code 1 because no matching unfinished marker is found
git diff --check exits with code 0
```

## Plan Self-Review

- Spec coverage: 写路径、读路径、progress、测试、运行态校准检查都有对应任务。
- 未完成项扫描: 计划中没有留空章节；执行时间相关文件名通过 `$TS` 命令生成。
- Type consistency: 使用现有 `UserSubscription`、`Group`、`RefreshExpiredUsageWindows`、`normalizeExpiredWindows`、`calculateProgress`、`usagewindow` 名称，和当前代码一致。
