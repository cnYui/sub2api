# 滚动周额度流量卡兜底实施结果

时间：2026-07-23 08:27:16 JST

关联文档：

- 设计：`docs/ai/context/20260723-002254-rolling-weekly-quota-traffic-credit-fallback-design_CN.md`
- 计划：`docs/ai/context/20260723-081317-rolling-weekly-quota-traffic-credit-fallback-plan_CN.md`

## 完成内容

- 修改 `OpenAIBillingAuthorizationService.Authorize` 的滚动周额度授权分支：仅在当前周额度足够时立即返回 `subscription`；额度不足时继续进入既有流量卡 reservation 分支。
- 保留缺少有效滚动周窗口事实时的旧订阅有效性校验，避免把“有效但已耗尽”的周窗口误判为 `ErrSubscriptionInvalid`。
- 新增滚动周额度不足场景的单元测试：
  - 有足额流量卡时返回 `BillingSourceTrafficCredit` 并创建 reservation。
  - 流量卡不可用时返回 `ErrTrafficCreditInsufficient`，不再返回 `ErrWeeklyLimitExceeded`。

## 未变更内容

- 未关闭 `BILLING_TRAFFIC_CREDIT_RESERVATION_ENABLED`，请求前预授权、durable usage fact、债务门禁和 reservation 结算继续生效。
- 未修改数据库 schema、环境变量、部署配置、流量卡扣费或耗尽事件逻辑。
- 周额度足够时仍优先使用套餐，单个请求不会同时使用套餐和流量卡。

## 验证

通过：

```bash
go test -v -tags unit ./internal/service -run "TestOpenAIBillingAuthorization_(ReservesTrafficCreditWhenRollingWeeklyQuotaExceeded|ReturnsTrafficCreditErrorWhenRollingWeeklyQuotaExceeded|RollingWeeklyIgnoresStaleWindowUsage|UsesSubscriptionWhenBudgetFits)$" -count=1
go test -tags unit ./internal/service -run "Test(OpenAIBillingAuthorization|PlanTrafficCredit|BillingCacheServiceAllowsOpenAITrafficPack|ShouldBillWithTrafficPack|UsageBillingCommandBuilder)" -count=1
go test -tags unit ./internal/repository -run "Test.*Traffic" -count=1
go test -tags unit ./internal/handler -run "TestTrafficCredit|TestAuthHandlerGetCurrentUserIncludesTrafficCreditExhaustionNotice" -count=1
```

`go test -tags unit ./internal/service -count=1` 未通过，但失败于既有的 `TestSettingService_GetAuthSourceDefaultSettings_ParsesValuesAndDefaults`：测试期望空切片，实际为 `nil`。失败不涉及本次修改的授权服务或测试文件。

## 运行态

本次仅修改本地代码和测试，没有执行容器、数据库、Redis、Nginx、环境变量或公网链路操作。
