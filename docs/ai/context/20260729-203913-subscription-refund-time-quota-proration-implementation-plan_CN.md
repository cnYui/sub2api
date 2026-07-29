# 订阅退款时间与额度双比例实施计划

> **执行要求：** 测试先行逐项完成；不修改运行态数据库、Redis、容器或支付网关。

**目标：** 订阅退款取时间消耗和额度消耗的较高比例，并在两个退款弹窗展示该依据。

**架构：** `payment_refund_quote.go` 保持为报价、审计依据和全部退款入口的唯一事实源。权益段提供起止时间和额度，Vue 仅展示服务端返回的比例和金额。

**技术栈：** Go、Ent/SQL、Vue 3、TypeScript、Vitest、testify。

---

### 任务 1：后端报价与审计依据

**文件：**
- 修改：`backend/internal/service/payment_refund_quote.go`
- 测试：`backend/internal/service/payment_refund_test.go`

- [ ] **步骤 1：写失败的报价测试**

在现有退款权益夹具新增以下测试：时间位于 28 天周期第 21 天且无 usage fact 时，退款 29 元本金的 7.25 元；额度比例高于时间比例时仍取额度比例；权益段到期时不具资格；起止时间相同则人工审核。核心断言：

```go
require.InDelta(t, 0.75, quote.TimeRatio, 1e-3)
require.InDelta(t, 0.75, quote.ConsumptionRatio, 1e-3)
require.InDelta(t, 7.25, quote.EstimatedRefundAmount, 1e-2)
require.True(t, quote.Eligible)
```

- [ ] **步骤 2：运行测试确认失败**

```powershell
go test -tags unit ./internal/service -run 'TestAdminSubscriptionRefundQuote(UsesElapsedTimeWhenHigherThanQuota|UsesQuotaWhenHigherThanElapsedTime|AtExpiryIsNotEligible|RequiresManualReviewForInvalidEntitlementWindow)$' -count=1
```

预期：编译失败，因为报价尚无 `TimeRatio`、`ConsumptionRatio`。

- [ ] **步骤 3：实现统一报价计算**

新增私有权益值对象和时间比例函数：

```go
type refundEntitlement struct {
    ID int64
    PeriodTotalQuotaUSD float64
    StartsAt time.Time
    ExpiresAt time.Time
}

func calculateRefundTimeRatio(startsAt, expiresAt, now time.Time) (float64, bool) {
    duration := expiresAt.Sub(startsAt)
    if duration <= 0 { return 0, false }
    return math.Min(math.Max(now.Sub(startsAt).Seconds()/duration.Seconds(), 0), 1), true
}
```

将权益查询扩展为读取 `id, period_total_quota_usd, starts_at, expires_at`。为报价添加 JSON 字段 `time_ratio`、`consumption_ratio`，并计算：

```go
quote.TimeRatio, validWindow = calculateRefundTimeRatio(entitlement.StartsAt, entitlement.ExpiresAt, quote.CalculatedAt)
quote.ConsumptionRatio = math.Max(quote.UsageRatio, quote.TimeRatio)
quote.EstimatedRefundAmount = math.Max(quote.PurchaseBaseAmount*(1-quote.ConsumptionRatio), 0)
quote.Eligible = quote.EstimatedRefundAmount > 0
```

权益缺失、总额度无效、时间范围无效、重叠或用量不能唯一归属时继续人工审核。`Basis()` 保存两个比例和权益段起止时间；手续费、订单本金、网关退款金额和资金拆分保持原语义。

- [ ] **步骤 4：验证后端**

```powershell
go test -tags unit ./internal/service -run 'TestAdminSubscriptionRefundQuote(UsesElapsedTimeWhenHigherThanQuota|UsesQuotaWhenHigherThanElapsedTime|AtExpiryIsNotEligible|RequiresManualReviewForInvalidEntitlementWindow)$' -count=1
go test -tags unit ./internal/service -run 'Test(PrepareRefundUsesSubscriptionQuoteAndPersistsBasis|ExecuteRefundRecalculatesAdminSubscriptionQuoteInsideTransaction|RequestRefundAutomaticallyRefundsAlipaySubscriptionWithoutFeeAndRevokesSubscription|RequestRefundAutomaticallyRefundsBalanceSubscriptionWithoutFee)$' -count=1
```

预期：PASS，所有入口仍消费同一事务内重算的报价。

### 任务 2：类型与两个退款弹窗

**文件：**
- 修改：`frontend/src/types/payment.ts`
- 修改：`frontend/src/i18n/locales/zh.ts`
- 修改：`frontend/src/i18n/locales/en.ts`
- 修改：`frontend/src/views/user/UserOrdersView.vue`
- 修改：`frontend/src/components/admin/payment/AdminRefundDialog.vue`
- 测试：`frontend/src/views/user/__tests__/UserOrdersView.spec.ts`
- 测试：`frontend/src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts`

- [ ] **步骤 1：写失败的弹窗测试**

两个报价 fixture 增加：

```ts
time_ratio: 0.75,
consumption_ratio: 0.75,
```

用户弹窗断言 `payment.refundQuote.timeRatio` 和 `payment.refundQuote.consumptionRatio` 出现；管理员弹窗断言两个 `75%` 和原服务端预计退款金额出现。

- [ ] **步骤 2：运行测试确认失败**

```powershell
pnpm exec vitest run src/views/user/__tests__/UserOrdersView.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts
```

预期：FAIL，页面还未渲染两个比例。

- [ ] **步骤 3：只展示服务端字段**

类型增加：

```ts
time_ratio: number
consumption_ratio: number
```

中英文退款报价增加 `timeRatio` 与 `consumptionRatio`。用户与管理员弹窗在额度使用比例后，分别以 `Math.round(refundQuote.time_ratio * 100)` 和 `Math.round(refundQuote.consumption_ratio * 100)` 展示百分比；不在前端计算退款。

- [ ] **步骤 4：验证前端**

```powershell
pnpm exec vitest run src/views/user/__tests__/UserOrdersView.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts
pnpm typecheck
```

预期：均 PASS。

### 任务 3：回归、记录与提交

**文件：**
- 新建：`docs/ai/context/20260729-203913-subscription-refund-time-quota-proration-result_CN.md`

- [ ] **步骤 1：运行相关回归**

```powershell
go test -tags unit ./internal/service -count=1
pnpm exec vitest run src/views/user/__tests__/UserOrdersView.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts
pnpm typecheck
git diff --check
```

- [ ] **步骤 2：新建结果记录**

记录公式、订单 #60 的只读根因、实际命令和结果；不记录用户邮箱、支付密钥、交易流水号或完整网关异常。

- [ ] **步骤 3：提交分支改动**

```powershell
git add backend/internal/service/payment_refund_quote.go backend/internal/service/payment_refund_test.go frontend/src/types/payment.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts frontend/src/views/user/UserOrdersView.vue frontend/src/views/user/__tests__/UserOrdersView.spec.ts frontend/src/components/admin/payment/AdminRefundDialog.vue frontend/src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts docs/ai/context/20260729-203913-subscription-refund-time-quota-proration-implementation-plan_CN.md docs/ai/context/20260729-203913-subscription-refund-time-quota-proration-result_CN.md
git commit -m "fix: 按时间和额度比例计算订阅退款"
```
