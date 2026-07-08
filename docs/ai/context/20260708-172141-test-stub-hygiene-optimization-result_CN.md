# 后端测试 stub 卫生优化结果

> 2026-07-08 17:21 JST

## 结果

已在本地 `main` 分支完成后端测试 stub 卫生优化。

## 改动

- `backend/Makefile`
  - 新增 `test-server-unit` phony target。
  - 固定执行 `go test -count=1 -tags=unit ./internal/server`，避免后续验证 `api_contract_test.go` 时漏掉 `unit` build tag 或被缓存掩盖。
- `backend/internal/server/api_contract_test.go`
  - 新增 `TestStubUserSubscriptionRepoRejectsUnexpectedDailyWindowCalibrationCalls`，明确约束日窗口校准方法在 API contract stub 中必须失败优先。
  - 将 `stubUserSubscriptionRepo.CalibrateActiveDailyUsageWindows` 从空成功改为 `nil, errors.New("not implemented")`。
  - 将 `stubUserSubscriptionRepo.CountStaleActiveDailyWindows` 从 `0, nil` 改为 `0, errors.New("not implemented")`。

## TDD 记录

- 红灯 1：`make -C backend test-server-unit`
  - 结果：失败，`No rule to make target 'test-server-unit'`。
- 绿灯 1：新增 `backend/Makefile` target 后重跑 `make -C backend test-server-unit`
  - 结果：通过，`ok github.com/Wei-Shaw/sub2api/internal/server`。
- 红灯 2：新增 `TestStubUserSubscriptionRepoRejectsUnexpectedDailyWindowCalibrationCalls` 后运行：

```bash
go test -count=1 -tags=unit ./internal/server -run TestStubUserSubscriptionRepoRejectsUnexpectedDailyWindowCalibrationCalls
```

  - 结果：失败，当前 stub 返回 `&service.SubscriptionDailyWindowCalibrationResult{...}` 而不是 `nil`。
- 绿灯 2：改 stub 返回 `not implemented` 后重跑同一测试。
  - 结果：通过。

## 验证

在 `backend/` 下运行：

```bash
go test -count=1 -tags=unit ./internal/server -run TestStubUserSubscriptionRepoRejectsUnexpectedDailyWindowCalibrationCalls
go test -count=1 -tags=unit ./...
go test -count=1 ./...
```

均通过。

在仓库根目录运行：

```bash
make -C backend test-server-unit
git diff --check
```

均通过。

## 公网影响

未触碰 18084 公网服务：

- 未构建镜像。
- 未重启或替换 Docker 容器。
- 未修改 nginx。
- 未访问或修改公网 PostgreSQL、Redis。
- 未部署公网。
- 未提交 git。
