# 流量卡进度条修复实施计划

> **执行要求：** 按任务顺序使用 TDD 实施；每个生产代码修改前必须先运行对应失败测试。

**目标：** 让仍可用流量卡按 `已用额度 / 初始总额` 正向展示进度，同时保持批次用满后从汇总移除、最后一张用满后卡片消失。

**架构：** 后端在现有 `TrafficCreditSummary` 增加当前可用批次的初始总额，并用与剩余额度完全相同的过滤条件聚合。前端只消费该结构，通过 `初始总额 - 剩余额度` 计算已用额度，不读取商品、订单或流水推断总额。

**技术栈：** Go、database/sql、SQLite 单元测试、Vue 3、TypeScript、Vitest。

---

### Task 1：扩展后端流量卡汇总契约

**文件：**

- 修改：`backend/internal/repository/traffic_pack_repo_test.go`
- 修改：`backend/internal/service/traffic_pack.go`
- 修改：`backend/internal/repository/traffic_pack_repo.go`

- [ ] **Step 1：先为初始总额和耗尽过滤补失败测试**

在 `TestTrafficPackRepository_CreditPurchaseIsIdempotentAndSummarizes` 增加：

```go
require.InDelta(t, 20, summary.TotalInitialUSD, 0.000001)
```

在 `TestTrafficPackRepository_DeductConsumesEarliestExpiringCredits` 扣费前增加多批次汇总断言：

```go
summary, err := repo.GetSummary(ctx, 9, now.Add(48*time.Hour))
require.NoError(t, err)
require.InDelta(t, 15, summary.TotalInitialUSD, 0.000001)
require.InDelta(t, 15, summary.TotalRemainingUSD, 0.000001)
```

扣费后保留剩余额度断言，并增加已耗尽 5 USD 批次已被汇总移除的断言：

```go
summary, err = repo.GetSummary(ctx, 9, now.Add(48*time.Hour))
require.NoError(t, err)
require.InDelta(t, 10, summary.TotalInitialUSD, 0.000001)
require.InDelta(t, 8, summary.TotalRemainingUSD, 0.000001)
```

最后增加过期批次不参与汇总的断言：

```go
summary, err = repo.GetSummary(ctx, 9, now.AddDate(1, 0, 2))
require.NoError(t, err)
require.Zero(t, summary.TotalInitialUSD)
require.Zero(t, summary.TotalRemainingUSD)
```

- [ ] **Step 2：运行后端目标测试，确认 RED**

运行：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/repository -run '^TestTrafficPackRepository_(CreditPurchaseIsIdempotentAndSummarizes|DeductConsumesEarliestExpiringCredits)$'
```

预期：编译失败，提示 `TrafficCreditSummary` 没有 `TotalInitialUSD` 字段。

- [ ] **Step 3：实现最小后端契约**

在 `TrafficCreditSummary` 增加字段：

```go
type TrafficCreditSummary struct {
	TotalInitialUSD   float64    `json:"total_initial_usd"`
	TotalRemainingUSD float64    `json:"total_remaining_usd"`
	NextExpiringUSD   float64    `json:"next_expiring_usd"`
	NextExpiresAt     *time.Time `json:"next_expires_at,omitempty"`
}
```

将 `GetSummary()` 的第一条查询改为同时汇总初始额和剩余额度：

```go
if err := r.db.QueryRowContext(ctx, `
	SELECT COALESCE(SUM(initial_usd), 0), COALESCE(SUM(remaining_usd), 0)
	FROM user_traffic_credits
	WHERE user_id = $1 AND platform = $2 AND remaining_usd > 0 AND expires_at > $3
`, userID, service.TrafficPackPlatformOpenAI, now).Scan(
	&summary.TotalInitialUSD,
	&summary.TotalRemainingUSD,
); err != nil {
	return nil, err
}
```

- [ ] **Step 4：运行后端目标测试，确认 GREEN**

运行同 Step 2 命令。

预期：两个目标测试全部通过。

- [ ] **Step 5：运行流量包相邻后端测试**

运行：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/repository -run 'TrafficPack|TrafficCredit'
```

预期：全部通过。

- [ ] **Step 6：提交后端契约修改**

```bash
git add backend/internal/service/traffic_pack.go backend/internal/repository/traffic_pack_repo.go backend/internal/repository/traffic_pack_repo_test.go
git commit -m "fix: expose traffic credit initial total"
```

### Task 2：修正订阅页流量卡进度展示

**文件：**

