# 流量卡预授权、事务预留与 Debt Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在订阅或余额不能承担请求时，根据最终 OpenAI 请求计算保守预算并事务预留流量卡额度；额度不足时在进入上游前拒绝，结算后释放差额，异常欠费进入 debt 并阻断后续流量卡请求。

**Architecture:** `TrafficCreditBudgetEstimator` 使用最终出站请求、输出上限和冻结价格快照计算费用上界；`TrafficCreditReservationRepository` 通过 `reserved_usd`、FEFO 分配和 `FOR UPDATE` 防止并发超卖。请求级 `BillingAuthorization` 固定唯一计费来源和 reservation ID，usage fact settlement 在同一数据库事务内消费预留、写 ledger、释放差额或记录 debt。

**Tech Stack:** Go、Gin、PostgreSQL、`database/sql`、gjson/sjson、项目现有 `BillingService/ModelPricingResolver`、Wire、`testify`、Go build tags `unit`/`integration`。

---

## 前置条件

必须先完成 `20260715-092959-openai-usage-fact-durable-outbox-implementation-plan_CN.md`，并确认：

- OpenAI usage fact 已同步持久化；
- settlement worker 可重放；
- `UsageFactPayload` 可携带扩展后的 `UsageBillingCommand`；
- 扣费失败不会删除 usage fact 或 usage log。

## 文件结构

**新建：**

- `backend/migrations/164_traffic_credit_reservations.sql`
- `backend/internal/service/traffic_credit_reservation.go`
- `backend/internal/service/traffic_credit_reservation_test.go`
- `backend/internal/repository/traffic_credit_reservation_repo.go`
- `backend/internal/repository/traffic_credit_reservation_repo_integration_test.go`
- `backend/internal/service/openai_traffic_credit_budget.go`
- `backend/internal/service/openai_traffic_credit_budget_test.go`
- `backend/internal/service/openai_billing_authorization.go`
- `backend/internal/service/openai_billing_authorization_test.go`

**修改：**

- `backend/internal/repository/migrations_schema_integration_test.go`
- `backend/internal/repository/wire.go`
- `backend/internal/service/traffic_pack.go`
- `backend/internal/repository/traffic_pack_repo.go`
- `backend/internal/service/billing_service.go`
- `backend/internal/service/usage_billing.go`
- `backend/internal/repository/usage_billing_repo.go`
- `backend/internal/repository/usage_billing_repo_integration_test.go`
- `backend/internal/service/usage_fact.go`
- `backend/internal/service/usage_fact_settlement_service.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_gateway_record_usage_test.go`
- `backend/internal/service/billing_cache_service.go`
- `backend/internal/service/effective_group_resolver.go`
- `backend/internal/config/config.go`
- `backend/internal/config/config_test.go`
- `deploy/config.example.yaml`
- `backend/internal/service/wire.go`
- `backend/cmd/server/wire_gen.go`

## Task 1：创建 reservation schema

**Files:**

- Create: `backend/migrations/164_traffic_credit_reservations.sql`
- Modify: `backend/internal/repository/migrations_schema_integration_test.go`

- [ ] **Step 1: 写 schema RED 测试**

```go
requireColumn(t, tx, "user_traffic_credits", "reserved_usd", "numeric", 0, false)

var reservationsRegclass sql.NullString
require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.traffic_credit_reservations')").Scan(&reservationsRegclass))
require.True(t, reservationsRegclass.Valid)
requireColumn(t, tx, "traffic_credit_reservations", "reserved_usd", "numeric", 0, false)
requireColumn(t, tx, "traffic_credit_reservations", "debt_usd", "numeric", 0, false)
requireIndex(t, tx, "traffic_credit_reservations", "idx_traffic_credit_reservations_request_api_key")
requireIndex(t, tx, "traffic_credit_reservations", "idx_traffic_credit_reservations_user_debt")
requireColumn(t, tx, "usage_facts", "reservation_id", "bigint", 0, true)
```

- [ ] **Step 2: 运行 RED**

```bash
cd backend
go test -count=1 -tags=integration ./internal/repository -run TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate
```

Expected: FAIL，`reserved_usd` 或 reservation 表不存在。

- [ ] **Step 3: 新建迁移**

