# 149/199 元订阅套餐实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans or equivalent inline execution. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 `149 元 / 每日 135 USD` 和 `199 元 / 每日 179 USD` 两档替代上一轮本地未发布的 `198 元 / 每日 179 USD` 方案。

**Architecture:** 单个新 migration 幂等 seed 两个 subscription group 和两个 subscription plan；前端继续通过 checkout API 动态渲染；运行态上游绑定留到公网发布后单独完成。

**Tech Stack:** Go migration tests, PostgreSQL SQL migrations, Vue 3 + Vitest purchase page tests.

---

### Task 1: 后端迁移测试 RED/GREEN

**Files:**
- Modify: `backend/migrations/auth_identity_payment_migrations_regression_test.go`
- Delete: `backend/migrations/161_seed_codex_198_subscription_plan.sql`
- Create: `backend/migrations/161_seed_codex_149_199_subscription_plans.sql`

- [ ] **Step 1: Write failing migration test**

把测试改为读取 `161_seed_codex_149_199_subscription_plans.sql`，断言包含：

```go
require.Contains(t, sql, "codex-pool-135-usd")
require.Contains(t, sql, "149 元订阅池")
require.Contains(t, sql, "daily_limit_usd = plan.daily_usd")
require.Contains(t, sql, "price = plan.price")
require.Contains(t, sql, "codex-pool-179-usd")
require.Contains(t, sql, "199 元订阅池")
require.Contains(t, sql, "149.00::NUMERIC")
require.Contains(t, sql, "199.00::NUMERIC")
require.NotContains(t, sql, "INSERT INTO account_groups")
require.NotContains(t, sql, "UPDATE account_groups")
```

- [ ] **Step 2: Run RED**

```bash
cd backend
go test -count=1 ./migrations -run TestMigration161SeedsCodex149And199SubscriptionPlansWithoutAccountBinding
```

Expected: FAIL，原因是新 migration 文件不存在。

- [ ] **Step 3: Replace migration**

新增 `161_seed_codex_149_199_subscription_plans.sql`，用 `VALUES` 循环处理两档：

```sql
('codex-pool-135-usd', '149 元订阅池', 149.00::NUMERIC, 135::NUMERIC, 149::INTEGER),
('codex-pool-179-usd', '199 元订阅池', 199.00::NUMERIC, 179::NUMERIC, 199::INTEGER)
```

删除未提交的 `161_seed_codex_198_subscription_plan.sql`。

- [ ] **Step 4: Run GREEN**

```bash
cd backend
go test -count=1 ./migrations -run 'TestMigration161|TestMigration15[4-7]'
```

Expected: PASS。

### Task 2: 前端购买页测试 RED/GREEN

**Files:**
- Modify: `frontend/src/views/user/__tests__/PaymentView.spec.ts`

- [ ] **Step 1: Write failing frontend expectation**

把六档 fixture 改为七档 fixture，先让测试期望：

```ts
expect(wrapper.text()).toContain('阅读订阅套餐F')
expect(wrapper.text()).toContain('日限额135刀')
expect(wrapper.text()).toContain('¥149')
expect(wrapper.text()).toContain('阅读订阅套餐G')
expect(wrapper.text()).toContain('日限额179刀')
expect(wrapper.text()).toContain('¥199')
```

并断言选择 G 档时：

```ts
expect(createOrder).toHaveBeenCalledWith(expect.objectContaining({
  amount: 199,
  order_type: 'subscription',
  plan_id: 7,
}))
```

- [ ] **Step 2: Run RED**

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts --testNamePattern "149|199|plan G|plan F"
```

Expected: FAIL，原因是 fixture 仍是 198 档或没有 149/199 两档。

- [ ] **Step 3: Update fixture**

新增或重命名为 `checkoutInfoWithSevenZPayPlansFixture()`，包含：

```ts
{ id: 6, group_id: 10, name: '149 元订阅池', price: 149, daily_limit_usd: 135, group_name: 'codex-pool-135-usd', sort_order: 149 }
{ id: 7, group_id: 11, name: '199 元订阅池', price: 199, daily_limit_usd: 179, group_name: 'codex-pool-179-usd', sort_order: 199 }
```

- [ ] **Step 4: Run GREEN**

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts --testNamePattern "149|199|plan G|plan F"
```

Expected: PASS。

### Task 3: 验证与上下文

**Files:**
- Modify: `AGENTS.md`
- Add: `docs/ai/context/YYYYMMDD-HHMMSS-codex-149-199-subscription-result_CN.md`

- [ ] **Step 1: Full targeted verification**

```bash
cd backend && go test -count=1 ./migrations
cd frontend && pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts
git diff --check
```

- [ ] **Step 2: Update memory**

更新 `AGENTS.md` 顶部，说明 198 元本地方案已被 149/199 两档替代。

- [ ] **Step 3: Write result doc**

新增结果文档，记录测试、未做事项、发布后绑定 `codex-pool-135-usd` 与 `codex-pool-179-usd` 的要求。
