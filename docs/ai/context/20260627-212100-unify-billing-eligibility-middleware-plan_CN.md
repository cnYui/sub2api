# 认证中间件统一计费准入修复计划

## 目标

修复“订阅日额度耗尽后，OpenAI/GPT 流量卡无法接管扣费”的问题。认证中间件不再把订阅额度耗尽当作最终拒绝条件，最终额度、余额、流量卡兜底和限流判断统一交给 `BillingCacheService.CheckBillingEligibility()`。

## 设计

- 保留认证中间件的身份、API Key 状态、订阅存在性、订阅过期和暂停校验。
- 认证中间件遇到 `ErrDailyLimitExceeded` / `ErrWeeklyLimitExceeded` / `ErrMonthlyLimitExceeded` 时不再直接 429，而是继续把 subscription 放入 context。
- handler 内现有 `BillingCacheService.CheckBillingEligibility()` 负责最终判断：
  - 订阅额度仍可用：按订阅放行。
  - 订阅额度不足且 OpenAI 流量卡可用：放行并在后扣阶段扣流量卡。
  - 订阅额度不足且无流量卡：返回原有超限错误。
- 非订阅模式的余额不足也不应在认证中间件直接拒绝，否则“无订阅但有 OpenAI 流量卡”仍会被挡在统一准入前。
- Google API Key 中间件走同样策略，避免另一条入口保留旧行为。

## 实施步骤

1. 先补中间件单测：订阅额度错误不 abort，请求可进入下游 handler，且 subscription 仍在 context。
2. 补非订阅余额不足单测：中间件不直接 abort，留给后续统一计费准入。
3. 修改 `backend/internal/server/middleware/api_key_auth.go`：对订阅限额错误继续放行，对非订阅余额不足不再直接 abort。
4. 修改 `backend/internal/server/middleware/api_key_auth_google.go`：对订阅限额错误继续放行，对非订阅余额不足不再直接 abort。
5. 跑目标单测，再跑相关 service 流量包单测。
6. 写结果上下文，并视情况更新 `AGENTS.md` 长期记忆。

## 验证命令

```bash
go test ./internal/server/middleware -run 'TestAPIKeyAuth(AllowsSubscriptionQuotaExceededForUnifiedBilling|AllowsZeroBalanceForUnifiedBilling|GoogleAllowsSubscriptionQuotaExceededForUnifiedBilling|GoogleAllowsZeroBalanceForUnifiedBilling)'
go test -tags=unit ./internal/service -run 'TestBillingCacheServiceAllowsOpenAITrafficPackWhenBalanceEmpty|TestBuildUsageBillingCommand_UsesTrafficPackInsteadOfBalance|TestPlanTrafficCreditDeductions'
```