```sql
ALTER TABLE user_traffic_credits
    ADD COLUMN IF NOT EXISTS reserved_usd DECIMAL(20,10) NOT NULL DEFAULT 0;

DO $$
BEGIN
    ALTER TABLE user_traffic_credits
        ADD CONSTRAINT user_traffic_credits_reserved_nonnegative CHECK (reserved_usd >= 0);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE user_traffic_credits
        ADD CONSTRAINT user_traffic_credits_reserved_within_remaining CHECK (reserved_usd <= remaining_usd);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS traffic_credit_reservations (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(255) NOT NULL,
    api_key_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    platform VARCHAR(30) NOT NULL,
    model VARCHAR(255) NOT NULL,
    request_fingerprint VARCHAR(64) NOT NULL,
    pricing_snapshot JSONB NOT NULL,
    reserved_usd DECIMAL(20,10) NOT NULL,
    settled_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    debt_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'reserved',
    last_error TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT traffic_credit_reservations_status_check
        CHECK (status IN ('reserved','dispatched','unknown','settled','released','debt')),
    CONSTRAINT traffic_credit_reservations_amount_check
        CHECK (reserved_usd > 0 AND settled_usd >= 0 AND debt_usd >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_traffic_credit_reservations_request_api_key
    ON traffic_credit_reservations (request_id, api_key_id);

CREATE INDEX IF NOT EXISTS idx_traffic_credit_reservations_user_debt
    ON traffic_credit_reservations (user_id, platform, id)
    WHERE status = 'debt' AND debt_usd > 0;

CREATE TABLE IF NOT EXISTS traffic_credit_reservation_items (
    reservation_id BIGINT NOT NULL REFERENCES traffic_credit_reservations(id) ON DELETE RESTRICT,
    credit_id BIGINT NOT NULL REFERENCES user_traffic_credits(id) ON DELETE RESTRICT,
    reserved_usd DECIMAL(20,10) NOT NULL,
    settled_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    PRIMARY KEY (reservation_id, credit_id),
    CONSTRAINT traffic_credit_reservation_items_amount_check
        CHECK (reserved_usd > 0 AND settled_usd >= 0 AND settled_usd <= reserved_usd)
);

ALTER TABLE usage_facts ADD COLUMN IF NOT EXISTS reservation_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_usage_facts_reservation_id ON usage_facts (reservation_id) WHERE reservation_id IS NOT NULL;
```

`CREATE TABLE IF NOT EXISTS` 内约束随表首次创建；`user_traffic_credits` 的两个新增约束已使用 `DO` 块保证迁移重放安全。

- [ ] **Step 4: 运行 GREEN 并提交**

```bash
cd backend
go test -count=1 -tags=integration ./internal/repository -run TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate
git add migrations/164_traffic_credit_reservations.sql internal/repository/migrations_schema_integration_test.go
git commit -m "feat: add traffic credit reservation schema"
```

Expected: PASS 后提交。

## Task 2：定义预留领域模型和 FEFO planner

**Files:**

- Create: `backend/internal/service/traffic_credit_reservation.go`
- Create: `backend/internal/service/traffic_credit_reservation_test.go`
- Modify: `backend/internal/service/traffic_pack.go`

- [ ] **Step 1: 写 planner 测试**

```go
func TestPlanTrafficCreditReservationsUsesAvailableAmount(t *testing.T) {
    batches := []TrafficCreditBatch{
        {ID: 1, RemainingUSD: 5, ReservedUSD: 4, ExpiresAt: time.Now().Add(time.Hour)},
        {ID: 2, RemainingUSD: 10, ReservedUSD: 0, ExpiresAt: time.Now().Add(2 * time.Hour)},
    }
    plan, covered := PlanTrafficCreditReservations(batches, 3)
    require.True(t, covered)
    require.Equal(t, []TrafficCreditReservationItem{
        {CreditID: 1, ReservedUSD: 1},
        {CreditID: 2, ReservedUSD: 2},
    }, plan)
}

func TestPlanTrafficCreditReservationsRejectsConcurrentOversell(t *testing.T) {
    plan, covered := PlanTrafficCreditReservations([]TrafficCreditBatch{{ID: 1, RemainingUSD: 1, ReservedUSD: 0.9}}, 0.2)
    require.False(t, covered)
    require.Equal(t, 0.1, plan[0].ReservedUSD)
}
```

- [ ] **Step 2: 运行 RED**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run TestPlanTrafficCreditReservations
```

Expected: FAIL。

- [ ] **Step 3: 增加领域类型**

`TrafficCreditBatch` 增加 `ReservedUSD float64`，并新增：

```go
type TrafficCreditReservationStatus string