- 修改：`frontend/src/types/payment.ts`
- 修改：`frontend/src/views/user/SubscriptionsView.vue`
- 修改：`frontend/src/views/user/__tests__/SubscriptionsView.spec.ts`
- 修改：`frontend/src/views/user/__tests__/PaymentView.spec.ts`

- [ ] **Step 1：先补部分消费与耗尽隐藏的失败测试**

给现有未消费 fixture 增加：

```ts
total_initial_usd: 10,
```

新增部分消费场景，返回：

```ts
traffic_credit_summary: {
  total_initial_usd: 10,
  total_remaining_usd: 7,
  next_expiring_usd: 7,
  next_expires_at: '2027-06-26T08:57:24+08:00',
},
```

断言：

```ts
expect(wrapper.text()).toContain('$3.00 / $10.00')
expect(wrapper.find('[data-testid="traffic-credit-progress"]').attributes('style')).toContain('width: 30%')
```

新增耗尽场景，返回 `total_initial_usd: 0`、`total_remaining_usd: 0`，断言不存在 `[data-testid="traffic-credit-progress"]`，并显示无订阅空状态。

- [ ] **Step 2：运行前端目标测试，确认 RED**

运行：

```bash
pnpm --dir frontend exec vitest run src/views/user/__tests__/SubscriptionsView.spec.ts
```

预期：部分消费测试仍得到 `$0.00 / $7.00` 和 `width: 0%`，断言失败。

- [ ] **Step 3：扩展前端类型并实现最小展示修复**

在 `TrafficCreditSummary` 增加：

```ts
total_initial_usd: number
```

将 `trafficCreditUsageRows` 改为：

```ts
const trafficCreditUsageRows = computed<SubscriptionUsageRow[]>(() => {
  const total = trafficCreditSummary.value?.total_initial_usd ?? 0
  const remaining = trafficCreditSummary.value?.total_remaining_usd ?? 0
  const used = Math.max(total - remaining, 0)
  return [{
    label: '总计',
    value: `$${used.toFixed(2)} / $${total.toFixed(2)}`,
    progressWidth: getProgressWidth(used, total),
    progressClass: getProgressBarClass(used, total),
    testId: 'traffic-credit-progress',
  }]
})
```

给 `PaymentView.spec.ts` 的流量卡汇总 fixture 补充与数据一致的 `total_initial_usd`，不修改购买页生产逻辑。

- [ ] **Step 4：运行前端目标测试，确认 GREEN**

运行同 Step 2 命令。

预期：订阅页全部测试通过。

- [ ] **Step 5：运行前端相邻测试和类型检查**

运行：

```bash
pnpm --dir frontend exec vitest run src/views/user/__tests__/SubscriptionsView.spec.ts src/views/user/__tests__/PaymentView.spec.ts
pnpm --dir frontend run typecheck
```

预期：全部通过，无 TypeScript 错误。

- [ ] **Step 6：提交前端展示修改**

```bash
git add frontend/src/types/payment.ts frontend/src/views/user/SubscriptionsView.vue frontend/src/views/user/__tests__/SubscriptionsView.spec.ts frontend/src/views/user/__tests__/PaymentView.spec.ts
git commit -m "fix: correct traffic credit progress display"
```

### Task 3：完整验证并记录结果

**文件：**

- 新建：`docs/ai/context/YYYYMMDD-HHMMSS-traffic-credit-progress-fix-result_CN.md`
- 修改：`AGENTS.md`

- [ ] **Step 1：运行后端相关包完整单元测试**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/repository ./internal/service
```

- [ ] **Step 2：运行前端目标测试、类型检查和构建**

```bash
pnpm --dir frontend exec vitest run src/views/user/__tests__/SubscriptionsView.spec.ts src/views/user/__tests__/PaymentView.spec.ts
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
```

- [ ] **Step 3：检查补丁质量与变更范围**

```bash
git diff --check HEAD~2..HEAD
git status --short
git diff --stat main...HEAD
```

确认没有数据库迁移、支付流程、扣费逻辑或运行态配置改动。

- [ ] **Step 4：新建结果文档并更新项目记忆**

记录 RED/GREEN 证据、最终测试结果、字段契约、历史耗尽语义保持情况和未部署范围；只创建新文档，不覆盖历史文件。

- [ ] **Step 5：提交结果文档**

```bash
git add AGENTS.md docs/ai/context/*traffic-credit-progress-fix-result_CN.md
git commit -m "docs: record traffic credit progress fix"
```
