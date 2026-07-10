# API Key Concurrency Replacement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把模型请求入口并发限制从用户级替换为 API Key 级，让每把 Key 默认最多 5 并发，多把 Key 可自然扩大并发。

**Architecture:** 在现有 Redis Sorted Set 并发槽框架中新增 `api_key` 维度。网关入口只替换第一层调用方槽位；上游账号槽、计费、订阅和账号选择保持不变。

**Tech Stack:** Go、Gin、Redis、Ent、现有 unit test build tag。

---

## 文件结构

- Modify: `backend/internal/service/concurrency_service.go`
  - 扩展 `ConcurrencyCache` 接口，新增 API Key 槽位与等待队列方法。
  - 新增 `AcquireAPIKeySlot`、`IncrementAPIKeyWaitCount`、`DecrementAPIKeyWaitCount`。
- Modify: `backend/internal/repository/concurrency_cache.go`
  - 新增 `concurrency:api_key:` 和 `concurrency:wait:api_key:` key。
  - 复用现有 Redis Lua 脚本实现 API Key acquire/release/count/wait。
  - 启动清理覆盖 API Key 槽位和等待队列。
- Modify: `backend/internal/handler/gateway_helper.go`
  - 新增 API Key helper 方法。
  - `waitForSlotWithPingTimeout` 支持 `api_key` slot type。
- Modify: `backend/internal/handler/gateway_handler.go`
- Modify: `backend/internal/handler/gateway_handler_responses.go`
- Modify: `backend/internal/handler/gateway_handler_chat_completions.go`
- Modify: `backend/internal/handler/gemini_v1beta_handler.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
  - 把第一层 user slot 替换为 api key slot。
- Modify tests/stubs:
  - `backend/internal/service/concurrency_service_test.go`
  - `backend/internal/handler/gateway_helper_fastpath_test.go`
  - `backend/internal/handler/gateway_helper_hotpath_test.go`
  - 其它编译报错的 `ConcurrencyCache` stub 文件。
- Modify: `AGENTS.md`
  - 记录本次设计、实现和验证结论。

## Task 1: Service API Key 槽位

**Files:**
- Modify: `backend/internal/service/concurrency_service_test.go`
- Modify: `backend/internal/service/concurrency_service.go`

- [ ] **Step 1: 写失败测试**

在 `concurrency_service_test.go` 增加：

```go
func TestAcquireAPIKeySlot_UsesAPIKeyDimension(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireResult: true}
	svc := NewConcurrencyService(cache)

	result, err := svc.AcquireAPIKeySlot(context.Background(), 96, 5)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.NotNil(t, result.ReleaseFunc)

	result.ReleaseFunc()
	require.Equal(t, []int64{96}, cache.releasedAPIKeyIDs)
	require.Len(t, cache.releasedRequestIDs, 1)
	require.NotEmpty(t, cache.releasedRequestIDs[0])
}

func TestAcquireAPIKeySlot_UnlimitedConcurrency(t *testing.T) {
	svc := NewConcurrencyService(&stubConcurrencyCacheForTest{})

	result, err := svc.AcquireAPIKeySlot(context.Background(), 96, 0)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.NotNil(t, result.ReleaseFunc)
}
```

- [ ] **Step 2: 运行失败测试**

Run:

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestAcquireAPIKeySlot'
```

Expected: FAIL，原因是 `AcquireAPIKeySlot` 或 `ConcurrencyCache` API Key 方法不存在。

- [ ] **Step 3: 最小实现**

在 `ConcurrencyCache` 增加：

```go
AcquireAPIKeySlot(ctx context.Context, apiKeyID int64, maxConcurrency int, requestID string) (bool, error)
ReleaseAPIKeySlot(ctx context.Context, apiKeyID int64, requestID string) error
GetAPIKeyConcurrency(ctx context.Context, apiKeyID int64) (int, error)
IncrementAPIKeyWaitCount(ctx context.Context, apiKeyID int64, maxWait int) (bool, error)
DecrementAPIKeyWaitCount(ctx context.Context, apiKeyID int64) error
```

在 `ConcurrencyService` 增加与 `AcquireUserSlot` 同构的方法，释放失败日志写 `api key slot`。

- [ ] **Step 4: 运行通过测试**

Run:

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestAcquireAPIKeySlot'
```

Expected: PASS。

## Task 2: Redis cache API Key 维度

**Files:**
- Modify: `backend/internal/repository/concurrency_cache.go`

- [ ] **Step 1: 写失败测试**

如现有 repository 单测依赖 Redis，可优先用 service 层 stub 覆盖行为；本任务的失败信号来自 Task 1 扩展接口后 `repository.concurrencyCache` 未实现新方法导致编译失败。

Run:

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
```

