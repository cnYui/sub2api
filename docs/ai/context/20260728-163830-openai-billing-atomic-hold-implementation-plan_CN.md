# OpenAI 原子计费授权与本地校准实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为所有 OpenAI 入口建立请求前唯一授权、套餐与流量卡 PostgreSQL 原子 hold、固定资金来源结算、图片/PDF 预算、5 分钟 unknown 核销，并在隔离的 `18081 -> 18087` 双层环境用真实请求校准估算公式。

**Architecture:** 请求体先在事务外解析为不可变、版本化的 `OpenAIBillingBudgetPlan`；授权仓储在单个 PostgreSQL 事务内锁定套餐或流量卡、读取原子可用额度、执行纯函数预算适配并创建 `billing_authorizations`。响应后的 usage fact 必须引用同一个 authorization，结算事务只能消费已固定的资金来源；不完整 usage 进入 unknown，30 秒轮询，5 分钟后转平台 suspense 并释放用户额度。

**Tech Stack:** Go 1.26.4、Gin、PostgreSQL 18、Redis 8、Docker Compose、Testcontainers、`github.com/ledongthuc/pdf v0.0.0-20250511090121-5959a4027728`、PowerShell、Node.js 静态编排校验。

---

## 固定范围

- 本次完成设计文档中的 P0、P1。
- 仅开发和验证本地候选链路，不部署公网，不执行公网 migration，不重启 `18080/18086`。
- 不处理历史 74 条冻结 reservation 和 2902 条 debt。
- 视频输入不支持；请求前明确拒绝，不按零成本放行。
- OpenAI 模型计费不使用账户余额；购买、充值、退款及非 OpenAI 计费路径保持现状。
- 单请求硬上限是 2 USD，但实际 hold 为预算值与单一资金来源原子可用额度共同决定的结果。
- 不允许套餐与流量卡混合；一次流量卡授权可以按到期顺序占用多张流量卡。
- 不改动或提交 `frontend/src/views/HomeView.vue`、`frontend/src/views/__tests__/HomeView.spec.ts` 及其他用户原有修改。

## 文件责任与边界

### 新建

- `backend/migrations/180_openai_billing_authorizations.sql`：保留历史数据地把流量卡 reservation 演进为通用 authorization，并增加 source、估算、suspense、时间戳和索引。
- `backend/migrations/openai_billing_authorizations_regression_test.go`：校验 migration 不会丢历史记录或产生旧列双写。
- `backend/internal/service/billing_authorization.go`：authorization 领域类型、状态、仓储接口、状态转换错误。
- `backend/internal/service/openai_billing_budget.go`：文本/图片/PDF/输出预算计划、2 USD 上限和纯函数适配。
- `backend/internal/service/openai_billing_budget_test.go`：预算、倍率、动态额度、混合输入测试。
- `backend/internal/service/openai_attachment_inspector.go`：附件解析接口、图片元数据和 PDF 检查结果。
- `backend/internal/service/openai_attachment_inspector_test.go`：内联图像、PDF、超限和不可解析输入测试。
- `backend/internal/service/openai_billing_finalizer.go`：派发前释放、派发标记、transport unknown、终态 usage 处理的统一入口。
- `backend/internal/service/openai_billing_reconciliation.go`：unknown claim、核销、超时 suspense 服务。
- `backend/internal/service/openai_billing_reconciliation_test.go`：30 秒轮询与 5 分钟超时状态测试。
- `backend/internal/service/billing_authorization_metrics.go`、`billing_authorization_metrics_test.go`：按 source/status 记录授权状态，不记录敏感信息。
- `backend/internal/repository/billing_authorization_repo.go`：套餐和流量卡的事务内原子 hold、状态转换、结算和 suspense。
- `backend/internal/repository/billing_authorization_repo_integration_test.go`：PostgreSQL 并发、结算、释放和债务集成测试。
- `deploy/docker-compose.openai-billing-candidate.yml`：外层 18081、内层 18087、两套 PostgreSQL 和两套空 Redis。
- `deploy/.env.openai-billing-candidate.local.example`：只包含候选环境变量名和无效示例值。
- `deploy/sql/openai-billing-candidate-sanitize.sql`：同时用于两套克隆库，关闭支付、邮件、通知、监控等外部副作用。
- `deploy/sql/openai-billing-candidate-outer-route.sql`：只用于外层克隆库，禁用其他 OpenAI 上游并把唯一内部上游改到候选内层。
- `deploy/openai-billing-candidate.ps1`：备份、校验 dump、恢复、启动、停止和健康检查。
- `deploy/verify-openai-billing-candidate.mjs`：静态校验端口、网络、容器名、Redis 隔离和副作用开关。
- `backend/internal/integration/openai_billing_gateway_test.go`：全入口 mock upstream、15 并发和故障注入验收。
- `docs/ai/context/<执行时间>-openai-billing-calibration-result_CN.md`：真实请求估算与实际 usage 对比结果。

### 修改

- `backend/internal/service/openai_billing_authorization.go`：只负责编排“完整套餐、完整流量卡、可缩减套餐、可缩减流量卡、402”。
- `backend/internal/service/openai_gateway_service.go`：统一 authorization state、usage fact 强制关联、删除 OpenAI 响应后选源。
- `backend/internal/service/openai_gateway_messages.go`：Anthropic Messages 转换完成后授权，递归重试复用 authorization。
- `backend/internal/service/openai_embeddings.go`：输入预算、请求前授权和统一 finalizer。
- `backend/internal/service/openai_images.go`、`openai_images_responses.go`：复用图片元数据预算，资金不足时只减少允许减少的 `n`。
- `backend/internal/service/openai_ws_forwarder.go`、`backend/internal/handler/openai_gateway_handler.go`：WebSocket 每个 `response.create` turn 独立授权。
- `backend/internal/service/usage_billing.go`：把流量卡专用 ID 改为通用 `AuthorizationID`。
- `backend/internal/service/usage_fact.go`、`backend/internal/repository/usage_fact_repo.go`：usage fact 强制引用 authorization，并支持按 authorization 查询核销证据。
- `backend/internal/service/usage_fact_settlement_service.go`、`backend/internal/repository/usage_billing_repo.go`：固定来源结算，禁止 OpenAI 余额扣费和流量卡追加扣款。
- `backend/internal/service/usage_fact_worker.go`：接入 reconciliation，保留 pending fact 重试。
- `backend/internal/config/config.go`、`backend/internal/config/config_test.go`、`deploy/config.example.yaml`：通用 authorization 配置。
- `backend/internal/service/wire.go`、`backend/internal/repository/wire.go`、`backend/cmd/server/wire_gen.go`：替换依赖注入。
- `backend/internal/server/routes/gateway.go`：显式注册 `/v1/responses/input_tokens` 的 501 handler。
- 相关现有单元测试和集成测试：更新接口名称并增加固定来源断言。
- `backend/go.mod`、`backend/go.sum`：加入 PDF 解析依赖。

### 删除

- `backend/internal/service/openai_traffic_credit_budget.go`
- `backend/internal/service/openai_traffic_credit_budget_test.go`
- `backend/internal/service/traffic_credit_reservation.go`
- `backend/internal/service/traffic_credit_reservation_metrics.go`
- `backend/internal/service/traffic_credit_reservation_metrics_test.go`
- `backend/internal/repository/traffic_credit_reservation_repo.go`
- `backend/internal/repository/traffic_credit_reservation_repo_integration_test.go`

这些旧文件只在通用替代实现和对应测试全部通过后删除；数据库历史数据通过 migration 原地演进，不做物理清理。

## 核心类型契约

后续任务统一使用以下名称，禁止在不同入口继续引入 `ReservationID` 或 `TrafficCreditReservationID`：

```go
type BillingSource string

const (
	BillingSourceSubscription  BillingSource = "subscription"
	BillingSourceTrafficCredit BillingSource = "traffic_credit"
)

var ErrBillingAuthorizationDebtOutstanding = errors.New("billing authorization debt is outstanding")

type BillingAuthorizationStatus string

const (
	BillingAuthorizationReserved   BillingAuthorizationStatus = "reserved"
	BillingAuthorizationDispatched BillingAuthorizationStatus = "dispatched"
	BillingAuthorizationUnknown    BillingAuthorizationStatus = "unknown"
	BillingAuthorizationSettled    BillingAuthorizationStatus = "settled"
	BillingAuthorizationReleased   BillingAuthorizationStatus = "released"
	BillingAuthorizationDebt       BillingAuthorizationStatus = "debt"
	BillingAuthorizationSuspense   BillingAuthorizationStatus = "suspense"
)

type BillingAuthorization struct {
	ID                    int64
	RequestID             string
	APIKeyID              int64
	UserID                int64
	Source                BillingSource
	SubscriptionID        *int64
	EntitlementPeriodID   *int64
	RequestFingerprint    string
	ReservedUSD           float64
	SettledUSD            float64
	DebtUSD               float64
	SuspenseUSD           float64
	PricingSnapshot       json.RawMessage
	EstimateBreakdown     json.RawMessage
	EstimatorVersion      string
	Status                BillingAuthorizationStatus
	EffectiveBody         []byte
	EffectiveImageCount   int
	ExpiresAt             time.Time
	DispatchedAt          *time.Time
	SettledAt             *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
```

## Task 1：创建隔离 worktree 并锁定基线