const (
    TrafficCreditReservationReserved TrafficCreditReservationStatus = "reserved"
    TrafficCreditReservationDispatched TrafficCreditReservationStatus = "dispatched"
    TrafficCreditReservationUnknown TrafficCreditReservationStatus = "unknown"
    TrafficCreditReservationSettled TrafficCreditReservationStatus = "settled"
    TrafficCreditReservationReleased TrafficCreditReservationStatus = "released"
    TrafficCreditReservationDebt TrafficCreditReservationStatus = "debt"
)

type TrafficCreditReservationInput struct {
    RequestID string
    APIKeyID int64
    UserID int64
    Platform string
    Model string
    RequestFingerprint string
    PricingSnapshot json.RawMessage
    ReserveUSD float64
    ExpiresAt time.Time
}

type TrafficCreditReservation struct {
    ID int64
    TrafficCreditReservationInput
    SettledUSD float64
    DebtUSD float64
    Status TrafficCreditReservationStatus
}

type TrafficCreditReservationItem struct {
    CreditID int64
    ReservedUSD float64
    SettledUSD float64
}
```

`PlanTrafficCreditReservations` 复用 `PlanTrafficCreditDeductions` 的 FEFO 排序，但每批次可用额度为 `max(remaining_usd-reserved_usd, 0)`。

- [ ] **Step 4: 运行 GREEN 并提交**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestPlanTrafficCreditReservations|TestPlanTrafficCreditDeductions'
git add internal/service/traffic_credit_reservation.go internal/service/traffic_credit_reservation_test.go internal/service/traffic_pack.go
git commit -m "feat: plan traffic credit reservations"
```

Expected: PASS 后提交。

## Task 3：实现事务预留 repository

**Files:**

- Create: `backend/internal/repository/traffic_credit_reservation_repo.go`
- Create: `backend/internal/repository/traffic_credit_reservation_repo_integration_test.go`
- Modify: `backend/internal/repository/traffic_pack_repo.go`
- Modify: `backend/internal/repository/wire.go`

- [ ] **Step 1: 写并发集成测试**

```go
func TestTrafficCreditReservationRepository_ConcurrentReserveCannotOversell(t *testing.T) {
    // 创建 remaining_usd=1 的批次，并发执行两次 ReserveUSD=0.75。
    // 断言只成功一次，reserved_usd=0.75，另一请求返回 ErrInsufficientBalance。
}

func TestTrafficCreditReservationRepository_IdempotentByRequestAndKey(t *testing.T) {
    // 相同 request/key/fingerprint 返回同 reservation；不同 fingerprint 返回冲突。
}

func TestTrafficCreditReservationRepository_ReleaseRestoresAvailableAmount(t *testing.T) {
    // reserved -> released 后批次 reserved_usd 回到 0，remaining_usd 不变。
}
```

- [ ] **Step 2: 运行 RED**

```bash
cd backend
go test -count=1 -tags=integration ./internal/repository -run TestTrafficCreditReservationRepository
```

Expected: FAIL。

- [ ] **Step 3: 定义 repository 接口**

```go
type TrafficCreditReservationRepository interface {
    GetAvailableUSD(ctx context.Context, userID int64, platform string, now time.Time) (float64, error)
    Reserve(ctx context.Context, input TrafficCreditReservationInput) (*TrafficCreditReservation, bool, error)
    MarkDispatched(ctx context.Context, reservationID int64) error
    MarkUnknown(ctx context.Context, reservationID int64, reason string) error
    Release(ctx context.Context, reservationID int64, now time.Time) error
    HasOutstandingDebt(ctx context.Context, userID int64, platform string) (bool, error)
}
```

- [ ] **Step 4: 实现 Reserve 事务**

事务按 `expires_at, credited_at, id FOR UPDATE` 查询：

```sql
SELECT id, user_id, order_id, pack_id, initial_usd, remaining_usd, reserved_usd, credited_at, expires_at
FROM user_traffic_credits
WHERE user_id = $1 AND platform = $2
  AND remaining_usd - reserved_usd > 0
  AND expires_at > $3
ORDER BY expires_at, credited_at, id
FOR UPDATE;
```

planner 覆盖后插入 reservation/items，并逐批次执行：

```sql
UPDATE user_traffic_credits
SET reserved_usd = reserved_usd + $1, updated_at = $2
WHERE id = $3 AND remaining_usd - reserved_usd + 0.0000000001 >= $1;
```

