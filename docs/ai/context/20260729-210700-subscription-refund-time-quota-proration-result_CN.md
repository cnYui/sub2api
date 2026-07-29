# 订阅退款时间与额度双比例结果记录

## 结果

订阅退款报价统一改为：

```text
时间消耗比例 = clamp((当前时间 - 权益开始时间) / (权益结束时间 - 权益开始时间), 0, 1)
额度消耗比例 = clamp(已用额度 / 周期总额度, 0, 1)
实际消耗比例 = max(时间消耗比例, 额度消耗比例)
预计退款 = 订单本金 × (1 - 实际消耗比例)
```

手续费继续不退。实际消耗比例为 100% 时，预计退款为 0，自动退款入口拒绝执行；起止时间或额度数据无效时，维持人工审核。

## 根因

对订单 #60 的只读核查显示，原报价仅按额度消耗比例计算：已用额度约 49.94 USD、周期总额度 1032 USD，得到约 4.84% 的消耗比例并错误报价 94.21 元。该订单的权益周期已经接近结束，原逻辑遗漏了约 29.9 天的时间消耗，导致退款金额接近全额。

## 改动

- 后端权益查询补充 `starts_at`、`expires_at`，报价返回并审计时间消耗比例、额度消耗比例与实际消耗比例。
- 退款执行仍在事务内重新计算同一份报价，避免用户看到的报价与最终退款依据分离。
- 用户和管理员退款弹窗展示时间消耗比例与实际消耗比例；前端不自行计算退款金额。
- 回归测试覆盖时间优先、额度优先、到期不可自动退款、无效时间范围和既有退款入口。

## 验证

2026-07-29 执行并通过：

```powershell
go test -tags unit ./internal/service -run 'Test(AdminSubscriptionRefundQuote(UsesElapsedTimeWhenHigherThanQuota|UsesQuotaWhenHigherThanElapsedTime|AtExpiryIsNotEligible)|CalculateRefundTimeRatioRejectsInvalidEntitlementWindow|PrepareRefundUsesSubscriptionQuoteAndPersistsBasis|ExecuteRefundRecalculatesAdminSubscriptionQuoteInsideTransaction|RequestRefundAutomaticallyRefundsAlipaySubscriptionWithoutFeeAndRevokesSubscription|RequestRefundAutomaticallyRefundsBalanceSubscriptionWithoutFee)$' -count=1
pnpm exec vitest run src/views/user/__tests__/UserOrdersView.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts
pnpm typecheck
```

前端 Vitest 结果为 2 个测试文件、10 项断言通过；类型检查通过。`pnpm typecheck` 仅输出现有 Browserslist 数据过期提示。

完整后端服务测试执行：

```powershell
go test -tags unit ./internal/service -count=1
```

未通过，独立复现为 `TestSettingService_GetAuthSourceDefaultSettings_ParsesValuesAndDefaults`：测试期望空切片 `[]service.DefaultSubscriptionSetting{}`，实际得到 `nil`。该测试位于 `setting_service_auth_source_defaults_test.go`，与本次退款报价、权益时间或前端改动没有调用关系；本次不扩大范围修改它。

## 边界

未修改运行态数据库、Redis、容器、支付网关或公网链路。