**Files:**
- Copy: `docs/ai/context/20260728-162444-openai-billing-atomic-hold-final-design_CN.md`
- Copy: `docs/ai/context/20260728-163830-openai-billing-atomic-hold-implementation-plan_CN.md`
- Modify: `AGENTS.md`，只带入本任务已确认的计费定论，不带入其他工作树内容

- [ ] **Step 1：按技能创建隔离 worktree**

执行时先完整读取 `superpowers:using-git-worktrees`，然后运行：

```powershell
git rev-parse --git-dir
git rev-parse --git-common-dir
git branch --show-current
git worktree add D:\CodeWorkSpace\sub2api-openai-billing -b codex/openai-billing-atomic-hold HEAD
```

Expected：新 worktree 分支为 `codex/openai-billing-atomic-hold`，原 `D:\CodeWorkSpace\sub2api` 的未提交前端文件保持原样。

- [ ] **Step 2：复制本任务文档并检查差异边界**

```powershell
Copy-Item D:\CodeWorkSpace\sub2api\docs\ai\context\20260728-162444-openai-billing-atomic-hold-final-design_CN.md D:\CodeWorkSpace\sub2api-openai-billing\docs\ai\context\
Copy-Item D:\CodeWorkSpace\sub2api\docs\ai\context\20260728-163830-openai-billing-atomic-hold-implementation-plan_CN.md D:\CodeWorkSpace\sub2api-openai-billing\docs\ai\context\
git status --short
```

Expected：只出现两份计费文档和精确挑选的 `AGENTS.md` 计费记忆，不出现 `frontend/src/views/HomeView.vue`。

- [ ] **Step 3：运行后端基线测试**

Run：

```powershell
Set-Location D:\CodeWorkSpace\sub2api-openai-billing\backend
go test -tags=unit ./internal/service ./internal/repository ./internal/server -count=1
```

Expected：PASS；若基线本身失败，先把完整命令和失败用例写入新的上下文文档，不能把基线失败归因于本任务。

- [ ] **Step 4：只提交设计和计划文档**

```powershell
git add AGENTS.md docs/ai/context/20260728-162444-openai-billing-atomic-hold-final-design_CN.md docs/ai/context/20260728-163830-openai-billing-atomic-hold-implementation-plan_CN.md
git commit -m "docs: 固化 OpenAI 原子计费设计与实施计划"
```

Expected：提交不包含前端文件、密钥、dump 或候选 `.env`。

## Task 2：建立双层候选环境和可恢复数据库副本

**Files:**
- Create: `deploy/docker-compose.openai-billing-candidate.yml`
- Create: `deploy/.env.openai-billing-candidate.local.example`
- Create: `deploy/sql/openai-billing-candidate-sanitize.sql`
- Create: `deploy/sql/openai-billing-candidate-outer-route.sql`
- Create: `deploy/openai-billing-candidate.ps1`
- Create: `deploy/verify-openai-billing-candidate.mjs`
- Test: `deploy/verify-openai-billing-candidate.mjs`

- [ ] **Step 1：先写会失败的静态编排校验**

校验器必须读取 compose、env example、common sanitize SQL 和 outer route SQL，并断言：

```js
const requiredServices = [
  "billing-outer",
  "billing-outer-postgres",
  "billing-outer-redis",
  "billing-inner",
  "billing-inner-postgres",
  "billing-inner-redis",
];

assert.deepEqual(Object.keys(compose.services).sort(), requiredServices.sort());
assert.equal(compose.services["billing-outer"].ports[0], "127.0.0.1:18081:8080");
assert.equal(compose.services["billing-inner"].ports[0], "127.0.0.1:18087:8080");
assert.ok(!rawCompose.includes("sub2api-dev"));
assert.ok(!rawCompose.includes("sub2api-upstream-latest"));
assert.ok(!rawCompose.includes("nginx"));
assert.ok(rawSanitize.includes("payment_enabled', 'false'"));
assert.ok(rawOuterRoute.includes("http://billing-inner:8080/v1"));
assert.ok(rawOuterRoute.includes("schedulable = false"));
```

- [ ] **Step 2：运行校验并确认失败**

Run：`node deploy/verify-openai-billing-candidate.mjs`

Expected：FAIL，提示候选 compose 或 sanitize 文件不存在。

- [ ] **Step 3：实现六服务 compose 和副作用隔离 SQL**

compose 必须满足：

```yaml
name: sub2api-openai-billing-candidate

services:
  billing-outer:
    ports: ["127.0.0.1:18081:8080"]
    depends_on:
      billing-outer-postgres: { condition: service_healthy }
      billing-outer-redis: { condition: service_healthy }
  billing-inner:
    ports: ["127.0.0.1:18087:8080"]
    depends_on:
      billing-inner-postgres: { condition: service_healthy }
      billing-inner-redis: { condition: service_healthy }
```

两个 Redis 都使用空目录、`--save "" --appendonly no`；外层和内层 PostgreSQL 使用不同目录；只创建候选网络。common sanitize SQL 必须关闭支付、退款入口、SMTP、通知、渠道监控和外部备份上传。outer route SQL 先定位当前指向 18086 的内部账号，禁用外层其他 OpenAI 账号，再执行：

```sql
UPDATE accounts
SET credentials = jsonb_set(credentials, '{base_url}', '"http://billing-inner:8080/v1"'::jsonb, true),
    status = 'active',
    schedulable = true,
    updated_at = NOW()
WHERE deleted_at IS NULL
  AND platform = 'openai'
  AND credentials->>'base_url' LIKE '%18086%';
```

禁用语句只在外层执行：

```sql
UPDATE accounts
SET schedulable = false, updated_at = NOW()
WHERE deleted_at IS NULL
  AND platform = 'openai'
  AND COALESCE(credentials->>'base_url', '') NOT LIKE '%18086%';
```

- [ ] **Step 4：实现 PowerShell 的 Backup、Restore、Up、Down、Verify 动作**

脚本使用固定来源容器 `sub2api-postgres-dev`、`sub2api-upstream-postgres`，dump 输出到候选 worktree 的 `deploy/openai-billing-candidate/dumps/`；每次递归目录操作前用 `Resolve-Path` 检查目标位于该候选目录。dump 校验必须执行：

```powershell
docker run --rm -v "${DumpDir}:/dumps:ro" postgres:18-alpine pg_restore --list "/dumps/$DumpName"
if ($LASTEXITCODE -ne 0) { throw "候选数据库 dump 无法读取" }
```

脚本不得把任何 API Key 写入参数默认值、文件或日志。

- [ ] **Step 5：运行静态校验并创建双库备份**

```powershell
node deploy/verify-openai-billing-candidate.mjs
.\deploy\openai-billing-candidate.ps1 -Action Backup
.\deploy\openai-billing-candidate.ps1 -Action Restore
```

Expected：静态校验 PASS；两份 dump 均能 `pg_restore --list`；恢复目标容器名只以 `billing-outer-` 或 `billing-inner-` 开头；`docker ps` 中 `18080/18086` 容器启动时间不变。

- [ ] **Step 6：提交候选编排**

```powershell
git add deploy/docker-compose.openai-billing-candidate.yml deploy/.env.openai-billing-candidate.local.example deploy/sql/openai-billing-candidate-sanitize.sql deploy/sql/openai-billing-candidate-outer-route.sql deploy/openai-billing-candidate.ps1 deploy/verify-openai-billing-candidate.mjs
git commit -m "test: 增加 OpenAI 计费双层候选环境"
```

## Task 3：迁移通用 authorization 表和领域契约