任一更新无行返回即回滚。幂等冲突规则与 usage fact 相同。

- [ ] **Step 5: 修改现有流量卡查询**

`HasAvailableCredit/GetSummary/listDeductibleCredits` 的内部可用判断统一考虑 `remaining_usd-reserved_usd`。用户页面 `TotalRemainingUSD` 仍汇总 `remaining_usd`，不得把短时预留显示成已消费。

- [ ] **Step 6: 注册 provider，运行 GREEN 并提交**

```bash
cd backend
go test -count=1 -tags=integration ./internal/repository -run 'TestTrafficCreditReservationRepository|TestTrafficPackRepository'
git add internal/repository/traffic_credit_reservation_repo.go internal/repository/traffic_credit_reservation_repo_integration_test.go internal/repository/traffic_pack_repo.go internal/repository/wire.go
git commit -m "feat: reserve traffic credit atomically"
```

Expected: PASS 后提交。

## Task 4：实现 OpenAI 保守预算估算

**Files:**

- Create: `backend/internal/service/openai_traffic_credit_budget.go`
- Create: `backend/internal/service/openai_traffic_credit_budget_test.go`
- Modify: `backend/internal/service/billing_service.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `deploy/config.example.yaml`

- [ ] **Step 1: 写预算测试**

```go
func TestOpenAITrafficCreditBudget_RejectsTinyResidual(t *testing.T) {
    estimator := newTestTrafficBudgetEstimator(0.01, 256, 8192)
    _, err := estimator.Estimate(context.Background(), OpenAITrafficBudgetInput{
        Model: "gpt-5.6-sol", Body: []byte(`{"input":"hello"}`), AvailableUSD: 0.00111155,
    })
    require.ErrorIs(t, err, ErrTrafficCreditInsufficient)
}

func TestOpenAITrafficCreditBudget_RejectsExplicitUnaffordableLimit(t *testing.T) {
    maxOutput := 8192
    estimator := newTestTrafficBudgetEstimator(0.01, 256, 8192)
    _, err := estimator.Estimate(context.Background(), OpenAITrafficBudgetInput{
        Model: "gpt-5.6-sol", Body: []byte(`{"input":"hello","max_output_tokens":8192}`),
        ExplicitMaxOutputTokens: &maxOutput, AvailableUSD: 0.02,
    })
    require.ErrorIs(t, err, ErrTrafficCreditInsufficient)
}

func TestOpenAITrafficCreditBudget_InjectsAffordableLimitWhenMissing(t *testing.T) {
    estimator := newTestTrafficBudgetEstimator(0.01, 256, 8192)
    got, err := estimator.Estimate(context.Background(), OpenAITrafficBudgetInput{
        Model: "gpt-5.6-sol", Body: []byte(`{"input":"hello"}`), AvailableUSD: 0.2,
    })
    require.NoError(t, err)
    require.GreaterOrEqual(t, got.EffectiveMaxOutputTokens, 256)
    require.LessOrEqual(t, got.ReserveUSD, 0.2)
    require.True(t, gjson.GetBytes(got.Body, "max_output_tokens").Exists())
}
```

- [ ] **Step 2: 运行 RED**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run TestOpenAITrafficCreditBudget
```

Expected: FAIL。

- [ ] **Step 3: 增加配置**

```go
TrafficCreditReservationEnabled bool `mapstructure:"traffic_credit_reservation_enabled"`
TrafficCreditReservationShadow bool `mapstructure:"traffic_credit_reservation_shadow"`
TrafficCreditMinimumReserveUSD float64 `mapstructure:"traffic_credit_minimum_reserve_usd"`
TrafficCreditMinimumOutputTokens int `mapstructure:"traffic_credit_minimum_output_tokens"`
TrafficCreditDefaultMaxOutputTokens int `mapstructure:"traffic_credit_default_max_output_tokens"`
TrafficCreditReservationTimeoutSeconds int `mapstructure:"traffic_credit_reservation_timeout_seconds"`
```

默认值：`false/true/0.01/256/8192/900`。金额不得为负，token 和 timeout 必须大于 0。

- [ ] **Step 4: 实现 estimator**

```go
type OpenAITrafficBudgetInput struct {
    Model string
    GroupID *int64
    ServiceTier string
    RateMultiplier float64
    Body []byte
    AvailableUSD float64
}

type OpenAITrafficBudget struct {
    Body []byte
    InputTokenUpperBound int
    EffectiveMaxOutputTokens int
    ReserveUSD float64
    PricingSnapshot json.RawMessage
}
```

