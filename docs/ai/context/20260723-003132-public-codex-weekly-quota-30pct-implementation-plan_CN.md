# Public Codex Weekly Quota 30 Percent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 7 个公开 Codex 订阅档位的周额度提升到新整数值，并同步后端配置、数据库迁移、有效权益、未发放订单快照和前端展示。

**Architecture:** 继续以后端 `group.go` 的公开 Codex 固定映射作为代码层唯一来源，接口、下单快照、权益发放和管理端校准都通过现有函数派生。数据库使用新增前向迁移更新当前可变配置、有效权益事实和未发放订单快照，不修改历史迁移、不清空任何用量事实。

**Tech Stack:** Go、Ent、PostgreSQL SQL migration、Vue 3、TypeScript、Vitest。

---

## 文件结构

- Modify: `backend/internal/service/group.go`
  - 责任：公开 Codex 分组到周额度的后端固定映射。
- Create: `backend/migrations/176_increase_public_codex_weekly_quota_amounts.sql`
  - 责任：前向更新数据库里的分组、套餐、有效权益和未发放订单快照。
- Modify: `frontend/src/utils/subscriptionQuota.ts`
  - 责任：公开 Codex 分组到周额度的前端固定映射。
- Modify: `frontend/src/i18n/locales/zh.ts`
  - 责任：首页公开套餐说明。
- Modify tests:
  - `backend/internal/service/payment_config_service_test.go`
  - `backend/internal/service/subscription_weekly_window_test.go`
  - `backend/internal/service/payment_fulfillment_test.go`
  - `backend/internal/service/payment_refund_test.go`
  - `frontend/src/utils/__tests__/subscriptionQuota.spec.ts`
  - `frontend/src/views/user/__tests__/PaymentView.spec.ts`
  - `frontend/src/views/user/__tests__/SubscriptionsView.spec.ts`
  - `frontend/src/views/__tests__/KeyUsageView.spec.ts`
  - `frontend/src/views/__tests__/HomeView.spec.ts`
  - `frontend/src/views/admin/orders/__tests__/PlanEditDialog.spec.ts`
  - `frontend/src/components/common/__tests__/SubscriptionProgressMini.spec.ts`

## Task 1: 后端固定额度映射

**Files:**
- Modify: `backend/internal/service/group.go`
- Test: `backend/internal/service/subscription_weekly_window_test.go`
- Test: `backend/internal/service/payment_config_service_test.go`

- [ ] **Step 1: 先把后端测试期望改成新值，验证失败**

把 `subscription_weekly_window_test.go` 中公开 Codex 断言改成：

```go
snapshot := BuildPlanQuotaSnapshot("codex-pool-29-usd", &dailyLimit, nil, nil, 30, "day")
require.Equal(t, 102.0, *snapshot.WeeklyLimitUSD)
require.Equal(t, 408.0, *snapshot.PeriodTotalQuotaUSD)
```

把 `payment_config_service_test.go` 中公开套餐文案断言改成：

```go
if plan.Description != "28 天订阅，每 7 天刷新 76 USD 周额度，购买时间起滚动计算" {
	t.Fatalf("Description = %q", plan.Description)
}
if plan.Features != "周额度 76 USD\n28 天有效期\n购买时间起每 7 天刷新" {
	t.Fatalf("Features = %q", plan.Features)
}
if updated.Description != "28 天订阅，每 7 天刷新 102 USD 周额度，购买时间起滚动计算" {
	t.Fatalf("Description = %q", updated.Description)
}
if updated.Features != "周额度 102 USD\n28 天有效期\n购买时间起每 7 天刷新" {
	t.Fatalf("Features = %q", updated.Features)
}
```

Run:

```powershell
go test -tags unit ./internal/service -run "Test.*(PublicCodex|PaymentConfig)"
```

Expected: FAIL，失败原因是实际返回仍为 58/78 旧额度。

- [ ] **Step 2: 修改后端映射**

把 `backend/internal/service/group.go` 的 map 改成：