**Files:**
- Create: `backend/migrations/180_openai_billing_authorizations.sql`
- Create: `backend/migrations/openai_billing_authorizations_regression_test.go`
- Create: `backend/internal/service/billing_authorization.go`
- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/service/usage_fact.go`

- [ ] **Step 1：写 migration 回归失败测试**

测试必须断言 migration 包含下列结构：

```go
func TestMigration180EvolvesReservationsWithoutDroppingHistory(t *testing.T) {
	raw, err := FS.ReadFile("180_openai_billing_authorizations.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "ALTER TABLE traffic_credit_reservations RENAME TO billing_authorizations")
	require.Contains(t, sql, "billing_source")
	require.Contains(t, sql, "estimate_breakdown")
	require.Contains(t, sql, "estimator_version")
	require.Contains(t, sql, "suspense_usd")
	require.Contains(t, sql, "authorization_id")
	require.NotContains(t, sql, "DROP TABLE traffic_credit_reservations")
}
```

- [ ] **Step 2：运行测试并确认失败**

Run：`go test ./migrations -run TestMigration180 -count=1`

Expected：FAIL，提示 migration 文件不存在。

- [ ] **Step 3：实现幂等 migration**

migration 需要：

```sql
DO $$
BEGIN
  IF to_regclass('public.billing_authorizations') IS NULL
     AND to_regclass('public.traffic_credit_reservations') IS NOT NULL THEN
    ALTER TABLE traffic_credit_reservations RENAME TO billing_authorizations;
  END IF;
  IF to_regclass('public.billing_authorization_traffic_credit_items') IS NULL
     AND to_regclass('public.traffic_credit_reservation_items') IS NOT NULL THEN
    ALTER TABLE traffic_credit_reservation_items RENAME TO billing_authorization_traffic_credit_items;
  END IF;
END $$;

ALTER TABLE billing_authorizations
  ADD COLUMN IF NOT EXISTS billing_source VARCHAR(20) NOT NULL DEFAULT 'traffic_credit',
  ADD COLUMN IF NOT EXISTS subscription_id BIGINT,
  ADD COLUMN IF NOT EXISTS entitlement_period_id BIGINT,
  ADD COLUMN IF NOT EXISTS estimate_breakdown JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS estimator_version VARCHAR(64) NOT NULL DEFAULT 'legacy-traffic-credit-v1',
  ADD COLUMN IF NOT EXISTS suspense_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS dispatched_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS settled_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS reconciled_at TIMESTAMPTZ;
```

状态约束允许 `reserved/dispatched/unknown/settled/released/debt/suspense`。旧记录保持 `traffic_credit`。将 `usage_facts.reservation_id` 原地改名为 `authorization_id`，增加：

```sql
CREATE INDEX IF NOT EXISTS idx_billing_authorizations_subscription_active
ON billing_authorizations (subscription_id, entitlement_period_id, id)
WHERE billing_source = 'subscription' AND status IN ('reserved', 'dispatched', 'unknown');

CREATE INDEX IF NOT EXISTS idx_billing_authorizations_unknown_reconcile
ON billing_authorizations (updated_at, id)
WHERE status = 'unknown';
```

- [ ] **Step 4：定义通用仓储接口和命令字段**

`billing_authorization.go` 至少定义：

```go
type BillingAuthorizationReserveMode string

const (
	BillingAuthorizationReserveFull BillingAuthorizationReserveMode = "full"
	BillingAuthorizationReserveFit  BillingAuthorizationReserveMode = "fit"
)

type BillingAuthorizationReserveInput struct {
	RequestID           string
	APIKeyID            int64
	UserID              int64
	Platform            string
	Model               string
	RequestFingerprint  string
	Source               BillingSource
	SubscriptionID       *int64
	EntitlementPeriodID  *int64
	BudgetPlan           OpenAIBillingBudgetPlan
	Mode                 BillingAuthorizationReserveMode
	ExpiresAt            time.Time
}

type BillingAuthorizationRepository interface {
	HasOutstandingDebt(context.Context, int64, string) (bool, error)
	TryReserve(context.Context, BillingAuthorizationReserveInput) (*BillingAuthorization, bool, error)
	MarkDispatched(context.Context, int64, time.Time) error
	MarkUnknown(context.Context, int64, string, time.Time) error
	Release(context.Context, int64, string, time.Time) error
	ClaimUnknown(context.Context, time.Time, int) ([]BillingAuthorization, error)
	MoveUnknownToSuspense(context.Context, int64, string, time.Time) error
}
```

`UsageBillingCommand` 把 `TrafficCreditReservationID` 替换为：

```go
AuthorizationID  *int64
OpenAIActualCost float64
```

`UsageFact` 同样只保留 `AuthorizationID *int64`。

- [ ] **Step 5：运行 migration 和领域测试**

```powershell
go test ./migrations -run TestMigration180 -count=1
go test -tags=unit ./internal/service -run 'TestUsageFact|TestUsageBilling' -count=1
go test -tags=integration ./internal/repository -run TestMigrationsSchema -count=1
```

Expected：PASS；集成库存在新表名、新列和索引，旧表数据条数与迁移前一致。

- [ ] **Step 6：提交 schema 和领域契约**

```powershell
git add backend/migrations/180_openai_billing_authorizations.sql backend/migrations/openai_billing_authorizations_regression_test.go backend/internal/service/billing_authorization.go backend/internal/service/usage_billing.go backend/internal/service/usage_fact.go
git commit -m "feat: 演进通用计费授权模型"
```

## Task 4：实现版本化价格快照和文本预算纯函数

**Files:**
- Create: `backend/internal/service/openai_billing_budget.go`
- Create: `backend/internal/service/openai_billing_budget_test.go`
- Modify: `backend/internal/service/pricing_service.go`
- Modify: `backend/internal/service/billing_service.go`
- Test: `backend/internal/service/openai_billing_budget_test.go`

- [ ] **Step 1：写文本预算失败测试**

至少覆盖：

```go
func TestFitOpenAIBillingBudgetCapsAtTwoUSD(t *testing.T) {
	plan := testTextBudgetPlan(3.00, false)
	_, err := FitOpenAIBillingBudget(plan, 5.00, BillingAuthorizationReserveFull)
	require.ErrorIs(t, err, ErrOpenAIBillingBudgetExceedsHardCap)
}

func TestFitOpenAIBillingBudgetUsesExactOneDollarAvailability(t *testing.T) {
	plan := testAdjustableTextBudgetPlan(0.10, 0.00003, 256, 128000)
	got, err := FitOpenAIBillingBudget(plan, 1.00, BillingAuthorizationReserveFit)
	require.NoError(t, err)
	require.LessOrEqual(t, got.ReserveUSD, 1.00)
	require.GreaterOrEqual(t, got.EffectiveMaxOutputTokens, 256)
}

func TestFitOpenAIBillingBudgetDoesNotRewriteExplicitLimit(t *testing.T) {
	plan := testTextBudgetPlan(1.20, true)
	_, err := FitOpenAIBillingBudget(plan, 1.00, BillingAuthorizationReserveFit)
	require.ErrorIs(t, err, ErrOpenAIBillingBudgetInsufficient)
}
```

再加入 `0.47 USD`、Embeddings 只有输入成本、tools/schema 协议开销、输入成本已超过可用额度、最低 256 Token 不可执行等用例。

- [ ] **Step 2：写倍率只应用一次的失败测试**

测试从 `deploy/data/model_pricing.json` 对应的有效价格构造 `BillingService`，断言：

```go
require.Equal(t, 5.0/1_000_000, snapshotFor("gpt-5.5").InputUSDPerToken)
require.Equal(t, 30.0/1_000_000, snapshotFor("gpt-5.5").OutputUSDPerToken)
require.Equal(t, 6.25/1_000_000, snapshotFor("gpt-5.6-sol").InputUSDPerToken)
require.Equal(t, 37.5/1_000_000, snapshotFor("gpt-5.6-sol").OutputUSDPerToken)
require.Equal(t, 10.0/1_000_000, snapshotFor("gpt-image-2").ImageInputUSDPerToken)
require.Equal(t, 37.5/1_000_000, snapshotFor("gpt-image-2").ImageOutputUSDPerToken)
```

价格文件已经保存销售侧有效价格，预算器和结算器直接使用该有效价格；不得再次按模型名乘 2 或 2.5。`PricingSnapshot` 记录价格来源哈希、模型名、service tier、group rate multiplier 和最终每 Token 单价。

- [ ] **Step 3：运行测试并确认失败**

Run：`go test -tags=unit ./internal/service -run 'TestFitOpenAIBillingBudget|TestOpenAIBillingPricingSnapshot' -count=1`

Expected：FAIL，提示预算计划或适配函数不存在。

- [ ] **Step 4：实现不可变预算计划和适配结果**

```go
const (
	OpenAIBillingEstimatorVersion = "openai-local-v1"
	OpenAIBillingHardCapUSD       = 2.0
	OpenAIBillingMinOutputTokens  = 256
)

type OpenAIBillingBudgetPlan struct {
	OriginalBody            []byte
	OutputLimitField        string
	ExplicitOutputLimit     bool
	RequestedOutputTokens   int
	DefaultOutputTokens     int
	MinimumOutputTokens     int
	FixedInputUSD           float64
	OutputUSDPerToken       float64
	ImageOutputUSDPerImage  []float64
	PricingSnapshot         json.RawMessage
	EstimateBreakdown       OpenAIBillingEstimateBreakdown
}

type OpenAIBillingBudgetFit struct {
	EffectiveBody       []byte
	EffectiveImageCount int
	ReserveUSD          float64
	PricingSnapshot     json.RawMessage
	EstimateBreakdown   json.RawMessage
}
```

适配算法固定为：

```go
capUSD := math.Min(OpenAIBillingHardCapUSD, availableUSD)
if plan.FullCostUSD() <= capUSD { return plan.FullFit(), nil }
if mode == BillingAuthorizationReserveFull { return OpenAIBillingBudgetFit{}, ErrOpenAIBillingBudgetInsufficient }
if plan.ExplicitOutputLimit { return OpenAIBillingBudgetFit{}, ErrOpenAIBillingBudgetInsufficient }
return fitAdjustableOutputAndImageCount(plan, capUSD)
```

文本 Token 计算必须计入 JSON 协议结构、消息角色、tools、JSON schema、函数名和参数；跳过 base64 图片/PDF 原始字节，但不能跳过其解析后的预算组件。

- [ ] **Step 5：运行预算测试**

Run：`go test -tags=unit ./internal/service -run 'TestFitOpenAIBillingBudget|TestOpenAIBillingPricingSnapshot|TestEstimateOpenAI' -count=1`

Expected：PASS；所有结果 `ReserveUSD <= min(2, availableUSD)`，显式输出上限从不被重写。

- [ ] **Step 6：提交预算基础**

```powershell
git add backend/internal/service/openai_billing_budget.go backend/internal/service/openai_billing_budget_test.go backend/internal/service/pricing_service.go backend/internal/service/billing_service.go
git commit -m "feat: 增加 OpenAI 版本化请求预算"
```

## Task 5：实现图片输入、PDF 和混合请求预算

**Files:**
- Create: `backend/internal/service/openai_attachment_inspector.go`
- Create: `backend/internal/service/openai_attachment_inspector_test.go`
- Modify: `backend/internal/service/openai_billing_budget.go`
- Modify: `backend/internal/service/openai_billing_budget_test.go`
- Modify: `backend/internal/service/openai_images.go`
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`

- [ ] **Step 1：写图片和 PDF 失败测试**

测试矩阵必须包含：

```go
func TestInspectOpenAIAttachmentsCountsImageDimensionsDetailAndQuantity(t *testing.T) {
	body := responseBodyWithPNG(1024, 768, "high", 2)
	got, err := inspector.Inspect(context.Background(), body)
	require.NoError(t, err)
	require.Len(t, got.Images, 2)
	require.Equal(t, 1024, got.Images[0].Width)
	require.Equal(t, 768, got.Images[0].Height)
	require.Equal(t, "high", got.Images[0].Detail)
}

func TestInspectOpenAIAttachmentsPricesPDFTextAndEveryPageVision(t *testing.T) {
	body := responseBodyWithInlinePDF(twoPagePDFWithText("hello", "world"))
	got, err := inspector.Inspect(context.Background(), body)
	require.NoError(t, err)
	require.Equal(t, 2, got.PDFs[0].PageCount)
	require.Greater(t, got.PDFs[0].TextTokens, 0)
	require.Len(t, got.PDFs[0].Pages, 2)
}

func TestInspectOpenAIAttachmentsRejectsUnresolvableFileID(t *testing.T) {
	_, err := inspector.Inspect(context.Background(), []byte(`{"input":[{"type":"input_file","file_id":"file_1"}]}`))
	require.ErrorIs(t, err, ErrOpenAIAttachmentNotLocallyReadable)
}
```

再覆盖：图片 low/high/auto、多个图片、图生图、PDF 解析失败、PDF 超 20 MiB、超过 200 页、解析超过 2 秒、视频 MIME、历史图片字段为 0 但请求体有图片/PDF。

图片 URL 用 `httptest.Server` 覆盖正常下载、超 20 MiB、超时和私网/非法地址拦截；实现时复用现有安全 URL 校验与受限下载器。multipart 上传和 data URL 直接从已读取的受限字节提取尺寸；无法取得尺寸的远程图片请求前失败，不能用未知零成本继续。

- [ ] **Step 2：运行测试并确认失败**

Run：`go test -tags=unit ./internal/service -run 'TestInspectOpenAIAttachments|TestOpenAIBillingBudget.*Image|TestOpenAIBillingBudget.*PDF' -count=1`

Expected：FAIL，提示 inspector 不存在。

- [ ] **Step 3：加入 PDF 依赖并实现隔离接口**

Run：

```powershell
go get github.com/ledongthuc/pdf@v0.0.0-20250511090121-5959a4027728
```

接口不得暴露第三方 PDF 类型：

```go
type OpenAIAttachmentInspector interface {
	Inspect(context.Context, []byte) (OpenAIAttachmentInspection, error)
}

type OpenAIPDFInspection struct {
	Text       string
	TextTokens int
	PageCount  int
	Pages      []OpenAIImageInput
}

type OpenAIImageInput struct {
	Width  int
	Height int
	Detail string
}
```

内联 `file_data` 支持原始 base64 和 data URL。当前代理没有 Files 内容读取能力，因此 `file_id` 请求前返回明确的 400 `attachment_not_locally_readable`；不得转发后按零成本计费。

- [ ] **Step 4：实现官方规则的版本化附件预算**

图片输入根据模型规则、尺寸、`detail` 和数量生成 Token 上界；PDF 预算固定为：

```go
pdfInputTokens := pdf.TextTokens
for _, page := range pdf.Pages {
	pdfInputTokens += estimateOpenAIImageInputTokens(model, page)
}
```

独立生图和图生图使用现有 `gptImage2OutputTokenUpperBounds` 的 size/quality 上界；未知或 auto 继续使用保守上界。多图费用按每张独立项保存到 `ImageOutputUSDPerImage`，资金不足时从尾部减少 `n`，至少保留 1 张；输入图片和 PDF 页数不可减少。

- [ ] **Step 5：运行附件和预算测试**

```powershell
go test -tags=unit ./internal/service -run 'TestInspectOpenAIAttachments|TestOpenAIBillingBudget.*Image|TestOpenAIBillingBudget.*PDF|TestOpenAIBillingBudget.*Mixed' -count=1
```

Expected：PASS；输入固定成本超过 2 USD 时返回请求前错误；请求历史字段为 0 不影响解析和预算。

- [ ] **Step 6：提交附件预算**

```powershell
git add backend/go.mod backend/go.sum backend/internal/service/openai_attachment_inspector.go backend/internal/service/openai_attachment_inspector_test.go backend/internal/service/openai_billing_budget.go backend/internal/service/openai_billing_budget_test.go backend/internal/service/openai_images.go
git commit -m "feat: 预算 OpenAI 图片与 PDF 输入"
```

## Task 6：实现套餐与流量卡事务内原子 hold

**Files:**
- Create: `backend/internal/repository/billing_authorization_repo.go`
- Create: `backend/internal/repository/billing_authorization_repo_integration_test.go`
- Modify: `backend/internal/repository/wire.go`
- Test: `backend/internal/repository/billing_authorization_repo_integration_test.go`

- [ ] **Step 1：写 15 并发和资金来源失败测试**

核心并发测试：

```go
func TestBillingAuthorizationRepositorySubscriptionHoldIsAtomicAcrossThreeAPIKeys(t *testing.T) {
	// 套餐可用 1 USD，每个请求完整预算 0.20 USD，3 个 Key 各发 5 个并发。
	results := runConcurrentReserves(t, repo, []int64{101, 102, 103}, 5, 0.20)
	require.Equal(t, 5, countReserved(results))
	require.Equal(t, 10, countInsufficient(results))
	require.InDelta(t, 1.00, sumActiveReservedUSD(t, subscriptionID), 1e-10)
}
```

再覆盖：

- 套餐完整预算成功时流量卡不变。
- 套餐不足、流量卡完整预算成功。
- 两者都只能覆盖缩减预算时仍按套餐优先。
- 0.47 USD 可用额度直接适配，不保留 0.5 USD 死区。
- 一个 traffic_credit authorization 可占用多张卡，但 `billing_source` 只有一个值。
- 过期卡和 `minimum_reserve_usd` 以下尾款不参与。
- 相同 `request_id + api_key_id` 且相同 fingerprint 幂等复用；不同 fingerprint 返回冲突。

- [ ] **Step 2：运行集成测试并确认失败**

Run：`go test -tags=integration ./internal/repository -run TestBillingAuthorizationRepository -count=1`

Expected：FAIL，提示 `NewBillingAuthorizationRepository` 不存在。

- [ ] **Step 3：实现套餐原子 hold**

`TryReserve` 的 subscription 分支必须在一个事务内：

```sql
SELECT us.id, us.daily_usage_usd, us.weekly_usage_usd, us.monthly_usage_usd,
       us.daily_window_start, us.weekly_window_start, us.monthly_window_start,
       g.daily_limit_usd, g.weekly_limit_usd, g.monthly_limit_usd
FROM user_subscriptions us
JOIN groups g ON g.id = us.group_id AND g.deleted_at IS NULL
WHERE us.id = $1 AND us.user_id = $2 AND us.status = 'active'
  AND us.deleted_at IS NULL AND us.expires_at > $3
FOR UPDATE OF us;
```

先按现有 timezone/window 规则规范当前用量，再查询同订阅所有 `reserved/dispatched/unknown` hold 总额。旧窗口仍在途的 hold 也计入当前判断，最多保守 5 分钟，防止跨窗口结算穿透。然后调用：

```go
fit, err := FitOpenAIBillingBudget(input.BudgetPlan, atomicAvailableUSD, input.Mode)
```

只有 fit 成功才插入 authorization。rolling weekly 同时锁定 `subscription_entitlement_periods` 的当前有效行并记录 `entitlement_period_id`。

- [ ] **Step 4：实现流量卡原子 hold**

同一事务中按 `expires_at, credited_at, id` 锁定：

```sql
SELECT id, remaining_usd, reserved_usd
FROM user_traffic_credits
WHERE user_id = $1 AND platform = $2
  AND expires_at > $3 AND remaining_usd > $4
ORDER BY expires_at, credited_at, id
FOR UPDATE;
```

用锁内可用总额调用相同 `FitOpenAIBillingBudget`，创建 authorization 后写 `billing_authorization_traffic_credit_items` 并增加每张卡的 `reserved_usd`。不得在事务外调用 `GetAvailableUSD`。

- [ ] **Step 5：所有状态更新检查 RowsAffected**

```go
func requireOneAuthorizationTransition(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil { return err }
	if affected != 1 { return ErrBillingAuthorizationInvalidTransition }
	return nil
}
```

`reserved -> dispatched`、`reserved/dispatched -> unknown`、允许的 release 路径均使用该检查；重复的同终态操作可显式幂等返回成功。

- [ ] **Step 6：运行 15 并发集成测试**

Run：`go test -tags=integration ./internal/repository -run TestBillingAuthorizationRepository -count=1`

Expected：PASS；重复运行 10 次仍只有可覆盖的请求成功，`reserved_usd` 无负数、无超额。

- [ ] **Step 7：提交原子 hold 仓储**

```powershell
git add backend/internal/repository/billing_authorization_repo.go backend/internal/repository/billing_authorization_repo_integration_test.go backend/internal/repository/wire.go
git commit -m "feat: 原子占用套餐与流量卡额度"
```

## Task 7：实现固定顺序授权编排和动态缩减

**Files:**
- Modify: `backend/internal/service/openai_billing_authorization.go`
- Modify: `backend/internal/service/openai_billing_authorization_test.go`
- Modify: `backend/internal/service/openai_gateway_service.go:1155-1173`
- Modify: `backend/internal/service/wire.go`

- [ ] **Step 1：改写授权服务失败测试**

测试调用顺序必须可观察：

```go
require.Equal(t, []reserveAttempt{
	{Source: BillingSourceSubscription, Mode: BillingAuthorizationReserveFull},
	{Source: BillingSourceTrafficCredit, Mode: BillingAuthorizationReserveFull},
	{Source: BillingSourceSubscription, Mode: BillingAuthorizationReserveFit},
}, repo.Attempts())
```

另加：显式输出上限不进入 fit；多图 fit 只减少 `n`；debt 在任何资金来源尝试前返回 402；重试复用相同 authorization；套餐和流量卡都失败时上游调用计数为 0。

- [ ] **Step 2：运行授权测试并确认失败**

Run：`go test -tags=unit ./internal/service -run TestOpenAIBillingAuthorization -count=1`

Expected：FAIL，旧服务仍先 `CheckAllLimits` 或事务外查询流量卡。

- [ ] **Step 3：实现唯一编排顺序**

```go
attempts := []struct {
	source BillingSource
	mode   BillingAuthorizationReserveMode
}{
	{BillingSourceSubscription, BillingAuthorizationReserveFull},
	{BillingSourceTrafficCredit, BillingAuthorizationReserveFull},
	{BillingSourceSubscription, BillingAuthorizationReserveFit},
	{BillingSourceTrafficCredit, BillingAuthorizationReserveFit},
}
```

不存在有效套餐时跳过 subscription；平台不支持流量卡时跳过 traffic_credit；`ExplicitOutputLimit` 且完整预算失败时跳过两个 fit 尝试。每次仓储调用内部完成“可用额度读取、预算适配、hold”，服务不得预读额度。

错误映射固定为：

```go
case errors.Is(err, ErrOpenAIBillingBudgetInsufficient):
	status, code = http.StatusPaymentRequired, "insufficient_quota"
case errors.Is(err, ErrOpenAIBillingBudgetExceedsHardCap):
	status, code = http.StatusPaymentRequired, "request_budget_exceeds_limit"
case errors.Is(err, ErrBillingAuthorizationDebtOutstanding):
	status, code = http.StatusPaymentRequired, "billing_debt_outstanding"
case errors.Is(err, ErrOpenAIAttachmentNotLocallyReadable):
	status, code = http.StatusBadRequest, "attachment_not_locally_readable"
```

- [ ] **Step 4：把响应对象统一为 AuthorizationID**

```go
type OpenAIBillingAuthorization struct {
	AuthorizationID     int64
	Source              BillingSource
	RequestFingerprint  string
	ReserveUSD          float64
	EffectiveBody       []byte
	EffectiveImageCount int
	PricingSnapshot     json.RawMessage
	EstimateBreakdown   json.RawMessage
}
```

标准运行模式下 `AuthorizationID` 必须大于 0；不再允许 subscription authorization 没有 durable ID。配置关闭时返回 `ErrBillingPreauthUnavailable`，不能回退旧计费或 shadow 放行；内层 `RunModeSimple` 明确不承担外层用户计费。

- [ ] **Step 5：运行测试**

Run：`go test -tags=unit ./internal/service -run TestOpenAIBillingAuthorization -count=1`

Expected：PASS；所有成功授权都有唯一 ID，调用序列与固定顺序完全一致。

- [ ] **Step 6：提交授权编排**

```powershell
git add backend/internal/service/openai_billing_authorization.go backend/internal/service/openai_billing_authorization_test.go backend/internal/service/openai_gateway_service.go backend/internal/service/wire.go
git commit -m "feat: 固定 OpenAI 请求资金来源"
```

## Task 8：固定 authorization 结算并移除 OpenAI 余额/追加扣款

**Files:**
- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Modify: `backend/internal/repository/usage_billing_repo_integration_test.go`
- Modify: `backend/internal/service/usage_fact.go`
- Modify: `backend/internal/repository/usage_fact_repo.go`
- Modify: `backend/internal/service/usage_fact_settlement_service.go`
- Modify: `backend/internal/service/usage_fact_settlement_service_test.go`
- Modify: `backend/internal/service/openai_gateway_service.go:6418-6600`

- [ ] **Step 1：写固定来源结算失败测试**

至少增加以下用例：

```go
func TestUsageBillingOpenAISettlementNeverUsesBalance(t *testing.T) {
	authorization := createSubscriptionAuthorization(t, 0.50)
	before := loadUserBalance(t, authorization.UserID)
	applyOpenAIUsage(t, authorization.ID, 0.20)
	require.Equal(t, before, loadUserBalance(t, authorization.UserID))
}

func TestUsageBillingTrafficCreditActualAboveHoldCreatesDebtWithoutUsingAnotherCard(t *testing.T) {
	authorization := createTrafficAuthorization(t, 0.40, []float64{0.40, 5.00})
	result := applyOpenAIUsage(t, authorization.ID, 0.55)
	require.InDelta(t, 0.15, result.AuthorizationDebtUSD, 1e-10)
	require.InDelta(t, 5.00, loadSecondCardRemainingUSD(t), 1e-10)
}

func TestUsageFactRequiresAuthorizationForOpenAI(t *testing.T) {
	_, err := NewUsageFact(openAIUsagePayloadWithoutAuthorization())
	require.ErrorIs(t, err, ErrOpenAIBillingAuthorizationRequired)
}
```

再覆盖：actual 小于 hold 时立即释放差额；subscription actual 超 hold 进入同一 authorization debt；重放 settlement 幂等；authorization 与 request/api_key/user/fingerprint 不一致时冲突；非 OpenAI 的余额命令仍可正常工作。

- [ ] **Step 2：运行测试并确认失败**

```powershell
go test -tags=unit ./internal/service -run 'TestUsageFact|TestUsageFactSettlement' -count=1
go test -tags=integration ./internal/repository -run 'TestUsageBilling.*Authorization|TestUsageBillingOpenAI' -count=1
```

Expected：FAIL，旧逻辑仍允许 `deductUsageBillingTrafficPackPartial` 或 OpenAI balance cost。

- [ ] **Step 3：使 OpenAI billing command 只携带实际总费用和 AuthorizationID**

`buildOpenAIUsageRecord` 必须要求：

```go
authorization := result.BillingAuthorization
if authorization == nil || authorization.AuthorizationID <= 0 {
	return nil, ErrOpenAIBillingAuthorizationRequired
}

params.authorizationID = &authorization.AuthorizationID
params.billingSource = authorization.Source
params.openAIActualCost = cost.TotalCost
params.balanceCost = 0
params.subscriptionCost = 0
params.trafficPackCost = 0
```

最终赋值必须是 `command.OpenAIActualCost = cost.TotalCost`；旧的 `BalanceCost/SubscriptionCost/TrafficPackCost` 只服务非 OpenAI 路径。`UsageBillingApplyResult` 增加 `AuthorizationDebtUSD float64`。删除 `authorization == nil` 时调用 `shouldBillWithTrafficPack()` 的分支。

- [ ] **Step 4：在单个事务中锁 authorization 并按 source 结算**

```go
switch authorization.Source {
case service.BillingSourceSubscription:
	return settleSubscriptionAuthorization(ctx, tx, authorization, cmd.OpenAIActualCost, cmd.CompletedAt)
case service.BillingSourceTrafficCredit:
	return settleTrafficCreditAuthorization(ctx, tx, authorization, cmd.OpenAIActualCost, policy)
default:
	return service.ErrInvalidInput
}
```

subscription 分支在同一事务内把 authorization 从活动状态转为 settled/debt，并调用现有日、周、月使用量更新；状态改变和 usage 增量不可分成两个事务。traffic_credit 分支只消费 authorization items，未使用金额从各卡 `reserved_usd` 释放。删除 actual 超 hold 后调用 `deductUsageBillingTrafficPackPartial()` 的代码。

- [ ] **Step 5：让 usage fact 强制引用 AuthorizationID**

`UsageFactRepository` 增加：

```go
FindByAuthorizationID(ctx context.Context, authorizationID int64) ([]UsageFact, error)
```

`CreatePending` 写 `authorization_id`；OpenAI payload 的 authorization 为空时创建失败。非 OpenAI usage fact 可以为空，判断条件使用 payload platform，而不是全表 `NOT NULL`，避免破坏其他平台。

- [ ] **Step 6：运行结算测试**

```powershell
go test -tags=unit ./internal/service -run 'TestUsageFact|TestUsageFactSettlement|TestOpenAIUsageBilling' -count=1
go test -tags=integration ./internal/repository -run 'TestUsageBilling.*Authorization|TestUsageBillingOpenAI' -count=1
```

Expected：PASS；OpenAI 用户余额始终不变，流量卡不会追加扣其他未 hold 卡，差额只进入 authorization debt。

- [ ] **Step 7：提交固定来源结算**

```powershell
git add backend/internal/service/usage_billing.go backend/internal/repository/usage_billing_repo.go backend/internal/repository/usage_billing_repo_integration_test.go backend/internal/service/usage_fact.go backend/internal/repository/usage_fact_repo.go backend/internal/service/usage_fact_settlement_service.go backend/internal/service/usage_fact_settlement_service_test.go backend/internal/service/openai_gateway_service.go
git commit -m "fix: 按固定授权来源结算 OpenAI 用量"
```

## Task 9：统一 finalizer、unknown 核销和 suspense

**Files:**
- Create: `backend/internal/service/openai_billing_finalizer.go`
- Create: `backend/internal/service/openai_billing_reconciliation.go`
- Create: `backend/internal/service/openai_billing_reconciliation_test.go`
- Modify: `backend/internal/repository/billing_authorization_repo.go`
- Modify: `backend/internal/repository/billing_authorization_repo_integration_test.go`
- Modify: `backend/internal/service/usage_fact_worker.go`
- Modify: `backend/internal/service/usage_fact_worker_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `deploy/config.example.yaml`

- [ ] **Step 1：写状态机和核销失败测试**

```go
func TestOpenAIBillingFinalizerTransportErrorMarksUnknown(t *testing.T) {
	finalizer.OnTransportError(context.Background(), authorization, errors.New("connection reset"))
	require.Equal(t, BillingAuthorizationUnknown, repo.Status(authorization.ID))
}

func TestOpenAIBillingReconciliationMovesFiveMinuteUnknownToSuspense(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 5, 1, 0, time.UTC)
	authorization := repo.UnknownAt(now.Add(-5*time.Minute-time.Second), 0.80)
	reconciler.RunOnce(context.Background(), now)
	require.Equal(t, BillingAuthorizationSuspense, repo.Status(authorization.ID))
	require.InDelta(t, 0.80, repo.SuspenseUSD(authorization.ID), 1e-10)
	require.InDelta(t, 0.0, repo.UserReservedUSD(authorization.UserID), 1e-10)
}
```

再覆盖：完整 usage fact 结算 unknown；明确未派发释放；4 分 59 秒不 suspense；非法状态转换返回错误；重复核销幂等；claim 使用 `SKIP LOCKED`。

- [ ] **Step 2：运行测试并确认失败**

Run：`go test -tags=unit ./internal/service -run 'TestOpenAIBillingFinalizer|TestOpenAIBillingReconciliation|TestUsageFactWorker' -count=1`

Expected：FAIL，finalizer/reconciler 不存在。

- [ ] **Step 3：实现统一 finalizer API**

```go
type OpenAIBillingFinalizer interface {
	ReleaseBeforeDispatch(context.Context, *OpenAIBillingAuthorization, string) error
	MarkDispatched(context.Context, *OpenAIBillingAuthorization) error
	MarkTransportUnknown(context.Context, *OpenAIBillingAuthorization, error)
	AttachForSettlement(*OpenAIForwardResult, *OpenAIBillingAuthorization)
}
```

规则固定：获取 token、构造 URL/请求体失败时 release；紧邻 `httpUpstream.Do` 或 WebSocket 上游写入前 mark dispatched；DNS/TCP/TLS/transport 不确定错误 mark unknown；拿到完整 usage 时把 authorization 附到 result，由 usage fact 结算；明确上游 4xx/5xx 且无 billable usage 可 release；failover 期间不创建第二个 authorization。

- [ ] **Step 4：实现 unknown claim 与 suspense 释放**

仓储 claim SQL：

```sql
SELECT id
FROM billing_authorizations
WHERE status = 'unknown' AND updated_at <= $1
ORDER BY updated_at, id
FOR UPDATE SKIP LOCKED
LIMIT $2;
```

核销顺序：

1. 查找相同 `authorization_id` 的完整 usage fact，有则调用 settlement。
2. 查到明确 `upstream_dispatched=false` 的终态证据则 release。
3. `unknown_at + 5m <= now` 时转 suspense，释放套餐活动 hold 或流量卡 `reserved_usd`，`suspense_usd=reserved_usd`。
4. 其余保持 unknown。

- [ ] **Step 5：增加配置并接入 worker**

配置名称固定为：

```yaml
billing:
  openai_authorization_enabled: true
  openai_authorization_hard_cap_usd: 2
  openai_authorization_reserved_timeout_seconds: 900
  openai_unknown_reconcile_interval_seconds: 30
  openai_unknown_timeout_seconds: 300
  openai_minimum_output_tokens: 256
  openai_default_max_output_tokens: 8192
  openai_pdf_max_bytes: 20971520
  openai_pdf_max_pages: 200
  openai_pdf_parse_timeout_seconds: 2
