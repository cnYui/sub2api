# OpenAI Usage Fact 与 Durable Outbox Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 OpenAI 成功响应的用量先同步持久化为不可丢失 usage fact，再由 PostgreSQL durable outbox 幂等执行扣费和 `usage_logs` 投影，消除内存队列丢任务及扣费失败导致用量消失的问题。

**Architecture:** 新建 `usage_facts` 表同时承担不可变事实和 durable outbox；`OpenAIGatewayService` 把现有 `RecordUsage()` 拆成“构建事实、持久化事实、结算事实”三段。HTTP 非流式响应使用缓冲 writer，SSE 使用终止事件 gate，确保 fact 写入成功后才交付完整成功响应；后台 worker 使用 `FOR UPDATE SKIP LOCKED` 拉取 pending fact，复用现有 `UsageBillingRepository.Apply()` 幂等扣费，并独立投影 `usage_logs`。

**Tech Stack:** Go、Gin、PostgreSQL、原生 `database/sql`、Wire、项目现有 migration runner、`testify`、Go build tags `unit`/`integration`。

---

## 实施边界

本计划覆盖 OpenAI HTTP Responses、Chat Completions、Images、Embeddings 及 Anthropic Messages 兼容入口。通用 Anthropic/Gemini gateway 不在本计划内。OpenAI WS 先改为同步持久化 fact、禁止内存队列丢弃；协议级终止帧 gate 另行实施。

本计划不实现流量卡预留。流量卡不足时，现有扣费事务仍可能返回 `ErrInsufficientBalance`，但 usage fact 和 `usage_logs` 必须保留，fact 标记为 `debt`。预留和 debt gate 由下一份计划实现。

## 文件结构

**新建：**

- `backend/migrations/163_usage_facts_durable_outbox.sql`
- `backend/internal/service/usage_fact.go`
- `backend/internal/service/usage_fact_test.go`
- `backend/internal/repository/usage_fact_repo.go`
- `backend/internal/repository/usage_fact_repo_integration_test.go`
- `backend/internal/service/usage_fact_settlement_service.go`
- `backend/internal/service/usage_fact_settlement_service_test.go`
- `backend/internal/service/usage_fact_worker.go`
- `backend/internal/service/usage_fact_worker_test.go`
- `backend/internal/handler/usage_fact_response_gate.go`
- `backend/internal/handler/usage_fact_response_gate_test.go`

**修改：**