```go
var publicCodexSubscriptionWeeklyLimitsUSD = map[string]float64{
	"codex-pool-19-usd":  76,
	"codex-pool-29-usd":  102,
	"codex-pool-49-usd":  154,
	"codex-pool-69-usd":  206,
	"codex-pool-89-usd":  258,
	"codex-pool-135-usd": 389,
	"codex-pool-179-usd": 520,
}
```

- [ ] **Step 3: 验证后端映射测试通过**

Run:

```powershell
go test -tags unit ./internal/service -run "Test.*(PublicCodex|PaymentConfig)"
```

Expected: PASS。

## Task 2: 数据库前向迁移

**Files:**
- Create: `backend/migrations/176_increase_public_codex_weekly_quota_amounts.sql`

- [ ] **Step 1: 新增迁移文件**

创建迁移，核心 SQL 结构如下：

```sql
WITH plan_limits(group_name, weekly_usd, plan_label) AS (
  VALUES
    ('codex-pool-19-usd'::text, 76::numeric, '29 元订阅池'::text),
    ('codex-pool-29-usd', 102::numeric, '39 元订阅池'),
    ('codex-pool-49-usd', 154::numeric, '59 元订阅池'),
    ('codex-pool-69-usd', 206::numeric, '79 元订阅池'),
    ('codex-pool-89-usd', 258::numeric, '99 元订阅池'),
    ('codex-pool-135-usd', 389::numeric, '149 元订阅池'),
    ('codex-pool-179-usd', 520::numeric, '199 元订阅池')
)
UPDATE groups g
SET weekly_limit_usd = p.weekly_usd,
    daily_limit_usd = NULL,
    monthly_limit_usd = NULL,
    default_validity_days = 28,
    description = p.plan_label || '，每 7 天 ' || trim_scale(p.weekly_usd) || ' USD，28 天有效期',
    updated_at = NOW()
FROM plan_limits p
WHERE g.name = p.group_name
  AND g.subscription_type = 'subscription'
  AND g.deleted_at IS NULL;
```

同一文件继续更新 `subscription_plans`、`subscription_entitlement_periods` 和未完成发放的 `payment_orders.subscription_snapshot`。订单快照更新必须补齐 `version` 与 `plan_id`，只覆盖 `subscription_id IS NULL` 且 `plan_id IS NOT NULL` 的订阅订单；`PENDING` 必须额外满足 `expires_at > NOW()`，`PAID`、`RECHARGING` 视为已支付但未完成发放，`FAILED` 还必须满足 `paid_at IS NOT NULL` 才按可重试发放订单更新。

- [ ] **Step 2: 检查迁移只前向修改目标数据**

Run:

```powershell
rg -n "codex-pool-local-unlimited|UPDATE usage_|DELETE|DROP|TRUNCATE" backend/migrations/176_increase_public_codex_weekly_quota_amounts.sql
```

Expected: 不出现会删除或重写用量事实的 SQL；不出现 `codex-pool-local-unlimited`。

## Task 3: 支付发放与退款测试期望

**Files:**
- Modify: `backend/internal/service/payment_fulfillment_test.go`
- Modify: `backend/internal/service/payment_refund_test.go`

- [ ] **Step 1: 先改测试期望，验证失败**

把支付快照测试里的 29 元档断言改为：

```go
"weekly_limit_usd":       76,
"period_total_quota_usd": 304,
```

把退款测试里 29 元档退款基准改为：

```go
require.InDelta(t, 304, reloaded.RefundBasis["period_total_quota_usd"].(float64), 1e-9)
require.InDelta(t, 58, reloaded.RefundBasis["used_quota_usd"].(float64), 1e-9)
```

Run:

```powershell
go test -tags unit ./internal/service -run "Test.*(ConfirmPaymentFulfillsPublicCodex|Refund.*Quota)"
```

Expected: 相关测试在生产代码未更新前 FAIL。

- [ ] **Step 2: 确认生产代码无需额外修改**

因为支付发放和退款都读快照/权益段；后端 map 更新后新订单自然进入新额度，迁移更新有效权益后退款自然按新总额度计算。若测试仍失败，优先定位是否存在硬编码旧值。

- [ ] **Step 3: 验证支付与退款测试通过**

Run:

```powershell
go test -tags unit ./internal/service -run "Test.*(ConfirmPaymentFulfillsPublicCodex|Refund.*Quota)"
```

Expected: PASS。

## Task 4: 前端额度映射和页面测试

**Files:**
- Modify: `frontend/src/utils/subscriptionQuota.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: listed frontend spec files

- [ ] **Step 1: 先改前端测试期望，验证失败**

将公开 Codex 旧值改成新值：

```ts
expect(publicCodexSubscriptionWeeklyLimitUSD('codex-pool-19-usd')).toBe(76)
expect(publicCodexSubscriptionWeeklyLimitUSD('codex-pool-179-usd')).toBe(520)
```

购买页 fixture 改成：

```ts
weekly_limit_usd: 76,
period_total_quota_usd: 304,
```

并同步 39/59/79/99/149/199 元档为 `102/408`、`154/616`、`206/824`、`258/1032`、`389/1556`、`520/2080`。

Run:

```powershell
pnpm vitest run src/utils/__tests__/subscriptionQuota.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/SubscriptionsView.spec.ts src/views/__tests__/KeyUsageView.spec.ts src/views/__tests__/HomeView.spec.ts src/views/admin/orders/__tests__/PlanEditDialog.spec.ts src/components/common/__tests__/SubscriptionProgressMini.spec.ts
```

Expected: FAIL，失败原因是前端映射和文案仍为旧额度。

- [ ] **Step 2: 修改前端映射和首页文案**

把 `subscriptionQuota.ts` 的 map 改为：

```ts
const PUBLIC_CODEX_SUBSCRIPTION_WEEKLY_LIMITS_USD = new Map<string, number>([
  ['codex-pool-19-usd', 76],
  ['codex-pool-29-usd', 102],
  ['codex-pool-49-usd', 154],
  ['codex-pool-69-usd', 206],
  ['codex-pool-89-usd', 258],
  ['codex-pool-135-usd', 389],
  ['codex-pool-179-usd', 520],
])
```

把 `zh.ts` 首页描述改为新整数：

```ts
unifiedGatewayDesc: '28 天订阅，每 7 天刷新，周额度 76 刀'
multiAccountDesc: '28 天订阅，每 7 天刷新，周额度 102 刀'
balanceQuotaDesc: '28 天订阅，每 7 天刷新，周额度 154 刀'
premiumQuotaDesc: '28 天订阅，每 7 天刷新，周额度 258 刀'
```

- [ ] **Step 3: 验证前端相关测试通过**

Run:

```powershell
pnpm vitest run src/utils/__tests__/subscriptionQuota.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/SubscriptionsView.spec.ts src/views/__tests__/KeyUsageView.spec.ts src/views/__tests__/HomeView.spec.ts src/views/admin/orders/__tests__/PlanEditDialog.spec.ts src/components/common/__tests__/SubscriptionProgressMini.spec.ts
```

Expected: PASS。

## Task 5: 全量验证与结果文档

**Files:**
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-public-codex-weekly-quota-30pct-result_CN.md`

- [ ] **Step 1: 后端完整验证**

Run:

```powershell
go test ./...
```

Expected: PASS。

- [ ] **Step 2: 前端验证**

Run:

```powershell
pnpm typecheck
pnpm lint:check
pnpm test:run
pnpm build
```

Expected: PASS。

- [ ] **Step 3: 写结果文档**

记录：

```markdown
# 公共 Codex 周额度提升 30% 实施结果

- 分支：`codex/public-codex-weekly-quota-30pct`
- 新增迁移：`backend/migrations/176_increase_public_codex_weekly_quota_amounts.sql`
- 额度变化：58 -> 76、78 -> 102、118 -> 154、158 -> 206、198 -> 258、299 -> 389、400 -> 520 USD
- 数据策略：保留历史用量，更新上限、有效权益和未发放订单快照。
- 验证：列出实际执行的命令与结果。
```

- [ ] **Step 4: 检查上下文文档和 git diff**

Run:

```powershell
git ls-files --others --exclude-standard docs/ai/context
git diff --stat
```

Expected: 新增 design/plan/result 文档都可见；diff 只包含本次需求相关文件。