```

`UsageFactWorker` 每次 tick 只在达到 30 秒间隔时执行 reconciliation；reserved 清理只释放从未 dispatched 且已过 15 分钟的 authorization。

- [ ] **Step 6：运行状态机和 worker 测试**

```powershell
go test -tags=unit ./internal/service -run 'TestOpenAIBillingFinalizer|TestOpenAIBillingReconciliation|TestUsageFactWorker' -count=1
go test -tags=integration ./internal/repository -run 'TestBillingAuthorizationRepository.*Transition|TestBillingAuthorizationRepository.*Suspense' -count=1
```

Expected：PASS；unknown 最长 5 分钟，所有 transition 都检查受影响行数。

- [ ] **Step 7：提交状态机和核销**

```powershell
git add backend/internal/service/openai_billing_finalizer.go backend/internal/service/openai_billing_reconciliation.go backend/internal/service/openai_billing_reconciliation_test.go backend/internal/repository/billing_authorization_repo.go backend/internal/repository/billing_authorization_repo_integration_test.go backend/internal/service/usage_fact_worker.go backend/internal/service/usage_fact_worker_test.go backend/internal/config/config.go backend/internal/config/config_test.go deploy/config.example.yaml
git commit -m "feat: 核销 OpenAI 不确定计费状态"
```

## Task 10：接入 Responses、Chat、自动透传和 Messages

**Files:**
- Modify: `backend/internal/service/openai_gateway_service.go:1040-1225`
- Modify: `backend/internal/service/openai_gateway_service.go:3000-3620`
- Modify: `backend/internal/service/openai_gateway_chat_completions.go`
- Modify: `backend/internal/service/openai_gateway_chat_completions_raw.go`
- Modify: `backend/internal/service/openai_gateway_messages.go:28-340`
- Modify: `backend/internal/service/openai_gateway_service_hotpath_test.go`
- Modify: `backend/internal/service/openai_oauth_passthrough_test.go`
- Create: `backend/internal/service/openai_gateway_messages_billing_test.go`

- [ ] **Step 1：写四类入口失败测试**

每个测试使用 authorizer spy 和 upstream spy：

```go
require.Equal(t, 1, authorizer.AuthorizeCalls())
require.Equal(t, 1, authorizer.MarkDispatchedCalls())
require.Equal(t, 1, upstream.Calls())
require.Equal(t, authorization.AuthorizationID, result.BillingAuthorization.AuthorizationID)
```

增加：授权 402 时 upstream calls 为 0；raw fallback 与 failover 复用同一 ID；Messages 在 Anthropic -> Responses 转换和 OAuth transform 完成后预算；Messages previous_response_id 重试不重复 hold；token 获取失败 release。

- [ ] **Step 2：运行入口测试并确认失败**

```powershell
go test -tags=unit ./internal/service -run 'TestOpenAIGateway.*Billing|TestOpenAIOAuthPassthrough.*Billing|TestOpenAIMessages.*Billing' -count=1
```

Expected：FAIL；至少 Messages 尚未授权，旧辅助函数仍依赖可空 reservation ID。

- [ ] **Step 3：把 request state 改为通用 authorization state**

state 只保存一个 authorization，并记录：

```go
type openAIBillingAuthorizationRequestState struct {
	RequestFingerprint string
	Authorization      *OpenAIBillingAuthorization
	Dispatched         bool
	Unknown            bool
}
```

同 fingerprint 的 failover、raw fallback、Messages recursive retry 复用 state；不同 fingerprint 不能复用。

- [ ] **Step 4：统一派发边界**

所有路径必须形成相同顺序：

```go
authorization, effectiveBody, err := s.authorizeOpenAIForward(...)
if err != nil { return nil, err }
// token、URL、请求体构造失败：ReleaseBeforeDispatch
if err := s.billingFinalizer.MarkDispatched(ctx, authorization); err != nil { return nil, err }
resp, err := s.httpUpstream.Do(...)
if err != nil {
	s.billingFinalizer.MarkTransportUnknown(ctx, authorization, err)
	return nil, err
}
```

Messages 使用最终 `responsesBody` 预算，但 fingerprint 使用原始客户端请求；最终 effective output limit 必须回写到真正发送的 Responses body。

- [ ] **Step 5：运行入口测试**

Run：`go test -tags=unit ./internal/service -run 'TestOpenAIGateway.*Billing|TestOpenAIOAuthPassthrough.*Billing|TestOpenAIMessages.*Billing' -count=1`

Expected：PASS；所有目标 HTTP 文本入口在 upstream 前已有唯一 authorization。

- [ ] **Step 6：提交文本入口接入**

```powershell
git add backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_chat_completions.go backend/internal/service/openai_gateway_chat_completions_raw.go backend/internal/service/openai_gateway_messages.go backend/internal/service/openai_gateway_service_hotpath_test.go backend/internal/service/openai_oauth_passthrough_test.go backend/internal/service/openai_gateway_messages_billing_test.go
git commit -m "feat: 统一授权 OpenAI 文本入口"
```

## Task 11：接入 Embeddings、Images 和 input_tokens 明确失败

**Files:**
- Modify: `backend/internal/service/openai_embeddings.go`
- Modify: `backend/internal/handler/openai_embeddings.go`
- Modify: `backend/internal/service/openai_images.go`
- Modify: `backend/internal/service/openai_images_responses.go`
- Modify: `backend/internal/handler/openai_images.go`
- Modify: `backend/internal/server/routes/gateway.go:72-105`
- Create: `backend/internal/server/routes/gateway_input_tokens_test.go`
- Modify: `backend/internal/service/openai_images_test.go`
- Create: `backend/internal/service/openai_embeddings_billing_test.go`

- [ ] **Step 1：写 Embeddings、Images 和 501 失败测试**

```go
func TestResponsesInputTokensReturns501WithoutAuthorization(t *testing.T) {
	resp := postGateway(t, "/v1/responses/input_tokens", `{"model":"gpt-5.6-terra","input":"hi"}`)
	require.Equal(t, http.StatusNotImplemented, resp.Code)
	require.Equal(t, 0, authorizer.AuthorizeCalls())
	require.Equal(t, 0, upstream.Calls())
}