Expected: FAIL，`*concurrencyCache does not implement service.ConcurrencyCache`。

- [ ] **Step 2: 最小实现**

在 `concurrency_cache.go` 增加：

```go
apiKeySlotKeyPrefix = "concurrency:api_key:"
apiKeyWaitKeyPrefix = "concurrency:wait:api_key:"
```

新增 key helper 和五个方法，复用 `acquireScript`、`getCountScript`、`incrementWaitScript`、`decrementWaitScript`。`CleanupStaleProcessSlots` 的 slotPatterns 加入 `apiKeySlotKeyPrefix + "*"`, waitPatterns 加入 `apiKeyWaitKeyPrefix + "*"`.

- [ ] **Step 3: 运行通过测试**

Run:

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/repository
```

Expected: PASS 或只剩其它 stub 未补齐编译错误。

## Task 3: Handler helper 切换能力

**Files:**
- Modify: `backend/internal/handler/gateway_helper_fastpath_test.go`
- Modify: `backend/internal/handler/gateway_helper_hotpath_test.go`
- Modify: `backend/internal/handler/gateway_helper.go`

- [ ] **Step 1: 写失败测试**

在 fastpath 测试里增加 `TestConcurrencyHelper_TryAcquireAPIKeySlot`，断言 cache 收到 `apiKeyID=96`。在 hotpath 测试里增加 `TestAcquireAPIKeySlotWithWait_WaitSuccessDecrementsBeforeReturn`，复用 user wait 测试结构但断言 `apiKeyWaitIncrements/decrements`。

- [ ] **Step 2: 运行失败测试**

Run:

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run 'TestConcurrencyHelper_TryAcquireAPIKeySlot|TestAcquireAPIKeySlotWithWait'
```

Expected: FAIL，helper API 不存在或未调用 API Key wait queue。

- [ ] **Step 3: 最小实现**

新增：

```go
func (h *ConcurrencyHelper) TryAcquireAPIKeySlot(ctx context.Context, apiKeyID int64, maxConcurrency int) (func(), bool, error)
func (h *ConcurrencyHelper) AcquireAPIKeySlotWithWait(c *gin.Context, apiKeyID int64, maxConcurrency int, isStream bool, streamStarted *bool) (func(), error)
```

`waitForSlotWithPingTimeout` 的 `acquireSlot` 用 `switch slotType` 支持 `api_key`。

- [ ] **Step 4: 运行通过测试**

Run:

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run 'TestConcurrencyHelper_TryAcquireAPIKeySlot|TestAcquireAPIKeySlotWithWait'
```

Expected: PASS。

## Task 4: 网关入口替换 user slot

**Files:**
- Modify: `backend/internal/handler/gateway_handler.go`
- Modify: `backend/internal/handler/gateway_handler_responses.go`
- Modify: `backend/internal/handler/gateway_handler_chat_completions.go`
- Modify: `backend/internal/handler/gemini_v1beta_handler.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`

- [ ] **Step 1: 写失败测试**

优先补 helper/handler 级测试，断言入口使用 API Key 槽而不是 User 槽。可在已有 handler 测试 stub 中记录 `AcquireAPIKeySlot` 和 `AcquireUserSlot` 调用次数。

- [ ] **Step 2: 运行失败测试**

Run:

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run 'Concurrency|Responses|ChatCompletions|Gemini'
```

Expected: 新增断言 FAIL，当前代码仍调用 user slot。

- [ ] **Step 3: 最小实现**

替换以下调用：

```go
AcquireUserSlotWithWait(c, subject.UserID, subject.Concurrency, ...)
TryAcquireUserSlot(ctx, subject.UserID, subject.Concurrency)
```

为：

```go
AcquireAPIKeySlotWithWait(c, apiKey.ID, subject.Concurrency, ...)
TryAcquireAPIKeySlot(ctx, apiKey.ID, subject.Concurrency)
```

日志改为 `api_key_slot_acquire_failed`，错误 slot type 传 `api_key`。

- [ ] **Step 4: 运行通过测试**

Run:

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler
```

Expected: PASS。

## Task 5: 全量目标验证和上下文记录

**Files:**
- Modify: `AGENTS.md`
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-api-key-concurrency-replacement-result_CN.md`

- [ ] **Step 1: 运行后端目标测试**

Run:

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler
```

Expected: PASS。

- [ ] **Step 2: 运行 diff 检查**

Run:

```bash
git diff --check
```

Expected: 无输出，exit 0。

- [ ] **Step 3: 写结果文档和 AGENTS 记忆**

记录实际改动、验证命令、未部署事实和发布注意事项。AGENTS 顶部新增一条结论，说明公网 18084 需要构建发布后才生效。
