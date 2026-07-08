# 后端测试 stub 缺失方法修复深度核查

> 2026-07-08 12:12 JST

## 核查对象

- 历史文档：`docs/ai/context/20260708-104500-test-stub-missing-methods-fix_CN.md`
- 目标问题：`service.UserSubscriptionRepository` 新增 `CalibrateActiveDailyUsageWindows` 与 `CountStaleActiveDailyWindows` 后，测试 stub 未同步导致 unit 编译失败。

## 结论

- 编译阻塞已解决：`backend/internal/server/api_contract_test.go` 的 `stubUserSubscriptionRepo` 已补齐两个方法，`service.UserSubscriptionRepository` 的编译期断言已通过。
- 未发现其它构建标签路径还有同类“接口缺方法”遗漏：unit、默认测试集、integration 编译门、e2e 编译门均通过。
- 不能称为“完美”：原文档把触发命令写成 `go test ./internal/server`，但该文件带 `//go:build unit`，当前 `GOFLAGS` 为空时实际触发命令应是 `go test -tags=unit ./internal/server` 或包含 unit tag 的上层命令。
- 另一个测试卫生残留：`api_contract_test.go` 新增两个 stub 方法返回空结果与 `nil` 错误；同类 middleware stub 返回 `not implemented`。当前测试不会调用这两个方法，所以不影响通过，但未来如果 API contract 测试路径误调用后台校准能力，空成功值可能掩盖未建模依赖。

## 代码核查

- 接口定义位于 `backend/internal/service/user_subscription_port.go`，当前包含：
  - `CalibrateActiveDailyUsageWindows(ctx, dailyStart, upperBound, now, batchSize)`
  - `CountStaleActiveDailyWindows(ctx, dailyStart, now)`
- 真实 repository 实现位于 `backend/internal/repository/user_subscription_repo.go`，两个方法均存在。
- `rg` 检索到的测试实现均已补齐两个方法：
  - `backend/internal/server/api_contract_test.go`
  - `backend/internal/server/middleware/api_key_auth_test.go`
  - `backend/internal/server/middleware/api_key_auth_google_test.go`
  - `backend/internal/service/subscription_expiry_service_test.go`
  - `backend/internal/service/subscription_assign_idempotency_test.go`
  - `backend/internal/service/subscription_usage_window_scheduler_test.go`
- `api_contract_test.go` 已保留编译期断言：
  - `_ service.UserSubscriptionRepository = (*stubUserSubscriptionRepo)(nil)`

## 验证命令

在 `backend/` 下新跑：

```bash
go test -count=1 -tags=unit ./internal/server
go test -count=1 ./internal/server
go test -count=1 -tags=unit ./...
go test -run '^$' -count=1 -tags=integration ./...
go test -run '^$' -count=1 -tags=e2e ./internal/integration
go test -count=1 ./...
```

结果均为退出码 0。

## 建议

- 若追求测试桩严格性，建议把 `api_contract_test.go` 新增两个方法改成返回 `errors.New("not implemented")`，与同文件大多数未使用 stub 方法以及 middleware stub 保持一致。
- 后续文档记录带 build tag 的测试失败时，命令必须写全；若依赖 `GOFLAGS=-tags=unit`，也要显式记录环境。