func TestEmbeddingsInsufficientBudgetDoesNotCallUpstream(t *testing.T) {
	resp := postEmbeddingsWithAvailableUSD(t, 0)
	require.Equal(t, http.StatusPaymentRequired, resp.Code)
	require.Equal(t, 0, upstream.Calls())
}

func TestImagesFitReducesCountButKeepsAtLeastOne(t *testing.T) {
	result := authorizeImages(t, imageRequest(4), 0.70)
	require.GreaterOrEqual(t, result.EffectiveImageCount, 1)
	require.Less(t, result.EffectiveImageCount, 4)
}
```

- [ ] **Step 2：运行测试并确认失败**

```powershell
go test -tags=unit ./internal/service -run 'TestEmbeddings.*Billing|TestImages.*Billing|TestImagesFit' -count=1
go test -tags=unit ./internal/server/routes -run TestResponsesInputTokens -count=1
```

Expected：FAIL；Embeddings 未授权，input_tokens 被 wildcard 转发或没有明确 501。

- [ ] **Step 3：接入 Embeddings**

Embeddings 使用 `DoNotClampOutputLimit=true`，预算只有解析后的输入 Token；在获取 API key、构造 URL 或请求失败时 release，在 `Do` 前 dispatched，transport 错误 unknown，成功/带 usage 错误进入 usage fact。

- [ ] **Step 4：接入 Images 的动态数量**

`AuthorizeImagesRequest` 使用 inspector 得到输入图片成本，输出预算按每张图片保存。authorization 返回的 `EffectiveImageCount` 必须同步改写 JSON `n` 或 multipart 字段；显式要求多图允许减少，因为用户已确认预算不足时减少数量，至少生成 1 张。输入图片、mask、PDF 页不可删除。

- [ ] **Step 5：在 wildcard 前注册 input_tokens 501**

```go
gateway.POST("/responses/input_tokens", func(c *gin.Context) {
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
	c.JSON(http.StatusNotImplemented, gin.H{"error": gin.H{
		"type": "not_implemented_error",
		"code": "input_token_counting_unavailable",
		"message": "Local input token counting endpoint is not available",
	}})
})
```

该路由不授权、不 hold、不计费、不转发。

- [ ] **Step 6：运行测试**

```powershell
go test -tags=unit ./internal/service -run 'TestEmbeddings.*Billing|TestImages.*Billing|TestImagesFit' -count=1
go test -tags=unit ./internal/server/routes -run TestResponsesInputTokens -count=1
```

Expected：PASS。

- [ ] **Step 7：提交 Embeddings、Images 和 501**

```powershell
git add backend/internal/service/openai_embeddings.go backend/internal/handler/openai_embeddings.go backend/internal/service/openai_images.go backend/internal/service/openai_images_responses.go backend/internal/handler/openai_images.go backend/internal/server/routes/gateway.go backend/internal/server/routes/gateway_input_tokens_test.go backend/internal/service/openai_images_test.go backend/internal/service/openai_embeddings_billing_test.go
git commit -m "feat: 授权 OpenAI 向量与图片入口"
```

## Task 12：WebSocket 每个 turn 独立授权

**Files:**
- Modify: `backend/internal/service/openai_ws_forwarder.go:220-227`
- Modify: `backend/internal/service/openai_ws_forwarder.go:2820-2910`
- Modify: `backend/internal/service/openai_ws_forwarder.go:3430-3880`
- Modify: `backend/internal/handler/openai_gateway_handler.go:1414-1515`
- Modify: `backend/internal/service/openai_ws_protocol_forward_test.go`
- Modify: `backend/internal/service/openai_ws_v2/passthrough_relay.go`
- Modify: `backend/internal/service/openai_ws_v2/passthrough_relay_test.go`

- [ ] **Step 1：写每 turn 授权失败测试**

```go
func TestResponsesWebSocketCreatesOneAuthorizationPerTurn(t *testing.T) {
	sendTurn(t, ws, `{"type":"response.create","model":"gpt-5.6-terra","input":"one"}`)
	sendTurn(t, ws, `{"type":"response.create","model":"gpt-5.6-terra","input":"two"}`)
	require.Equal(t, 2, authorizer.AuthorizeCalls())
	require.NotEqual(t, authorizer.IDs()[0], authorizer.IDs()[1])
}

