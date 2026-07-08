# 后端测试 stub 卫生优化实施计划

> 2026-07-08 17:15 JST

**目标：** 在本地 `main` 分支优化后端测试入口与 `api_contract_test.go` stub 行为，不影响 18084 公网服务。

**架构：** 保持现有测试结构，只新增一个 Makefile 入口并收紧一个测试 stub 的默认行为。Make target 解决 unit build tag 命令口径；stub 返回 `not implemented` 解决误调用被伪装成成功的问题。

**技术栈：** Go test、Makefile、Go unit build tag。

---

## 保护边界

- 不执行 Docker 构建、重启、替换容器。
- 不修改 nginx 配置。
- 不访问或修改公网数据库、Redis、18084 应用容器。
- 不提交 git，除非用户明确要求。

## 任务 1：新增 server unit Make 入口

**文件：**

- 修改：`backend/Makefile`

- [ ] **Step 1：红灯验证目标不存在**

运行：

```bash
make -C backend test-server-unit
```

预期：失败，提示没有 `test-server-unit` 规则。

- [ ] **Step 2：新增 Make target**

把 `.PHONY` 从：

```makefile
.PHONY: build generate test test-unit test-integration test-e2e
```

改为：

```makefile
.PHONY: build generate test test-unit test-server-unit test-integration test-e2e
```

在 `test-unit` 后新增：

```makefile
test-server-unit:
	go test -count=1 -tags=unit ./internal/server
```

- [ ] **Step 3：绿灯验证目标可用**

运行：

```bash
make -C backend test-server-unit
```

预期：通过，输出包含 `ok github.com/Wei-Shaw/sub2api/internal/server`。

## 任务 2：让 api contract stub 失败优先

**文件：**

- 修改：`backend/internal/server/api_contract_test.go`

- [ ] **Step 1：新增红灯测试**

新增测试函数：

```go
func TestStubUserSubscriptionRepoRejectsUnexpectedDailyWindowCalibrationCalls(t *testing.T) {
	repo := stubUserSubscriptionRepo{}
	now := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

	result, err := repo.CalibrateActiveDailyUsageWindows(context.Background(), now, now, now, 100)
	require.Nil(t, result)
	require.ErrorContains(t, err, "not implemented")

	stale, err := repo.CountStaleActiveDailyWindows(context.Background(), now, now)
	require.Zero(t, stale)
	require.ErrorContains(t, err, "not implemented")
}
```

- [ ] **Step 2：验证红灯**

运行：

```bash
go test -count=1 -tags=unit ./internal/server -run TestStubUserSubscriptionRepoRejectsUnexpectedDailyWindowCalibrationCalls
```

预期：失败，因为当前 `CalibrateActiveDailyUsageWindows` 返回空结果和 `nil`。

- [ ] **Step 3：修改 stub 实现**

把两个方法改为：

```go
func (stubUserSubscriptionRepo) CalibrateActiveDailyUsageWindows(ctx context.Context, dailyStart, upperBound, now time.Time, batchSize int) (*service.SubscriptionDailyWindowCalibrationResult, error) {
	return nil, errors.New("not implemented")
}

func (stubUserSubscriptionRepo) CountStaleActiveDailyWindows(ctx context.Context, windowStart, now time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}
```

- [ ] **Step 4：验证绿灯**

运行：

```bash
go test -count=1 -tags=unit ./internal/server -run TestStubUserSubscriptionRepoRejectsUnexpectedDailyWindowCalibrationCalls
make -C backend test-server-unit
```

预期：两个命令都通过。

## 任务 3：回归与记录

**文件：**

- 新增：`docs/ai/context/YYYYMMDD-HHMMSS-test-stub-hygiene-optimization-result_CN.md`
- 修改：`AGENTS.md`

- [ ] **Step 1：运行回归**

运行：

```bash
go test -count=1 -tags=unit ./...
git diff --check
```

预期：全部通过。

如时间允许补跑：

```bash
go test -count=1 ./...
```

预期：默认测试集通过。

- [ ] **Step 2：写结果文档**

记录：

- 改动文件。
- 红绿测试结果。
- 明确未触碰 18084、公网容器、nginx、数据库、Redis。
- 未提交 git。

- [ ] **Step 3：更新 AGENTS.md**

在最高优先级定论顶部追加一条本地结果记忆，指向结果文档。
