# 后端测试 stub 缺失方法修复

> 2026-07-08 10:44 JST

## 问题描述

构建本地 `main` HEAD `83cf82584` 时，`go test ./internal/server` 编译失败：

```
internal/server/api_contract_test.go:1281:77: cannot use userSubRepo
(*stubUserSubscriptionRepo) as service.UserSubscriptionRepository value:
  *stubUserSubscriptionRepo does not implement service.UserSubscriptionRepository
```

根因：`service.UserSubscriptionRepository` 接口新增了两个方法，但 `internal/server/api_contract_test.go` 中的 `stubUserSubscriptionRepo` 未同步实现。

## 缺失的方法

| 方法签名 | 说明 |
|----------|------|
| `CalibrateActiveDailyUsageWindows(ctx, dailyStart, upperBound, now, batchSize) (*SubscriptionDailyWindowCalibrationResult, error)` | 东八区订阅用量窗口校准（来自 `feat: add east8 subscription usage refresh`） |
| `CountStaleActiveDailyWindows(ctx, windowStart, now) (int64, error)` | 统计 stale 日窗口数 |

## 修复

在 `backend/internal/server/api_contract_test.go` 的 `stubUserSubscriptionRepo` 末尾（`BatchUpdateExpiredStatus` 方法之后）添加：

```go
func (stubUserSubscriptionRepo) CalibrateActiveDailyUsageWindows(ctx context.Context, dailyStart, upperBound, now time.Time, batchSize int) (*service.SubscriptionDailyWindowCalibrationResult, error) {
	return &service.SubscriptionDailyWindowCalibrationResult{}, nil
}
func (stubUserSubscriptionRepo) CountStaleActiveDailyWindows(ctx context.Context, windowStart, now time.Time) (int64, error) {
	return 0, nil
}
```

## 验证

修复后所有测试通过：

```
ok  github.com/Wei-Shaw/sub2api/internal/payment    0.377s
ok  github.com/Wei-Shaw/sub2api/internal/service    88.817s
ok  github.com/Wei-Shaw/sub2api/internal/handler    23.236s
ok  github.com/Wei-Shaw/sub2api/internal/server     0.287s
ok  github.com/Wei-Shaw/sub2api/migrations          0.279s
ok  github.com/Wei-Shaw/sub2api/cmd/server           0.587s
```

## 教训

新增 repository interface 方法时，必须同时检查所有实现了该 interface 的 test stub，确保同步添加 stub 方法，避免编译失败阻塞 CI/CD 和发布流水线。