func TestResponsesWebSocketRejectsTurnBeforeUpstreamWriteWhenBudgetFails(t *testing.T) {
	authorizer.FailTurn(2, ErrOpenAIBillingBudgetInsufficient)
	sendTwoTurns(t, ws)
	require.Equal(t, 1, upstream.TurnWrites())
}
```

再覆盖：每 turn 独立并发槽；transport write/read 中断进入对应 authorization unknown；终态 usage 只结算当前 turn；HTTP fallback 复用当前 turn authorization，不复用前一个 turn。

- [ ] **Step 2：运行 WebSocket 测试并确认失败**

```powershell
go test -tags=unit ./internal/service -run 'TestResponsesWebSocket.*Authorization|TestOpenAIWS.*Billing' -count=1
```

Expected：FAIL，现有 hooks 没有携带每 turn authorization。

- [ ] **Step 3：扩展 hooks 的每 turn 上下文**

```go
type OpenAIWSTurnBilling struct {
	Authorization *OpenAIBillingAuthorization
	EffectiveBody []byte
}

type OpenAIWSIngressHooks struct {
	InitialRequestModel string
	BeforeRequest func(turn int, payload []byte, originalModel string) (*OpenAIWSTurnBilling, error)
	BeforeTurn    func(turn int, billing *OpenAIWSTurnBilling) error
	AfterTurn     func(turn int, billing *OpenAIWSTurnBilling, result *OpenAIForwardResult, turnErr error)
}
```

首 turn 也必须走 `BeforeRequest`，不能沿用握手时的空授权。effective body 是实际写给上游的 `response.create`。

- [ ] **Step 4：在 handler 中按 turn 维护 authorization**

`BeforeRequest` 完成内容审核后调用授权；`BeforeTurn` 抢占槽位并紧邻上游写入 mark dispatched；`AfterTurn` 根据 result/turnErr 调用 finalizer、持久化 usage fact、释放槽位。任一 turn 的状态不得保存在连接级单个指针中。

- [ ] **Step 5：运行 WebSocket 测试**

Run：`go test -tags=unit ./internal/service -run 'TestResponsesWebSocket.*Authorization|TestOpenAIWS.*Billing' -count=1`

Expected：PASS；两个 turn 有两个 authorization，失败 turn 不写上游。

- [ ] **Step 6：提交 WebSocket 授权**

```powershell
git add backend/internal/service/openai_ws_forwarder.go backend/internal/handler/openai_gateway_handler.go backend/internal/service/openai_ws_protocol_forward_test.go backend/internal/service/openai_ws_v2/passthrough_relay.go backend/internal/service/openai_ws_v2/passthrough_relay_test.go
git commit -m "feat: 按 turn 授权 Responses WebSocket"
```

## Task 13：全链路故障注入、依赖注入替换和旧实现清理

**Files:**
- Create: `backend/internal/integration/openai_billing_gateway_test.go`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/internal/repository/wire.go`
- Modify: `backend/cmd/server/wire_gen.go`
- Create: `backend/internal/service/billing_authorization_metrics.go`
- Create: `backend/internal/service/billing_authorization_metrics_test.go`
- Delete: `backend/internal/service/openai_traffic_credit_budget.go`
- Delete: `backend/internal/service/openai_traffic_credit_budget_test.go`
- Delete: `backend/internal/service/traffic_credit_reservation.go`
- Delete: `backend/internal/service/traffic_credit_reservation_metrics.go`
- Delete: `backend/internal/service/traffic_credit_reservation_metrics_test.go`
- Delete: `backend/internal/repository/traffic_credit_reservation_repo.go`
- Delete: `backend/internal/repository/traffic_credit_reservation_repo_integration_test.go`

