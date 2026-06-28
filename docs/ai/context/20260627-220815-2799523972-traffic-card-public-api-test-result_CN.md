# 2799523972@qq.com 流量卡公网 API 实测结果

时间：2026-06-27 22:08 JST

## 目标

使用 `2799523972@qq.com` 的真实 API Key 通过公网 `https://api.aaccx.pw` 发起一次最小模型请求，验证用户订阅已取消、账户仅剩 OpenAI/GPT 流量卡时，流量卡是否能真实兜底使用并扣费。

## 用户与 Key 状态

- 用户：`2799523972@qq.com`
- `users.id=31`
- 用户状态：`active`
- 用户余额：`0.00000000`
- API Key：`api_keys.id=37`
- Key 脱敏：`sk-10f93...6695c6`
- Key 状态：`active`
- Key 绑定分组：`groups.id=2 / codex-pool-19-usd`
- 分组类型：`subscription`
- 分组平台：`openai`
- 分组每日额度：`19.00000000`

## 订阅与流量卡请求前快照

订阅：

- `user_subscriptions.id=49`
- `status=active`
- `deleted_at=2026-06-27 21:02:42.204818+08`
- 结论：订阅已软删除，不再是 active subscription。

流量卡：

- `user_traffic_credits.id=38`：`initial_usd=10.0000000000`，`remaining_usd=10.0000000000`
- `user_traffic_credits.id=45`：`initial_usd=5.0000000000`，`remaining_usd=5.0000000000`
- 有效 OpenAI 流量卡余额合计：`15.0000000000`

账单/用量：

- `traffic_credit_ledger`：2 条，均为 `purchase`
- `traffic_credit_ledger` deduction 合计：`0`
- `usage_logs`：0 条
- `billing_usage_entries`：0 条

## 公网真实请求

请求：

- Endpoint：`https://api.aaccx.pw/v1/chat/completions`
- Model：`gpt-4o-mini`
- Prompt：要求回复固定短文本
- `max_tokens=8`
- Authorization 使用该用户 API Key，但未打印或记录完整 Key。

响应：

```json
{
  "http_code": 403,
  "code": "SUBSCRIPTION_NOT_FOUND",
  "message": "No active subscription found for this group"
}
```

## 请求后快照

流量卡：

- `user_traffic_credits.id=38`：`remaining_usd=10.0000000000`
- `user_traffic_credits.id=45`：`remaining_usd=5.0000000000`
- 有效 OpenAI 流量卡余额合计仍为：`15.0000000000`

账单/用量：

- `traffic_credit_ledger`：仍为 2 条，均为 `purchase`
- `traffic_credit_ledger` deduction 合计仍为：`0`
- `usage_logs`：仍为 0 条
- `billing_usage_entries`：仍为 0 条

## 根因

请求没有进入 handler 里的 `BillingCacheService.CheckBillingEligibility()` 计费准入，也没有进入真实扣费链路。

当前代码在认证中间件仍保留“订阅型 group 必须存在 active subscription”的硬拒绝：

- `backend/internal/server/middleware/api_key_auth.go:153-162`

逻辑为：

1. API Key 绑定的 group 是 `subscription`。
2. 中间件调用 `subscriptionService.GetActiveSubscription(user_id=31, group_id=2)`。
3. 因用户订阅已软删除，查询失败。
4. 中间件直接返回 `403 SUBSCRIPTION_NOT_FOUND`。
5. 后续 handler 的 `BillingCacheService.CheckBillingEligibility()` 没有机会用流量卡兜底。

另外 Google 风格认证中间件也有同类订阅硬拒绝：

- `backend/internal/server/middleware/api_key_auth_google.go:81-90`

## 结论

本次公网真实请求证明：在“订阅已取消，只剩流量卡”的状态下，该用户当前 API Key 不能使用，流量卡没有被真实使用，也没有扣费。

之前已上线的修复覆盖了“订阅存在但额度耗尽后流量卡兜底”的路径；但没有覆盖“订阅不存在/已取消后，订阅型 group API Key 继续使用流量卡”的路径。

下一步需要决定产品语义：

1. 如果流量卡应允许订阅取消后的旧订阅 Key 继续使用 OpenAI，则认证中间件不能把 `SUBSCRIPTION_NOT_FOUND` 当最终拒绝，应在用户存在 OpenAI 流量卡时放行，并让 `BillingCacheService.CheckBillingEligibility()` 作为事实入口。
2. 如果订阅 Key 必须随订阅取消失效，则当前行为符合“订阅 Key 失效”的策略，但这与“账户里只有流量卡也能用 API Key”的预期不一致。
