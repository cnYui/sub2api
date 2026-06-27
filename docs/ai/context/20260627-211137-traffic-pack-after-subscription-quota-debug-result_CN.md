# 流量卡在每日套餐额度耗尽后无法扣费排查结果

## 结论

根因不是流量卡表或扣费事务整体失效，而是订阅超限检查存在两条链路：

1. 认证中间件先执行 `SubscriptionService.ValidateAndCheckLimits()`。
2. handler 里稍后才执行 `BillingCacheService.CheckBillingEligibility()`，这里才有“订阅额度不足时用 OpenAI 流量卡兜底”的逻辑。

当用户订阅日额度已经用完后，请求会在认证中间件阶段被 `ErrDailyLimitExceeded` 拦截并返回 429，后面的流量卡兜底逻辑没有机会执行。

## 代码证据

- `backend/internal/server/middleware/api_key_auth.go`
  - 订阅模式下先加载 subscription。
  - 第 193-207 行附近调用 `subscriptionService.ValidateAndCheckLimits(subscription, apiKey.Group)`。
  - 如果返回 `ErrDailyLimitExceeded` / `ErrWeeklyLimitExceeded` / `ErrMonthlyLimitExceeded`，直接 `AbortWithError(..., 429, "USAGE_LIMIT_EXCEEDED", ...)`。
- `backend/internal/service/subscription_service.go`
  - `ValidateAndCheckLimits()` 第 888-897 行只按订阅自身限额判断，不查询 `TrafficPackService`。
- `backend/internal/service/billing_cache_service.go`
  - `CheckBillingEligibility()` 第 725-729 行才在订阅检查失败时调用 `canUseTrafficPackCredit()`。
  - 但这一步发生在 handler 内，晚于认证中间件。
- `backend/internal/service/gateway_service.go`
  - `shouldBillWithTrafficPack()` 具备“本次费用会使订阅超限则改用流量卡”的设计。
  - 前提是请求能走到后扣阶段；已超限后的新请求会先被中间件挡掉。

## 运行态证据

候选公网库 `sub2api-candidate-postgres` 中：

- `liyutong2883@gmail.com`：
  - `user_id=12`
  - 活跃订阅 `subscription_id=7`
  - 分组 `codex-pool-19-usd`
  - `daily_usage_usd=19.0741204000`
  - `daily_limit_usd=19.00000000`
  - 流量卡共 2 张，`initial_usd=20.0000000000`，`remaining_usd=20.0000000000`
  - `traffic_credit_ledger` 只有两条 `purchase`，没有 `deduction`
- 该用户 2026-06-26 最新 40 条 `usage_logs` 均为：
  - `billing_type=1`
  - `subscription_id=7`
  - 没有流量卡扣费记录
- 越界点：
  - 2026-06-26 15:36:43 累计 `18.9692144000`
  - 2026-06-26 15:37:08 本次 `0.1049060000`
  - 累计到 `19.0741204000`，仍记录为订阅计费

同时，其他用户已有成功扣流量卡的证据：

- `cnfoxian@gmail.com` 有 3 条 deduction，剩余 `9.8976433500`
- `changjunwang123@gmail.com` 有 53 条 deduction，剩余 `4.6534380000`
- `2246950894@qq.com` 有 70 条 deduction，剩余 `0.0046430000`

这说明流量卡扣费事务和表结构可用，问题集中在订阅超限前置拦截和切换时机。

## 修复方向

推荐把“订阅超限但存在 OpenAI 流量卡可用则放行”的判断前移到认证中间件，或更彻底地将中间件里的订阅额度检查统一替换为 `BillingCacheService.CheckBillingEligibility()` 的单一事实入口。

更稳的方案是：认证中间件只做身份、Key 状态、订阅存在性和过期状态校验；所有余额、订阅限额、流量卡兜底、RPM/API Key 限额都交给 `BillingCacheService.CheckBillingEligibility()`，避免双重计费判断继续分叉。

后续修复还需要补回归测试：

- 订阅日额度已满 + OpenAI 流量卡剩余 > 0：请求不应被认证中间件 429。
- 订阅本次请求会越过日额度 + OpenAI 流量卡剩余 > 0：本次应记为流量卡扣费，不应继续推高订阅日用量。
- 非 OpenAI 平台流量卡不兜底。