- [ ] **Step 1：写 mock upstream 全链路验收**

表驱动用例至少包括：

```go
tests := []struct {
	name              string
	upstreamBehavior  string
	wantStatus        BillingAuthorizationStatus
	wantUpstreamCalls int
}{
	{"request_before_dispatch_failure", "build_request_error", BillingAuthorizationReleased, 0},
	{"http_400_without_usage", "http_400", BillingAuthorizationReleased, 1},
	{"http_500_with_usage", "http_500_with_usage", BillingAuthorizationSettled, 1},
	{"transport_reset", "connection_reset", BillingAuthorizationUnknown, 1},
	{"sse_terminal_usage", "sse_complete", BillingAuthorizationSettled, 1},
	{"sse_disconnect_without_terminal", "sse_disconnect", BillingAuthorizationUnknown, 1},
	{"failover_then_success", "failover_success", BillingAuthorizationSettled, 2},
}
```

对每个入口断言 `actual_cost <= hold` 或状态为 unknown/suspense；402 用例 upstream calls 必须为 0。

- [ ] **Step 2：运行集成测试并确认失败**

Run：`go test -tags=e2e -v -timeout=300s ./internal/integration -run TestOpenAIBillingGateway`

Expected：FAIL，尚有未接入或旧依赖名。

- [ ] **Step 3：替换 Wire 依赖和指标名称**

