# 计费预检保底额度与线上复测

## 背景

成功响应后的结算事务已经改为无论流量卡是否足额覆盖都写入用量记录，并允许普通余额形成欠费。复测发现认证层仍将“普通余额大于零”以及“流量卡额度大于零”视为可放行条件，导致余额和流量卡只剩碎额时仍可请求上游。

## 处理

- `api_key_auth.go` 认证预检统一使用 `Billing.MinimumBalanceReserve`；普通余额低于阈值时，只有可用流量卡达到相同阈值才放行。
- `billing_cache_service.go` 的流量卡判断改为读取额度汇总，要求 `TotalRemainingUSD >= MinimumBalanceReserve`，不再以“额度大于零”为准。
- `deploy/docker-compose.18082.yml` 为 18082 设置 `BILLING_MINIMUM_BALANCE_RESERVE=0.01`。该值只控制请求前放行，不改变成功请求的实际结算金额。
- 保留成功后的余额负数机制：单次已放行请求可使余额转负；之后普通余额和可用流量卡都低于保底值时，后续请求直接拒绝。

## 验证

2026-08-05 在公网入口关闭、本地 `127.0.0.1:18082` 上使用已恢复集合中的 API Key 发起最小真实请求：

| Key ID | 请求结果 | 用量日志 | 余额/流量卡结果 |
| --- | --- | --- | --- |
| 167 | `403` | `2385 -> 2385` | 普通余额 `$0.00198299`、流量卡 `$0.00065645` 均未改变 |
| 201 | `403` | `394 -> 394` | 普通余额 `$0.00173528`、流量卡 `$0.00020520` 均未改变 |
| 210 | `200` | `21 -> 22` | `usage_logs.id=294009`，`actual_cost=$0.00189000`；普通余额 `$0.43456951 -> $0.43267951`，差额完全一致 |
| 121 | `502` | `2329 -> 2329` | 上游失败，未写入用量、未扣用户余额或流量卡 |

认证与计费单测均通过：

```text
go test ./internal/server/middleware -run "TestAPIKeyAuthRejectsBalanceBelowMinimumReserve|TestApiKeyAuthWithSubscriptionGoogle_BalanceBelowMinimumReserve" -count=1
go test -tags=unit ./internal/service -run "TestCanUseTrafficPackCredit_(RejectsCreditBelowMinimumReserve|AllowsCreditAtMinimumReserve)|TestCheckBillingEligibility_(RejectsBalanceBelowMinimumReserve|AllowsBalanceAtMinimumReserve)" -count=1
```

## 结论

余额/流量卡碎额绕过预检的入口已关闭。成功请求仍会留下 `usage_logs` 并落账；余额耗尽后的后续请求不再进入上游，因此不会出现成功请求却无账单的缺口。
