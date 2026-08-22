# 负余额自动切换流量卡规则修复

## 业务规则

流量卡只作为普通余额进入负数后的兜底资金池：

1. 普通余额大于等于 `0` 时，不切换流量卡；余额低于 `0.01 USD` 的正余额仍按普通余额保底阈值拒绝，不消耗流量卡。
2. 普通余额小于 `0` 且有效流量卡有净可用额度时，允许请求，后续结算从用户级全渠道流量卡额度池扣费。
3. 普通余额小于 `0` 且没有有效流量卡，或流量卡净额度不足（含流量卡欠费抵扣后不足）时，返回 `INSUFFICIENT_BALANCE`。
4. 规则对主 API、Gemini 原生 API 和 Antigravity Gemini 入口统一生效；`/v1/usage`、计费信息和异步生图查询仍按既有只读鉴权例外处理。

## 根因

主认证中间件在流量卡资格判断之前保留了负余额硬拦截，导致负余额用户永远无法进入 `CanUseTrafficPackCredit`。此前已有的后置判断只覆盖了余额保底阈值分支，因此修复没有真正覆盖负余额路径。Gemini 原生认证还存在同类硬拦截，且生产路由没有注入流量卡资格检查器。

## 实现

- `backend/internal/server/middleware/api_key_auth.go`：负余额先查询流量卡，只有无净额度时拒绝；非负余额不查询流量卡。
- `backend/internal/server/middleware/api_key_auth_google.go`：复用同一规则，并新增带计费缓存注入的构造函数。
- `backend/internal/server/routes/gateway.go`、`backend/internal/server/router.go`、`backend/internal/server/http.go` 和 `backend/cmd/server/wire_gen.go`：将生产 `BillingCacheService` 传入 Gemini/Antigravity 原生入口。
- `backend/internal/service/billing_cache_service.go`：流量卡自动切换条件收紧为 `balance < 0`；正余额低于保底阈值仍拒绝但不切卡。流量卡摘要中的 `TotalRemainingUSD` 已由仓库按流量卡欠费账本净额计算，净额不足时拒绝。
- 回归测试覆盖负余额有卡放行、负余额无卡拒绝、零余额/正余额低于阈值不切卡，以及 Gemini 入口的相同规则。

## 验证

已通过：

```powershell
go test -tags=unit ./internal/server/middleware -run "TestAPIKeyAuth(DoesNotSwitchTrafficPackAtZeroBalance|AllowsDebtWithTrafficPackFallback|SimpleModeStillRejectsDebt|RejectsDebtWithoutTrafficPack)|TestAPIKeyAuthGoogleSimpleModeStillRejectsDebt|TestAPIKeyAuthGoogleAllowsDebtWithTrafficPackFallback|TestApiKeyAuthWithSubscriptionGoogle_RejectsExhaustedBalance" -count=1
go test -tags=unit ./internal/service -run "TestCheckBillingEligibility|TestCheckFreshBalanceDebt|TestCanUseTrafficPackCredit" -count=1
go test -tags=unit ./internal/server/routes -run "TestGatewayRoutes" -count=1
```

本次仅完成源码和测试修复，尚未替换生产应用容器；生产发布仍需按既有约束只替换 `sub2api-official-18082` 并核验健康端点。