规则：

1. 输入 token 上界使用最终出站 JSON 的 UTF-8 字节数；只允许已验证 byte-level tokenizer 的 OpenAI 模型。
2. 显式 `max_output_tokens/max_completion_tokens` 不自动缩小；预算不足直接拒绝。
3. 缺少输出上限时，从默认 8192 开始二分寻找可支付上限；低于 256 拒绝，并把结果写回对应 endpoint 字段。
4. `BillingService.EstimateMaximumTokenCost` 使用 input、cache creation 5m/1h 中最高输入侧单价，叠加 service tier、长上下文和 rate multiplier，不能使用 cache discount 乐观估算。
5. 无定价、无法证明 tokenizer 上界或 snapshot 序列化失败时返回 `ErrBillingPreauthUnavailable`。

- [ ] **Step 5: 运行 GREEN 并提交**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/config ./internal/service -run 'TestOpenAITrafficCreditBudget|Test.*Pricing'
git add internal/service/openai_traffic_credit_budget.go internal/service/openai_traffic_credit_budget_test.go internal/service/billing_service.go internal/config/config.go internal/config/config_test.go ../deploy/config.example.yaml
git commit -m "feat: estimate OpenAI traffic credit budget"
```

Expected: PASS 后提交。

## Task 5：固定请求级计费来源并接入预授权

**Files:**

- Create: `backend/internal/service/openai_billing_authorization.go`
- Create: `backend/internal/service/openai_billing_authorization_test.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/billing_cache_service.go`
- Modify: `backend/internal/service/effective_group_resolver.go`
- Modify: `backend/internal/service/wire.go`

- [ ] **Step 1: 写来源决策测试**

```go
func TestOpenAIBillingAuthorization_UsesSubscriptionWhenBudgetFits(t *testing.T)
func TestOpenAIBillingAuthorization_UsesBalanceWhenNoSubscriptionAndBalanceEligible(t *testing.T)
func TestOpenAIBillingAuthorization_ReservesTrafficCreditWhenSubscriptionExceeded(t *testing.T)
func TestOpenAIBillingAuthorization_RejectsOutstandingDebt(t *testing.T)
func TestOpenAIBillingAuthorization_ReusesReservationAcrossRetry(t *testing.T)
```

每个测试必须断言唯一 `BillingSource`，不能只断言无错误。

- [ ] **Step 2: 运行 RED**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run TestOpenAIBillingAuthorization
```

Expected: FAIL。

- [ ] **Step 3: 定义授权对象**

```go
type BillingSource string

const (
    BillingSourceSubscription BillingSource = "subscription"
    BillingSourceBalance BillingSource = "balance"
    BillingSourceTrafficCredit BillingSource = "traffic_credit"
)

type OpenAIBillingAuthorization struct {
    Source BillingSource
    ReservationID *int64
    ReserveUSD float64
    PricingSnapshot json.RawMessage
    EffectiveBody []byte
}
```

授权服务必须保持现有优先级：有效订阅且预算可覆盖时订阅；无订阅时余额资格正常则余额；订阅超限或余额不可用时才尝试流量卡预留。

- [ ] **Step 4: 在最终出站前授权**

在 `OpenAIGatewayService.Forward()` 完成模型映射和账号相关请求转换、调用 `httpUpstream.Do` 之前执行一次 `Authorize()`。使用网关生成的稳定 request ID 和请求指纹；把授权结果写入请求 context/gin context。账号重试和 failover 复用同一个 reservation，不得重复预留。

预留成功后、网络调用前执行 `MarkDispatched()`。网络调用前失败可 `Release()`；发出请求后发生 transport unknown 必须 `MarkUnknown()`，不得按 TTL 自动释放。

- [ ] **Step 5: 收紧早期资格检查**

`BillingCacheService.canUseTrafficPackCredit()` 和 `EffectiveGroupResolver` 可继续作为轻量早期过滤，但查询必须使用 `remaining_usd-reserved_usd > 0`，避免把已全部预留的批次当成可用。最低预留金额由最终预算授权检查；最终准入只认 reservation 成功，不能再以 `HasAvailableCredit()` 作为权威结论。