Provider 统一为：

```go
ProvideBillingAuthorizationRepository
ProvideOpenAIBillingBudgetEstimator
ProvideOpenAIBillingAuthorizationService
ProvideOpenAIBillingFinalizer
ProvideOpenAIBillingReconciliationService
```

运行 `wire` 或项目现有生成方式更新 `backend/cmd/server/wire_gen.go`。指标从 `traffic_credit_reservation_*` 改为 `billing_authorization_*`，label 包含 source/status，不包含用户邮箱、API Key 或请求体。

- [ ] **Step 4：删除旧实现并扫描残留名称**

```powershell
rg -n "TrafficCreditReservation|traffic_credit_reservations|TrafficCreditReservationID|ReservationID|openai_traffic_credit_budget" backend/internal backend/cmd
```

Expected：只允许 migration 的历史表名和 migration 回归测试出现；业务代码没有旧类型或旧 ID。

- [ ] **Step 5：运行完整后端测试**

```powershell
Set-Location backend
go test -tags=unit ./... -count=1
go test -tags=integration ./internal/repository -count=1
go test -tags=e2e -v -timeout=300s ./internal/integration -run TestOpenAIBillingGateway
go vet ./...
```

Expected：全部 PASS；15 并发用例稳定；故障用例没有永久 reserved/dispatched。

- [ ] **Step 6：提交全链路清理**

```powershell
git add -A backend/internal backend/cmd backend/migrations backend/go.mod backend/go.sum
git commit -m "refactor: 收敛 OpenAI 通用计费授权链路"
```

## Task 14：启动 18081/18087、真实请求校准并输出结果

**Files:**
- Create: `deploy/.env.openai-billing-candidate.local`，从 example 复制的本地文件，不提交
- Create: `docs/ai/context/<执行时间>-openai-billing-calibration-result_CN.md`
- Modify as tests require: `backend/internal/service/openai_billing_budget.go`
- Modify as tests require: `backend/internal/service/openai_billing_budget_test.go`

- [ ] **Step 1：构建外层分支镜像和固定内层镜像**

```powershell
docker build -t sub2api-openai-billing-outer:local D:\CodeWorkSpace\sub2api-openai-billing
docker build -t sub2api-openai-billing-inner:local D:\CodeWorkSpace\sub2api-upstream-latest
```

候选 `.env` 指向这两个镜像，外层启用 authorization，内层保持当前 latest 行为且不重复用户计费。

- [ ] **Step 2：恢复最新双库备份并启动候选环境**

```powershell
.\deploy\openai-billing-candidate.ps1 -Action Backup
.\deploy\openai-billing-candidate.ps1 -Action Restore
.\deploy\openai-billing-candidate.ps1 -Action Up
.\deploy\openai-billing-candidate.ps1 -Action Verify
```

Expected：`http://127.0.0.1:18081/health` 和 `http://127.0.0.1:18087/health` 返回 200；外层 migration 180 已应用；公网 `18080/18086` 仍返回 200 且容器未重建。

- [ ] **Step 3：先跑不消耗真实额度的候选 smoke tests**

用克隆库对应测试用户验证：

- `/v1/responses/input_tokens` 返回 501 且 authorization 数不变。
- 明显超过 2 USD 的本地输入返回 402 且内层访问日志没有请求。
- 无可用套餐/流量卡返回 402。
- 1 USD 和 0.47 USD 可用额度能得到动态 output limit，不保留固定 0.5 USD。

查询断言：

```sql
SELECT request_id, billing_source, reserved_usd, status, estimator_version
FROM billing_authorizations
ORDER BY id DESC
LIMIT 20;
```

- [ ] **Step 4：通过进程环境注入已授权测试 Key**

执行前确认当前进程已有 `OPENAI_BILLING_TEST_KEY`，只在内存中构造 Authorization header：

```powershell
if ([string]::IsNullOrWhiteSpace($env:OPENAI_BILLING_TEST_KEY)) {
  throw "缺少进程级 OPENAI_BILLING_TEST_KEY"
}
```

不得输出该变量，不得写入 curl 文件、`.env`、测试快照、日志或文档。

- [ ] **Step 5：运行真实校准矩阵**

依次请求：

1. GPT-5.5 短文本，未显式 output limit。
2. GPT-5.6 Terra 短文本和 100k 以上长上下文。
3. GPT-5.6 Sol 显式 output limit。
4. 单张 low/high 图片输入。
5. 两张图片输入混合文字。
6. 两页 PDF，包含可提取文本和页面视觉。
7. GPT Image 2 单图生成、图生图、多图生成。
8. 人为设置克隆库可用额度为 1 USD、0.47 USD 的动态适配。

每个请求保存 `request_id`，从克隆库读取 authorization 的 `estimate_breakdown/pricing_snapshot/reserved_usd` 和 usage log 的实际 Token/费用，不保存请求正文或附件原始内容。

- [ ] **Step 6：按真实结果驱动公式迭代**

每发现一类 `actual_cost > hold`：

1. 先在 `openai_billing_budget_test.go` 增加复现该请求结构的失败测试。
2. 把 `OpenAIBillingEstimatorVersion` 从 `openai-local-vN` 增至下一版本。
3. 只调整通用协议开销、图片规则或 PDF 规则，不按用户/API Key/request_id 特判。
4. 重跑全部预算单测、mock upstream 测试和真实矩阵。

结束条件：所有取得完整 usage 的真实请求 `actual_cost <= reserved_usd`；模型提前结束产生的剩余 hold 已释放；不完整 usage 进入 unknown 并在 5 分钟内变为 settled/released/suspense。

- [ ] **Step 7：写校准结果文档**

文档表格至少包含：

```text
请求类型 | 模型 | estimator_version | 估算输入 Token | 估算输出上限 | hold USD | 实际 USD | 释放 USD | 最终状态
```

同时记录公网保护证据、候选容器名、migration 版本、测试命令和失败后修正次数；不记录 Key、完整请求体、附件内容和 OAuth 凭据。

- [ ] **Step 8：最终验证并提交校准改动**

执行前完整读取 `superpowers:verification-before-completion`，然后运行：

```powershell
Set-Location D:\CodeWorkSpace\sub2api-openai-billing\backend
go test -tags=unit ./... -count=1
go test -tags=integration ./internal/repository -count=1
go test -tags=e2e -v -timeout=300s ./internal/integration -run TestOpenAIBillingGateway
go vet ./...
Invoke-WebRequest http://127.0.0.1:18080/health -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:18086/health -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:18081/health -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:18087/health -UseBasicParsing
```

Expected：全部通过，四个 health 均为 200。

```powershell
git add backend/internal/service/openai_billing_budget.go backend/internal/service/openai_billing_budget_test.go docs/ai/context/*-openai-billing-calibration-result_CN.md
git commit -m "test: 校准 OpenAI 请求前计费预算"
```

## 最终验收清单

- [ ] Responses、Chat、Messages、Embeddings、Images、自动透传/failover、WebSocket 每 turn 请求前都有唯一 durable authorization。
- [ ] 资金来源严格为套餐、流量卡、402；结算不改选，不混合。
- [ ] OpenAI 用户余额不扣减、不透支；购买、充值、退款和非 OpenAI 路径回归通过。
- [ ] 3 个 API Key × 每 Key 5 并发时，套餐和流量卡都不会穿透。
- [ ] 2 USD 仅为硬上限；1 USD、0.47 USD 能动态适配；没有固定 0.5 USD 死区。
- [ ] 显式 output limit 不被静默修改；未显式 limit 才写入实际上游 body。
- [ ] 多图预算不足时减少 `n` 且至少 1 张；图片/PDF 输入固定成本不能被删除。
- [ ] PDF 文本和每页视觉都计费；不可解析、`file_id`、视频请求前明确失败。
- [ ] GPT-5.5、GPT-5.6、GPT Image 2 有效价格只使用一次，没有重复倍率。
- [ ] `/v1/responses/input_tokens` 返回 501，不 hold、不计费、不转发。
- [ ] actual 超 hold 只在同 authorization 记 debt；不会丢响应、不会追加扣其他卡。
- [ ] unknown 每 30 秒核销，最多 5 分钟；超时 suspense 并释放用户额度。
- [ ] 所有状态转换检查 RowsAffected，所有 hold 最终进入 settled/released/debt/suspense 或受控 unknown。
- [ ] 真实完整 usage 用例中 `actual_cost > hold` 为 0。
- [ ] `18080/18086` 容器、数据库、Redis、Nginx 和公网流量未改变。

## 执行交接

计划保存于 `docs/ai/context/20260728-163830-openai-billing-atomic-hold-implementation-plan_CN.md`。

执行方式：

1. **Subagent-Driven（推荐）**：使用 `superpowers:subagent-driven-development`，每个 Task 独立实现并在任务间复核。
2. **Inline Execution**：在当前会话使用 `superpowers:executing-plans`，分批执行并设置检查点。

当前协作约束下不会自动启动子代理；只有用户明确选择第一种方式后才进行委派。
