# OpenAI 套餐与流量卡计费优先级排查

## 背景

- 排查用户：`1510623550@qq.com`，运行态用户 ID 为 `41`，API Key ID 为 `45`。
- 现象：OpenAI `/v1/responses` 普通请求返回 402，错误为 `traffic credit is insufficient for request budget`。
- 运行态：`sub2api-candidate`，镜像创建时间约为 `2026-07-18 13:20 +08`。
- 本地当前分支为 `main`，相对 `personal/main` 领先 1 个提交：`fix(openai): correct preauth budget token estimate`。

## 运行态事实

- 用户状态 active，余额 `6.32`。
- 当前 active OpenAI 订阅：
  - `user_subscriptions.id=110`
  - `group_id=9`
  - group 为 `codex-pool-69-usd`
  - `daily_limit_usd=69`
  - 当前查询时 `daily_usage_usd` 约 `1.33`，日剩余额度约 `67.67`
- 用户流量卡：
  - 两张 OpenAI 流量卡共购买 20 USD。
  - 当前仅剩一张 `remaining_usd=0.00346455`，另一张为 0。
- 402 日志集中在 `2026-07-18 20:23:40 +08` 到 `21:14:50 +08`，均为 `traffic credit is insufficient for request budget`。
- `2026-07-18 21:25:00 +08` 管理员调用了 `POST /api/v1/admin/subscriptions/110/reset-quota`。
- 重置后该用户 `/v1/responses` 多条成功，`usage_logs.billing_type=1`、`subscription_id=110`，`usage_facts.effects.is_subscription=true`、`is_traffic_credit=false`。

## 当前分支与 personal/main 的设计对比

### 分组选择层

- `backend/internal/service/effective_group_resolver.go`
- `ResolveEffectiveGroup()` 对 OpenAI 平台先调用 `resolveOpenAISubscription()`。
- 有 active subscription 时直接返回 subscription group；没有 subscription 时才检查流量卡并返回内部流量卡分组。
- 测试 `TestEffectiveGroupResolver_SubscriptionBeatsTrafficPack` 固化了“分组解析层套餐优先”。

### 请求前预授权层

- `backend/internal/service/openai_billing_authorization.go`
- `Authorize()` 的实际策略不是“只要有套餐就永远用套餐”：
  1. 如果存在 `Subscription` 和 `Group`，先估算本次请求预算 `budget.ReserveUSD`。
  2. 只有 `subscription.CheckAllLimits(group, budget.ReserveUSD)` 全部通过，才返回 `BillingSourceSubscription`。
  3. 如果套餐额度检查不通过，代码继续进入流量卡预留逻辑。
  4. 流量卡余额不足时返回 `ErrTrafficCreditInsufficient`，最终映射为 402。
- 测试 `TestOpenAIBillingAuthorization_ReservesTrafficCreditWhenSubscriptionExceeded` 固化了“套餐预算不通过后可以兜底流量卡”的行为。

### 结算层

- `backend/internal/service/openai_gateway_service.go`
- 成功响应后如果 `BillingAuthorization.Source == BillingSourceTrafficCredit`：
  - 设置 `useTrafficPack=true`
  - 清空 `subscription`
  - `isSubscriptionBilling=false`
- `backend/internal/service/gateway_service.go`
- `buildUsageBillingCommand()` 中 `UseTrafficPack` 会写入 `TrafficPackCost` 和 `TrafficCreditReservationID`，不会写入 `SubscriptionCost`。
- `backend/internal/repository/usage_billing_repo.go`
- 看到 `TrafficPackCost > 0` 后按流量卡 reservation 或流量卡余额扣费。

### 当前分支相对 personal/main 的唯一相关差异

- `backend/internal/service/openai_traffic_credit_budget.go`
- `personal/main` 使用 `inputTokens := len(body)`，会把 JSON body 字节数，包括 base64 / image URL 传输载荷，当作输入 token 上限，容易把预算估得过高。
- 当前本地 `main` 改为 `estimateOpenAIRequestTextTokenUpperBound(body)`，只估算 JSON 文本，并跳过图片/base64/file 载荷。
- 该提交只修预算估算过高的问题，没有改变“套餐额度检查失败后自动尝试流量卡”的业务策略。

## 结论

- 当前代码的真实设计是两层优先级：
  - 分组选择：套餐优先于流量卡。
  - 预授权/结算：套餐预算能覆盖本次请求时用套餐；套餐预算不通过时自动尝试流量卡。
- 这与期望的“active 套餐存在且仍有金额时，应优先且固定按套餐计费，不应直接消耗流量卡”不完全一致。
- 本次 402 的直接原因是请求前授权进入了流量卡路径，而该用户流量卡只剩 `0.00346455 USD`，不足以覆盖请求预算。
- 重置订阅额度后，后续请求已按套餐计费成功，说明当前运行态不是账号、key 或上游不可用导致的 402。

## 后续建议

- 如果业务规则确定为“active subscription 存在时禁止自动消耗流量卡”，应修改 `OpenAIBillingAuthorizationService.Authorize()`：
  - 有 active subscription 时，预算检查通过则返回 `BillingSourceSubscription`。
  - 预算检查不通过则返回明确的 subscription quota 不足错误，不进入 traffic credit fallback。
  - 仅无 active subscription 时才允许流量卡预授权。
- 同步调整单测，移除或改写 `ReservesTrafficCreditWhenSubscriptionExceeded` 的期望。
