# 流量卡与周额度耗尽链路检查

时间：2026-07-23 00:15:57 JST

## 问题

检查当前项目在公共 Codex 套餐从每日刷新改为每 7 天滚动周额度后：

- 用户用完当前周额度后，是否还能使用流量卡；
- 流量卡用完后，是否仍按既有设计“用完就消失”。

## 结论

- 流量卡耗尽后的不可用/不可见机制仍在：扣费只更新 `user_traffic_credits.remaining_usd`，不会物理删除卡；当余额小于等于 `billing.traffic_credit_minimum_reserve_usd`（默认 0.01 USD）时，`HasAvailableCredit`、`GetSummary`、`ListUserCredits` 都会过滤掉该卡，因此用户侧可用列表和汇总里会“消失”。
- 流量卡跨过耗尽阈值时会写入 `traffic_credit_exhaustion_events`，`/api/v1/auth/me` 返回待确认事件，前端只弹一次“流量卡已用完”并批量 ack。
- 当前本机 `sub2api-dev` 运行态开启了请求前流量卡预授权：`BILLING_TRAFFIC_CREDIT_RESERVATION_ENABLED=true`、`BILLING_TRAFFIC_CREDIT_RESERVATION_SHADOW=false`。
- 重要风险：开启预授权后，OpenAI rolling weekly 订阅如果本次预算会超过当前周窗口，`OpenAIBillingAuthorizationService.Authorize` 会直接返回 `ErrWeeklyLimitExceeded`，没有落到后面的流量卡预留分支。因此“周额度用完后自动切流量卡”在当前开发态/OpenAI 预授权路径下可能被阻断。

## 依据

- `backend/internal/repository/traffic_pack_repo.go`
  - `GetSummary` / `ListUserCredits` / `HasAvailableCredit` 均要求 `remaining_usd > minimumReserve`、可用余额大于 0、未过期。
  - `decrementTrafficCredit` 只扣减 `remaining_usd`，不删除 `user_traffic_credits` 行。
- `backend/internal/repository/usage_billing_repo.go`
  - 无预授权结算路径和预授权结算路径都扣 `remaining_usd` 并写 `traffic_credit_ledger`。
  - 预授权部分结算会把接近 0 的余额归零。
- `backend/internal/repository/traffic_credit_exhaustion_repo.go`
  - `recordTrafficCreditExhaustion` 只在从未耗尽跨到耗尽时插入事件，并用 `(user_id, credit_id)` 去重。
- `backend/internal/service/openai_billing_authorization.go`
  - 普通订阅超额会继续尝试流量卡预留。
  - rolling weekly 分支在 `window.Allows(weeklyUsage, budget.ReserveUSD)` 为 false 时直接返回 `ErrWeeklyLimitExceeded`，没有继续尝试流量卡。
- `backend/internal/service/billing_cache_service.go`
  - 老的 `CheckBillingEligibility` 仍在订阅 eligibility 失败时调用 `canUseTrafficPackCredit` 放行。

## 已验证

```bash
go test -tags unit ./internal/service -run "Test(OpenAIBillingAuthorization|PlanTrafficCredit|BillingCacheServiceAllowsOpenAITrafficPack|ShouldBillWithTrafficPack|UsageBillingCommandBuilder)" -count=1
go test -tags unit ./internal/repository -run "Test.*Traffic" -count=1
go test -tags unit ./internal/handler -run "TestTrafficCredit|TestAuthHandlerGetCurrentUserIncludesTrafficCreditExhaustionNotice" -count=1
```

三组测试均通过。

## 后续建议

- 若业务要求明确是“周额度耗尽后继续用流量卡”，应补一个 rolling weekly + 预授权开启场景测试，并调整 `OpenAIBillingAuthorizationService.Authorize`：周额度预算不满足时继续走流量卡分支，而不是直接返回 `ErrWeeklyLimitExceeded`。
- “流量卡用完就消失”当前是展示/可用性消失，不是删除历史行；保留历史行是正确的审计设计。