- [ ] **Step 6: 运行 GREEN 并提交**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestOpenAIBillingAuthorization|TestBillingCacheService|TestEffectiveGroupResolver'
git add internal/service/openai_billing_authorization.go internal/service/openai_billing_authorization_test.go internal/service/openai_gateway_service.go internal/service/billing_cache_service.go internal/service/effective_group_resolver.go internal/service/wire.go
git commit -m "feat: authorize OpenAI traffic credit before forwarding"
```

Expected: PASS 后提交。

## Task 6：把 reservation 贯穿 usage fact 和原子结算

**Files:**

- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/service/usage_fact.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Modify: `backend/internal/repository/usage_billing_repo_integration_test.go`
- Modify: `backend/internal/service/usage_fact_settlement_service.go`

- [ ] **Step 1: 写结算集成测试**

```go
func TestUsageBillingRepository_SettlesReservationAndReleasesRemainder(t *testing.T) {
    // reserve 1.00，actual 0.25；断言 remaining 减 0.25、reserved 归零、reservation settled_usd=0.25。
}

func TestUsageBillingRepository_ReservationDebtDoesNotRollbackUsageFact(t *testing.T) {
    // 模拟 actual 高于 reservation 且无额外可用额度；断言 reservation=debt、debt_usd>0，fact 仍存在。
}

func TestUsageBillingRepository_ReplayDoesNotDoubleSettleReservation(t *testing.T) {
    // 同 request 重放两次，remaining 只扣一次。
}
```

- [ ] **Step 2: 运行 RED**

```bash
cd backend
go test -count=1 -tags=integration ./internal/repository -run 'TestUsageBillingRepository_.*Reservation'
```

Expected: FAIL。

- [ ] **Step 3: 扩展 billing command/result**

```go
type UsageBillingCommand struct {
    // 保留现有字段
    TrafficCreditReservationID *int64
}

type UsageBillingApplyResult struct {
    // 保留现有字段
    TrafficCreditDebtUSD float64
}
```

`UsageFact` 索引列和 payload 同时保存 reservation ID；`BuildUsageFact` 直接使用请求前授权结果，不再调用 `shouldBillWithTrafficPack()` 重新判断。

- [ ] **Step 4: 原子结算 reservation**

`usage_billing_repo.Apply()` 在同一事务中：

1. 锁 reservation 和 items；
2. 校验 request、Key、fingerprint；
3. 按实际费用扣 `remaining_usd`，同时减少完整 `reserved_usd`；
4. 写真实 deduction ledger；
5. actual 小于预留时释放差额；
6. actual 大于预留时尝试补扣未预留可用额度；
7. 未覆盖部分写 `debt_usd` 并返回 `TrafficCreditDebtUSD`，不要返回 `ErrInsufficientBalance` 回滚事务。

所有更新必须带金额守卫，金额使用 `DECIMAL(20,10)`。

- [ ] **Step 5: settlement worker 推进 fact 状态**

`UsageFactSettlementService` 根据 `result.TrafficCreditDebtUSD` 标记 fact `debt`，否则 `settled`；无论哪种状态，`usage_logs` 都已投影。旧的无 reservation 流量卡 fact 继续保留 `ErrInsufficientBalance -> debt` 兼容路径。

- [ ] **Step 6: 运行 GREEN 并提交**

```bash
cd backend
go test -count=1 -tags=integration ./internal/repository -run 'TestUsageBillingRepository_.*Reservation|TestUsageFactRepository'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestUsageFactSettlementService|TestOpenAIGatewayService'
git add internal/service/usage_billing.go internal/service/usage_fact.go internal/service/openai_gateway_service.go internal/repository/usage_billing_repo.go internal/repository/usage_billing_repo_integration_test.go internal/service/usage_fact_settlement_service.go
git commit -m "feat: settle reserved traffic credit atomically"
```

Expected: PASS 后提交。

## Task 7：实现 debt gate、异常释放和观测

**Files:**

- Modify: `backend/internal/service/openai_billing_authorization.go`
- Modify: `backend/internal/repository/traffic_credit_reservation_repo.go`
- Modify: `backend/internal/service/usage_fact_worker.go`
- Modify: `backend/internal/service/ops_service.go`
- Modify: relevant unit/integration tests

- [ ] **Step 1: 写 debt gate 和 stale reservation 测试**

```go
func TestOpenAIBillingAuthorization_BlocksTrafficCreditWhenDebtExists(t *testing.T)
func TestTrafficCreditReservationRepository_ReleasesUndispatchedExpiredReservation(t *testing.T)
func TestTrafficCreditReservationRepository_DoesNotReleaseDispatchedExpiredReservation(t *testing.T)
```

- [ ] **Step 2: 实现阻断和清理规则**

授权前查询 `HasOutstandingDebt(userID, openai)`；存在 debt 返回 `ErrBillingDebtOutstanding`。定时 reconciliation 只释放 `reserved` 且从未 dispatched 的过期记录；`dispatched/unknown` 只报警和人工对账，禁止按 TTL 自动释放。

- [ ] **Step 3: 增加指标**

至少暴露：预授权成功/拒绝数、shadow estimate 与 actual 差值、reservation 各状态数量、最老 pending/unknown 年龄、debt 用户数、`actual > reserved` 次数。日志带 request/user/key/reservation/fact ID，不记录完整 Key 或请求内容。

- [ ] **Step 4: 运行测试并提交**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'Debt|Reservation|Authorization'
go test -count=1 -tags=integration ./internal/repository -run 'Debt|Reservation'
git add internal/service/openai_billing_authorization.go internal/repository/traffic_credit_reservation_repo.go internal/service/usage_fact_worker.go internal/service/ops_service.go
git commit -m "feat: block traffic credit requests with debt"
```

