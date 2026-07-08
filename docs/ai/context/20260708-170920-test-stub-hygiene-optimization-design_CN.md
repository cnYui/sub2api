# 后端测试 stub 卫生优化设计

> 2026-07-08 17:09 JST

## 背景

`docs/ai/context/20260708-121254-test-stub-missing-methods-deep-audit_CN.md` 已确认：

- `service.UserSubscriptionRepository` 新增方法导致的 `server` unit stub 编译阻塞已解决。
- `unit`、默认测试集、`integration` 编译门、`e2e` 编译门均通过，未发现其它 stub 缺方法。
- 仍有两个残留问题：
  - 历史文档中的复现命令写成 `go test ./internal/server`，但目标测试文件是 `//go:build unit`，正确入口应带 `-tags=unit`。
  - `backend/internal/server/api_contract_test.go` 新增的两个 stub 方法返回空结果和 `nil`，未来若误调用，可能把未建模依赖伪装成成功。

## 目标

- 给 `server` unit 测试提供明确、可复用的 Make 入口，减少后续文档和 CI 手工命令漏写 `-tags=unit` 的概率。
- 让 `api_contract_test.go` 中未被当前测试路径使用的订阅日窗口校准 stub 失败优先，误调用时立刻暴露问题。
- 保留历史文档，不覆写、重命名或删除既有 `docs/ai/context/` 文件。

## 非目标

- 不重构 `UserSubscriptionRepository` 接口。
- 不抽取通用 mock 框架。
- 不改 scheduler、repository、billing cache 或业务逻辑。
- 不修改已存在历史文档内容，只通过新文档和 `AGENTS.md` 追加记忆纠偏。

## 方案比较

### 方案 A：只改文档

只在新文档里说明正确命令，不改代码。

优点：零代码风险。
缺点：后续仍容易手写错命令，stub 空成功的测试卫生问题仍存在。

### 方案 B：最小代码收口

在 `backend/Makefile` 增加 `test-server-unit` 目标；把 `api_contract_test.go` 的两个新增 stub 方法改为 `errors.New("not implemented")`。

优点：改动最小，直接消除两个残留点；符合现有 Makefile 和 stub 风格。
缺点：多一个 Makefile 目标，需要后续文档记住使用。

### 方案 C：抽象测试 stub

把多个 package 中的 `UserSubscriptionRepository` stub 抽到共享 testutil，统一行为。

优点：长期减少重复。
缺点：当前问题很窄，跨 package 测试依赖会增加耦合，改动面明显变大。

## 推荐设计

采用方案 B。

### Makefile 入口

在 `backend/Makefile` 中：

- `.PHONY` 增加 `test-server-unit`。
- 新增目标：

```makefile
test-server-unit:
	go test -count=1 -tags=unit ./internal/server
```

原因：

- `api_contract_test.go` 带 `//go:build unit`，必须显式带 `-tags=unit`。
- `-count=1` 避免缓存掩盖刚改过的 stub 编译问题。
- 保持入口窄，不把全量 unit 的耗时强加给单包验证。

### stub 失败优先

在 `backend/internal/server/api_contract_test.go` 中，把两个新增方法改为：

```go
func (stubUserSubscriptionRepo) CalibrateActiveDailyUsageWindows(ctx context.Context, dailyStart, upperBound, now time.Time, batchSize int) (*service.SubscriptionDailyWindowCalibrationResult, error) {
	return nil, errors.New("not implemented")
}

func (stubUserSubscriptionRepo) CountStaleActiveDailyWindows(ctx context.Context, windowStart, now time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}
```

原因：

- `api_contract_test.go` 当前只验证 API contract，不负责验证订阅日窗口后台校准。
- 同文件多数未使用 stub 方法已经返回 `not implemented`，保持一致。
- 未来如果 contract 测试意外触达后台校准方法，应立即失败并补显式测试建模，而不是吞掉依赖。

## 验证设计

实施后至少运行：

```bash
make -C backend test-server-unit
go test -count=1 -tags=unit ./...
git diff --check
```

其中：

- `make -C backend test-server-unit` 验证新增入口和目标 package。
- `go test -count=1 -tags=unit ./...` 验证其它 unit stub 没有被新行为影响。
- `git diff --check` 验证格式和尾随空白。

如时间允许，可补跑：

```bash
go test -count=1 ./...
```

用于确认默认测试集仍正常。

## 风险与处理

- 风险：`api_contract_test.go` 某条现有用例实际隐式调用新增方法。
  - 处理：`make -C backend test-server-unit` 会直接失败；届时应给该用例补明确的 fake 行为，而不是恢复空成功。
- 风险：后续文档仍手写 `go test ./internal/server`。
  - 处理：在结果文档和 `AGENTS.md` 里记录 `make -C backend test-server-unit` 作为首选入口。

## 交付边界

本设计只要求后续实施两个文件的代码改动：

- `backend/Makefile`
- `backend/internal/server/api_contract_test.go`

并新增一份结果上下文文档；不提交 git，除非用户明确要求。
