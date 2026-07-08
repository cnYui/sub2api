# 套餐 F：198 元订阅实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans or equivalent inline execution. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 上架 `198 元 / 每日 179 USD / 30 天` 的订阅套餐 F，并保证前后端购买链路按现有数据驱动逻辑工作。

**Architecture:** 新增独立 migration seed `groups` 与 `subscription_plans`，前端继续通过 checkout API 动态渲染套餐。源码 migration 不绑定上游账号，公网发布后单独做 `account_groups` 运行态绑定与真实请求验收。

**Tech Stack:** Go migration regression tests, PostgreSQL SQL migrations, Vue 3 + Vitest purchase page tests.

---

### Task 1: 后端迁移回归测试

**Files:**
- Modify: `backend/migrations/auth_identity_payment_migrations_regression_test.go`
- Create later: `backend/migrations/161_seed_codex_198_subscription_plan.sql`

- [ ] **Step 1: Write the failing test**

在 `auth_identity_payment_migrations_regression_test.go` 追加：

```go
func TestMigration161SeedsCodex198SubscriptionPlanWithoutAccountBinding(t *testing.T) {
	content, err := FS.ReadFile("161_seed_codex_198_subscription_plan.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "codex-pool-179-usd")
	require.Contains(t, sql, "198 元订阅池")
	require.Contains(t, sql, "daily_limit_usd = 179")
	require.Contains(t, sql, "price = 198.00")
	require.Contains(t, sql, "image_price_1k = 0.10")
	require.Contains(t, sql, "image_price_2k = 0.20")
	require.Contains(t, sql, "image_price_4k = 0.40")
	require.NotContains(t, sql, "INSERT INTO account_groups")
	require.NotContains(t, sql, "UPDATE account_groups")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd backend
go test -count=1 ./migrations -run TestMigration161SeedsCodex198SubscriptionPlanWithoutAccountBinding
```

Expected: FAIL，错误原因是 `161_seed_codex_198_subscription_plan.sql` 不存在。

- [ ] **Step 3: Create migration**

新增 `backend/migrations/161_seed_codex_198_subscription_plan.sql`，结构沿用 156/154：

- group name `codex-pool-179-usd`
- plan name/product name `198 元订阅池`
- group description `198 元订阅池，每日 179 USD，30 天有效期`
- plan description `月度订阅-时间 30天，日限额 179刀，24点刷新`
- price `198.00`
- daily limit `179`
- sort order `198`
- no `account_groups` mutation

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd backend
go test -count=1 ./migrations -run 'TestMigration161|TestMigration15[4-7]'
```

Expected: PASS。

### Task 2: 前端购买页 F 套餐回归测试

**Files:**
- Modify: `frontend/src/views/user/__tests__/PaymentView.spec.ts`

- [ ] **Step 1: Write the failing test**

新增六档 fixture 或扩展现有五档 fixture，包含：

```ts
{
  id: 6,
  group_id: 10,
  name: '198 元订阅池',
  price: 198,
  daily_limit_usd: 179,
  group_name: 'codex-pool-179-usd',
  sort_order: 198,
}
```

新增测试断言：

```ts
expect(wrapper.text()).toContain('阅读订阅套餐F')
expect(wrapper.text()).toContain('日限额179刀')
expect(wrapper.text()).toContain('¥198')
```

选择 F 卡片后确认下单：

```ts
expect(createOrder).toHaveBeenCalledWith(expect.objectContaining({
  amount: 198,
  payment_type: 'alipay',
  order_type: 'subscription',
  plan_id: 6,
  is_mobile: true,
}))
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts --testNamePattern "198|F"
```

Expected: FAIL，因为 fixture/测试辅助尚未覆盖六档。

- [ ] **Step 3: Minimal frontend test fixture update**

新增 `checkoutInfoWithSixManualPlansFixture()` 与 `checkoutInfoWithSixZPayPlansFixture()`，不修改生产组件；若生产组件已有数据驱动能力，测试应靠 fixture 变绿。

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts --testNamePattern "198|F"
```

Expected: PASS。

### Task 3: 回归验证与上下文记忆

**Files:**
- Modify: `AGENTS.md`
- Add: `docs/ai/context/*-result_CN.md`

- [ ] **Step 1: Run backend migration regression**

```bash
cd backend
go test -count=1 ./migrations
```

- [ ] **Step 2: Run frontend payment test**

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts
```

- [ ] **Step 3: Run static diff check**

```bash
git diff --check
```

- [ ] **Step 4: Update AGENTS.md**

在“最高优先级定论”顶部新增一条：本地已上架套餐 F 的代码与测试情况；注明未部署公网、未绑定运行态上游账号。

- [ ] **Step 5: Write result doc**

新增 `docs/ai/context/YYYYMMDD-HHMMSS-codex-plan-f-198-subscription-result_CN.md`，记录改动、测试、未做事项和发布验收要求。