Expected: PASS 后提交。

## Task 8：Shadow、强制启用和完整验证

**Files:**

- Modify: `backend/cmd/server/wire_gen.go`
- Create: `docs/ai/context/20260715-093000-traffic-credit-reservation-debt-gate-result_CN.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: 生成 Wire**

```bash
cd backend/cmd/server
go generate ./...
```

- [ ] **Step 2: 运行目标回归**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/config ./internal/service ./internal/handler -run 'TrafficCredit|Reservation|BillingAuthorization|UsageFact'
go test -count=1 -tags=integration ./internal/repository -run 'TrafficCredit|Reservation|UsageBilling|MigrationsRunner'
```

Expected: PASS。

- [ ] **Step 3: 运行完整 service/handler 和 server 编译**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler
go test -count=1 ./cmd/server
git diff --check
```

Expected: 全部 PASS，`git diff --check` 无输出。

- [ ] **Step 4: Shadow 验证**

首次部署配置：

```yaml
billing:
  traffic_credit_reservation_enabled: false
  traffic_credit_reservation_shadow: true
```

至少观察一个完整高峰周期，确认目标模型的 `actual_cost <= estimated_reserve_usd`，且无 tokenizer/定价缺失。Shadow 只记录，不改变现有准入。

- [ ] **Step 5: 小范围强制启用**

先对内部 Key 或白名单用户开启 reservation；验证：套餐到期且流量卡足够仍 200，`0.00111155 USD` 请求前 402，并发预留不超卖，fact 和 ledger 可对账。未通过不得全量开启。

- [ ] **Step 6: 全量启用并写结果文档**

结果文档记录配置、迁移、shadow 差值、canary 证据、错误码、测试命令、回退方式和是否部署。`AGENTS.md` 追加最终定论。

- [ ] **Step 7: 提交文档**

```bash
git add backend/cmd/server/wire_gen.go docs/ai/context/20260715-093000-traffic-credit-reservation-debt-gate-result_CN.md AGENTS.md
git commit -m "docs: record traffic credit reservation rollout"
```

## 完成条件

- 套餐到期不会停用 API Key；余额不可用但流量卡预算足够时仍可请求。
- `0.00111155 USD` 或任何不足以形成最低有效预算的残额在上游前返回 402。
- 并发请求的 reservation 总额不超过流量卡真实剩余额度。
- 请求前确定的 `BillingSource` 和 reservation ID 贯穿 usage fact 与结算，不再响应后重新判断。
- 实际费用低于预留时差额释放；高于预留时写 debt，不丢 fact 和 usage log。
- debt 未结清时新的流量卡请求被拒绝。
- `dispatched/unknown` reservation 不会因简单 TTL 被错误释放。
- 生产开启前已完成 shadow 和 canary，且有可执行的配置回退路径。

## 设计依据

- `docs/ai/context/20260715-090554-traffic-credit-preauthorization-and-durable-usage-design_CN.md`
- `docs/ai/context/20260715-092959-openai-usage-fact-durable-outbox-implementation-plan_CN.md`
- `docs/ai/context/20260714-204422-expired-user-request-billing-gap-code-cause_CN.md`