- `backend/internal/repository/migrations_schema_integration_test.go`
- `backend/internal/repository/wire.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_gateway_record_usage_test.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/wire.go`
- `backend/internal/config/config.go`
- `backend/internal/config/config_test.go`
- `deploy/config.example.yaml`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/openai_chat_completions.go`
- `backend/internal/handler/openai_images.go`
- `backend/internal/handler/openai_embeddings.go`
- `backend/internal/handler/usage_record_submit_task_test.go`
- `backend/cmd/server/wire.go`
- `backend/cmd/server/wire_gen.go`

## Task 1：创建 usage fact/outbox schema

**Files:**

- Create: `backend/migrations/163_usage_facts_durable_outbox.sql`
- Modify: `backend/internal/repository/migrations_schema_integration_test.go`

- [ ] **Step 1: 写 schema 集成测试**

在 `TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate` 中加入：

```go
var usageFactsRegclass sql.NullString
require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.usage_facts')").Scan(&usageFactsRegclass))
require.True(t, usageFactsRegclass.Valid, "expected usage_facts table to exist")
requireColumn(t, tx, "usage_facts", "request_id", "character varying", 255, false)
requireColumn(t, tx, "usage_facts", "request_fingerprint", "character varying", 64, false)
requireColumn(t, tx, "usage_facts", "payload", "jsonb", 0, false)
requireColumn(t, tx, "usage_facts", "billing_status", "character varying", 20, false)
requireIndex(t, tx, "usage_facts", "idx_usage_facts_request_api_key")
requireIndex(t, tx, "usage_facts", "idx_usage_facts_pending_claim")
requireConstraintDefinitionContains(t, tx, "usage_facts", "usage_facts_billing_status_check", "pending", "settling", "settled", "debt", "failed")
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
cd backend
go test -count=1 -tags=integration ./internal/repository -run TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate
```

Expected: FAIL，包含 `expected usage_facts table to exist`。

- [ ] **Step 3: 新建迁移**

```sql
CREATE TABLE IF NOT EXISTS usage_facts (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(255) NOT NULL,
    api_key_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    request_fingerprint VARCHAR(64) NOT NULL,
    payload_version INT NOT NULL DEFAULT 1,
    payload JSONB NOT NULL,
    billing_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempt_count INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT NOT NULL DEFAULT '',
    completed_at TIMESTAMPTZ NOT NULL,
    settled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT usage_facts_billing_status_check
        CHECK (billing_status IN ('pending', 'settling', 'settled', 'debt', 'failed')),
    CONSTRAINT usage_facts_attempt_count_check CHECK (attempt_count >= 0),
    CONSTRAINT usage_facts_payload_version_check CHECK (payload_version > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_usage_facts_request_api_key
    ON usage_facts (request_id, api_key_id);

CREATE INDEX IF NOT EXISTS idx_usage_facts_pending_claim
    ON usage_facts (next_attempt_at, id)
    WHERE billing_status IN ('pending', 'settling');
```

不添加用户、Key、账号外键，保证业务实体清理后财务事实仍保留。

- [ ] **Step 4: 运行迁移测试确认 GREEN**

```bash
cd backend
go test -count=1 -tags=integration ./internal/repository -run TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate
```

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add backend/migrations/163_usage_facts_durable_outbox.sql backend/internal/repository/migrations_schema_integration_test.go
git commit -m "feat: add durable usage facts schema"
```

## Task 2：定义 usage fact 领域模型

**Files:**

- Create: `backend/internal/service/usage_fact.go`
- Create: `backend/internal/service/usage_fact_test.go`

- [ ] **Step 1: 写 payload 往返测试**

```go
//go:build unit

package service

func TestUsageFactPayloadRoundTripPreservesBillingAndLog(t *testing.T) {
    payload := UsageFactPayload{
        BillingCommand: UsageBillingCommand{RequestID: "req-1", APIKeyID: 9, UserID: 7, TrafficPackCost: 0.25},
        UsageLog: UsageLog{RequestID: "req-1", APIKeyID: 9, UserID: 7, ActualCost: 0.25},
    }
    raw, err := EncodeUsageFactPayload(payload)
    require.NoError(t, err)
    got, err := DecodeUsageFactPayload(UsageFactPayloadVersion1, raw)
    require.NoError(t, err)
    require.Equal(t, payload.BillingCommand.RequestID, got.BillingCommand.RequestID)
    require.Equal(t, payload.UsageLog.ActualCost, got.UsageLog.ActualCost)
}

func TestNewUsageFactRejectsMissingRequestID(t *testing.T) {
    _, err := NewUsageFact(UsageFactPayload{})
    require.ErrorIs(t, err, ErrUsageFactRequestIDRequired)
}
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestUsageFactPayloadRoundTrip|TestNewUsageFact'
```

Expected: FAIL。

- [ ] **Step 3: 实现公开边界**

```go
const UsageFactPayloadVersion1 = 1

const (
    UsageFactStatusPending  = "pending"
    UsageFactStatusSettling = "settling"
    UsageFactStatusSettled  = "settled"
    UsageFactStatusDebt     = "debt"
    UsageFactStatusFailed   = "failed"
)

type UsageFactPayload struct {
    BillingCommand UsageBillingCommand `json:"billing_command"`
    UsageLog UsageLog `json:"usage_log"`
    Effects UsageSettlementEffectsPayload `json:"effects"`
}

type UsageSettlementEffectsPayload struct {
    UserID int64 `json:"user_id"`
    APIKeyID int64 `json:"api_key_id"`
    AccountID int64 `json:"account_id"`
    GroupID *int64 `json:"group_id,omitempty"`
    Platform string `json:"platform"`
    ActualCost float64 `json:"actual_cost"`
    IsSubscription bool `json:"is_subscription"`
    IsTrafficCredit bool `json:"is_traffic_credit"`
}

type UsageFact struct {
    ID int64
    RequestID string
    APIKeyID int64
    UserID int64
    AccountID int64
    RequestFingerprint string
    PayloadVersion int
    Payload json.RawMessage
    BillingStatus string
    AttemptCount int
    NextAttemptAt time.Time
    LastError string
    CompletedAt time.Time
    SettledAt *time.Time
    CreatedAt time.Time
    UpdatedAt time.Time
}

type UsageFactRepository interface {
    CreatePending(ctx context.Context, fact *UsageFact) (*UsageFact, bool, error)
    ClaimPending(ctx context.Context, limit int, now time.Time) ([]UsageFact, error)
    MarkSettled(ctx context.Context, id int64, settledAt time.Time) error
    MarkDebt(ctx context.Context, id int64, reason string, settledAt time.Time) error
    MarkRetry(ctx context.Context, id int64, reason string, nextAttemptAt time.Time) error
}
```

实现 `EncodeUsageFactPayload`、`DecodeUsageFactPayload` 和 `NewUsageFact`；后者先 `BillingCommand.Normalize()`，并复制 command 的 request、用户、Key、账号、指纹和完成时间到索引列。

- [ ] **Step 4: 运行 GREEN 并提交**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestUsageFactPayloadRoundTrip|TestNewUsageFact'
git add internal/service/usage_fact.go internal/service/usage_fact_test.go
git commit -m "feat: define durable usage fact model"
```

Expected: PASS 后提交。

## Task 3：实现 PostgreSQL usage fact repository

**Files:**

- Create: `backend/internal/repository/usage_fact_repo.go`
- Create: `backend/internal/repository/usage_fact_repo_integration_test.go`
- Modify: `backend/internal/repository/wire.go`

- [ ] **Step 1: 写 repository 集成测试**

测试名固定为：

```go
func TestUsageFactRepository_CreatePendingIsIdempotent(t *testing.T)
func TestUsageFactRepository_CreatePendingRejectsFingerprintConflict(t *testing.T)
func TestUsageFactRepository_ClaimPendingSkipsLockedRows(t *testing.T)
func TestUsageFactRepository_MarkDebtPreservesPayload(t *testing.T)
```

分别验证同指纹幂等、异指纹冲突、`SKIP LOCKED` 和 debt 不修改 payload。

- [ ] **Step 2: 运行测试确认 RED**

```bash
cd backend
go test -count=1 -tags=integration ./internal/repository -run TestUsageFactRepository
```

Expected: FAIL。

- [ ] **Step 3: 实现 CreatePending**

```sql
INSERT INTO usage_facts (
    request_id, api_key_id, user_id, account_id, request_fingerprint,
    payload_version, payload, billing_status, next_attempt_at, completed_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,'pending',$8,$9)
ON CONFLICT (request_id, api_key_id) DO NOTHING
RETURNING id, created_at, updated_at;
```

冲突后读取完整 fact；指纹相同返回原 fact，指纹不同返回 `service.ErrUsageBillingRequestConflict`。

- [ ] **Step 4: 实现 claim 和状态推进**

```sql
WITH candidates AS (
    SELECT id FROM usage_facts
    WHERE billing_status IN ('pending', 'settling')
      AND next_attempt_at <= $1
    ORDER BY next_attempt_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT $2
)
UPDATE usage_facts AS f
SET billing_status = 'settling', attempt_count = attempt_count + 1, updated_at = NOW()
FROM candidates
WHERE f.id = candidates.id
RETURNING f.id, f.request_id, f.api_key_id, f.user_id, f.account_id,
          f.request_fingerprint, f.payload_version, f.payload,
          f.billing_status, f.attempt_count, f.next_attempt_at,
          f.last_error, f.completed_at, f.settled_at, f.created_at, f.updated_at;
```

`MarkSettled/MarkDebt` 只从 `pending/settling` 推进；`MarkRetry` 恢复 `pending`。

- [ ] **Step 5: 注册 provider，运行 GREEN 并提交**

```bash
cd backend
go test -count=1 -tags=integration ./internal/repository -run TestUsageFactRepository
git add internal/repository/usage_fact_repo.go internal/repository/usage_fact_repo_integration_test.go internal/repository/wire.go
git commit -m "feat: persist and claim usage facts"
```

Expected: PASS 后提交。

## Task 4：实现 fact-first 结算服务

**Files:**

- Create: `backend/internal/service/usage_fact_settlement_service.go`
- Create: `backend/internal/service/usage_fact_settlement_service_test.go`
- Modify: `backend/internal/service/gateway_service.go`

- [ ] **Step 1: 写结算测试**

```go
func TestUsageFactSettlementService_MarksDebtAndWritesUsageLogOnInsufficientBalance(t *testing.T) {
    factRepo := &usageFactRepoStub{}
    billingRepo := &usageBillingRepoStub{err: ErrInsufficientBalance}
    logRepo := &usageLogRepoStub{}
    svc := NewUsageFactSettlementService(factRepo, billingRepo, logRepo, nil)
    err := svc.Settle(context.Background(), usageFactWithTrafficCost(0.25))
    require.NoError(t, err)
    require.Equal(t, int64(1), factRepo.markDebtID)
    require.Equal(t, "req-1", logRepo.created.RequestID)
}

func TestUsageFactSettlementService_RetriesTransientBillingFailure(t *testing.T) {
    transient := errors.New("database unavailable")
    svc := NewUsageFactSettlementService(&usageFactRepoStub{}, &usageBillingRepoStub{err: transient}, &usageLogRepoStub{}, nil)
    require.ErrorIs(t, svc.Settle(context.Background(), usageFactWithBalanceCost(0.25)), transient)
}
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run TestUsageFactSettlementService
```

Expected: FAIL。

- [ ] **Step 3: 导出 command 构建边界**

```go
func BuildUsageBillingCommand(requestID string, usageLog *UsageLog, p *postUsageBillingParams) *UsageBillingCommand {
    return buildUsageBillingCommand(requestID, usageLog, p)
}
```

payload 不保存 `*User/*APIKey/*Account` 指针，只保存 ID、费用命令和 `UsageLog` 快照。

结算副作用使用显式接口：

```go
type UsageSettlementEffects interface {
    Apply(ctx context.Context, payload UsageSettlementEffectsPayload, result *UsageBillingApplyResult)
}
```

`NewUsageFactSettlementService` 的参数固定为 `(factRepo, billingRepo, usageLogRepo, effects)`；测试可传 `nil`，生产由 Wire 注入正式实现。

- [ ] **Step 4: 实现 Settle**

```go
func (s *UsageFactSettlementService) Settle(ctx context.Context, fact UsageFact) error {
    payload, err := DecodeUsageFactPayload(fact.PayloadVersion, fact.Payload)
    if err != nil {
        return s.factRepo.MarkDebt(ctx, fact.ID, "invalid payload: "+err.Error(), time.Now())
    }
    result, billingErr := s.billingRepo.Apply(ctx, &payload.BillingCommand)
    if billingErr != nil && !errors.Is(billingErr, ErrInsufficientBalance) {
        return billingErr
    }
    if _, err := s.usageLogRepo.Create(ctx, &payload.UsageLog); err != nil {
        return err
    }
    now := time.Now()
    if errors.Is(billingErr, ErrInsufficientBalance) {
        return s.factRepo.MarkDebt(ctx, fact.ID, billingErr.Error(), now)
    }
    if err := s.factRepo.MarkSettled(ctx, fact.ID, now); err != nil {
        return err
    }
    if s.effects != nil {
        s.effects.Apply(ctx, payload.Effects, result)
    }
    return nil
}
```

必须调用正式 `UsageLogRepository.Create`，不得调用 `CreateBestEffort`。

- [ ] **Step 5: 运行 GREEN 并提交**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run TestUsageFactSettlementService
git add internal/service/usage_fact_settlement_service.go internal/service/usage_fact_settlement_service_test.go internal/service/gateway_service.go
git commit -m "feat: settle durable usage facts"
```

Expected: PASS 后提交。

## Task 5：增加 durable worker 和配置

**Files:**

- Create: `backend/internal/service/usage_fact_worker.go`
- Create: `backend/internal/service/usage_fact_worker_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `deploy/config.example.yaml`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/cmd/server/wire.go`

- [ ] **Step 1: 写 worker 重试测试**

```go
func TestUsageFactWorker_RetriesWithBackoff(t *testing.T) {
    repo := &usageFactRepoStub{claimed: []UsageFact{{ID: 1, AttemptCount: 1}}}
    settlement := &usageFactSettlementStub{err: errors.New("temporary")}
    worker := NewUsageFactWorker(repo, settlement, UsageFactWorkerConfig{BatchSize: 10, PollInterval: time.Millisecond, TaskTimeout: time.Second})
    worker.runOnce(context.Background())
    require.Equal(t, int64(1), repo.retryID)
    require.True(t, repo.retryAt.After(time.Now()))
}
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run TestUsageFactWorker
```

Expected: FAIL。

- [ ] **Step 3: 增加配置**

```go
UsageFactWorkerEnabled bool `mapstructure:"usage_fact_worker_enabled"`
UsageFactWorkerPollIntervalMS int `mapstructure:"usage_fact_worker_poll_interval_ms"`
UsageFactWorkerBatchSize int `mapstructure:"usage_fact_worker_batch_size"`
UsageFactWorkerTaskTimeoutSeconds int `mapstructure:"usage_fact_worker_task_timeout_seconds"`
```

默认值为 `true/250/100/10`，数值必须大于 0；同步更新 `deploy/config.example.yaml`。

- [ ] **Step 4: 实现 worker**

```go
type UsageFactSettler interface {
    Settle(ctx context.Context, fact UsageFact) error
}

type UsageFactWorker struct {
    repo UsageFactRepository
    settler UsageFactSettler
    cfg UsageFactWorkerConfig
    stopCh chan struct{}
    stopOnce sync.Once
    wg sync.WaitGroup
}
```

失败退避使用 `min(attempt_count, 8)` 的指数秒数，最大 256 秒，无固定最大重试次数。`Stop()` 等待当前批次结束。

- [ ] **Step 5: 注册生命周期**

`service/wire.go` 新增 `ProvideUsageFactWorker` 并启动；`cmd/server/wire.go` cleanup 加：

```go
{"UsageFactWorker", func() error {
    if usageFactWorker != nil {
        usageFactWorker.Stop()
    }
    return nil
}},
```

- [ ] **Step 6: 运行 GREEN 并提交**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/config ./internal/service -run 'TestUsageFactWorker|Billing'
git add internal/service/usage_fact_worker.go internal/service/usage_fact_worker_test.go internal/config/config.go internal/config/config_test.go ../deploy/config.example.yaml internal/service/wire.go cmd/server/wire.go
git commit -m "feat: process usage facts from durable outbox"
```

Expected: PASS 后提交。

## Task 6：拆分 OpenAI RecordUsage

**Files:**

- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_record_usage_test.go`

- [ ] **Step 1: 写 fact-first 测试**

```go
func TestOpenAIGatewayServicePersistUsageFact_DoesNotApplyBillingInline(t *testing.T) {
    factRepo := &usageFactRepoStub{}
    billingRepo := &usageBillingRepoStub{}
    svc := newOpenAIRecordUsageService(t, billingRepo)
    svc.usageFactRepo = factRepo
    fact, err := svc.PersistUsageFact(context.Background(), validOpenAIRecordUsageInput())
    require.NoError(t, err)
    require.NotNil(t, fact)
    require.Equal(t, 1, factRepo.createCalls)
    require.Zero(t, billingRepo.applyCalls)
}
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run TestOpenAIGatewayServicePersistUsageFact
```

Expected: FAIL。

- [ ] **Step 3: 拆分方法**

`OpenAIGatewayService` 注入 `UsageFactRepository`。把当前 `RecordUsage()` 中 token 归一化、定价、计费来源、`UsageLog` 和 `UsageBillingCommand` 构建移动到：

```go
func (s *OpenAIGatewayService) BuildUsageFact(ctx context.Context, input *OpenAIRecordUsageInput) (*UsageFact, error)
```

持久化方法：

```go
func (s *OpenAIGatewayService) PersistUsageFact(ctx context.Context, input *OpenAIRecordUsageInput) (*UsageFact, error) {
    fact, err := s.BuildUsageFact(ctx, input)
    if err != nil {
        return nil, err
    }
    persisted, _, err := s.usageFactRepo.CreatePending(ctx, fact)
    return persisted, err
}
```

兼容 `RecordUsage()` 只调用 `PersistUsageFact()`，不得内联 billing 或 `writeUsageLogBestEffort()`。

- [ ] **Step 4: 调整旧测试并运行回归**

原来断言余额立即变化的测试改为断言 fact payload；实际扣费移到 settlement service 测试。保留模型定价、token 分类、图片和渠道字段断言。

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestOpenAIGatewayService|TestBuildUsageBillingCommand|TestUsageFactSettlementService'
```

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_record_usage_test.go
git commit -m "refactor: persist OpenAI usage before settlement"
```

## Task 7：实现 HTTP/SSE 响应屏障

**Files:**

- Create: `backend/internal/handler/usage_fact_response_gate.go`
- Create: `backend/internal/handler/usage_fact_response_gate_test.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Modify: `backend/internal/handler/openai_chat_completions.go`
- Modify: `backend/internal/handler/openai_images.go`
- Modify: `backend/internal/handler/openai_embeddings.go`
- Modify: `backend/internal/handler/usage_record_submit_task_test.go`

- [ ] **Step 1: 写 response gate 测试**

```go
func TestUsageFactResponseGate_BuffersNonStreamingUntilRelease(t *testing.T) {
    recorder := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(recorder)
    gate := newUsageFactResponseGate(c.Writer, false)
    _, _ = gate.Write([]byte(`{"id":"resp_1"}`))
    require.Empty(t, recorder.Body.String())
    require.NoError(t, gate.Release())
    require.JSONEq(t, `{"id":"resp_1"}`, recorder.Body.String())
}

func TestUsageFactResponseGate_HoldsSSETerminalEvent(t *testing.T) {
    recorder := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(recorder)
    gate := newUsageFactResponseGate(c.Writer, true)
    _, _ = gate.Write([]byte("data: {\"type\":\"response.output_text.delta\"}\n\n"))
    _, _ = gate.Write([]byte("data: {\"type\":\"response.completed\"}\n\n"))
    require.Contains(t, recorder.Body.String(), "response.output_text.delta")
    require.NotContains(t, recorder.Body.String(), "response.completed")
    require.NoError(t, gate.Release())
    require.Contains(t, recorder.Body.String(), "response.completed")
}
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run TestUsageFactResponseGate
```

Expected: FAIL。

- [ ] **Step 3: 实现 gate**

```go
type usageFactResponseGate struct {
    gin.ResponseWriter
    stream bool
    pending bytes.Buffer
    released bool
    mu sync.Mutex
}
```

非流式暂存全部 body。流式按 SSE 空行分帧，只暂存 `response.completed`、`response.incomplete`、`[DONE]` 及其后续数据；普通 delta 立即写入原 writer。`Release()` 写出 pending，`Discard()` 清空 pending，不得缓存整个流。

- [ ] **Step 4: handler 同步持久化 fact**

成功路径固定为：安装 gate -> Forward -> `PersistUsageFact` -> `gate.Release()`。fact 写入失败时，非流式丢弃上游 200 body 并返回 503；SSE 丢弃终止事件并发送 `billing_persistence_error` 后关闭。

删除普通 OpenAI 成功路径的 `submitOpenAIUsageRecordTask()`。WS 路径直接同步调用 `PersistUsageFact()`，不得再提交可 dropped 的闭包。

- [ ] **Step 5: 增加 handler 行为测试**

覆盖：fact repo 错误时非流式不返回上游 200 body；SSE 不发 `response.completed`；fact 成功后终止事件发出；worker pool 配置为 drop 时仍调用 fact repo。

- [ ] **Step 6: 运行 GREEN 并提交**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run 'TestUsageFactResponseGate|Test.*Usage.*Persist|TestSubmit.*Usage'
git add internal/handler/usage_fact_response_gate.go internal/handler/usage_fact_response_gate_test.go internal/handler/openai_gateway_handler.go internal/handler/openai_chat_completions.go internal/handler/openai_images.go internal/handler/openai_embeddings.go internal/handler/usage_record_submit_task_test.go
git commit -m "fix: persist OpenAI usage before completing responses"
```

Expected: PASS 后提交。

## Task 8：生成 Wire、完整验证和结果文档

**Files:**

- Modify: `backend/cmd/server/wire_gen.go`
- Create: `docs/ai/context/20260715-092959-openai-usage-fact-durable-outbox-result_CN.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: 重新生成 Wire**

```bash
cd backend/cmd/server
go generate ./...
```

若 `go generate` 没有注册 Wire，使用仓库锁定版本的 `wire` 命令；不得手工拼接依赖图。

- [ ] **Step 2: 运行目标单测**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/config ./internal/service ./internal/handler -run 'UsageFact|OpenAIGatewayService|UsageRecord'
```

Expected: PASS。

- [ ] **Step 3: 运行 repository 集成测试**

```bash
cd backend
go test -count=1 -tags=integration ./internal/repository -run 'TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate|TestUsageFactRepository|TestUsageBillingRepository'
```

Expected: PASS。

- [ ] **Step 4: 运行完整回归和编译**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler
go test -count=1 ./cmd/server
```

Expected: PASS。

- [ ] **Step 5: 格式和差异检查**

```bash
gofmt -w backend/internal/service/usage_fact*.go backend/internal/repository/usage_fact*.go backend/internal/handler/usage_fact_response_gate*.go
git diff --check
```

Expected: `git diff --check` 无输出。

- [ ] **Step 6: 写结果文档并更新记忆**

结果文档记录 migration、事实状态机、response gate 覆盖入口、测试命令、WS 终止帧限制和部署状态。`AGENTS.md` 只追加一条定论。

- [ ] **Step 7: 提交最终文档**

```bash
git add backend/cmd/server/wire_gen.go docs/ai/context/20260715-092959-openai-usage-fact-durable-outbox-result_CN.md AGENTS.md
git commit -m "docs: record durable OpenAI usage fact rollout"
```

## 完成条件

- OpenAI HTTP 成功响应必须先创建唯一 usage fact。
- 普通 OpenAI usage 不再经过允许 `drop/sample` 的内存队列。
- `UsageBillingRepository.Apply()` 返回 `ErrInsufficientBalance` 时，fact 状态为 `debt`，`usage_logs` 仍存在。
- worker 重启后可继续处理 pending fact，重复处理不重复扣费或重复写 usage log。
- 非流式成功 body 和 SSE 终止事件在 fact 持久化成功前不可见。
- fact 写入失败时不得交付完整成功响应。
- 未修改流量卡预留逻辑；该部分由下一份计划完成。

## 设计依据

- `docs/ai/context/20260715-090554-traffic-credit-preauthorization-and-durable-usage-design_CN.md`
- `docs/ai/context/20260714-204422-expired-user-request-billing-gap-code-cause_CN.md`
- `docs/ai/context/20260713-134754-relay-architecture-security-hardening-whitepaper_CN.md`
