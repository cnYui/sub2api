# 订阅取消后流量卡兜底修复结果

时间：2026-06-27 22:27 JST

## 目标

修复 `2799523972@qq.com` 这类“订阅已取消、余额为 0、仍有 OpenAI/GPT 流量卡”的用户无法使用 API Key 的问题。

## 根因

认证中间件在订阅型 group 找不到 active subscription 时直接返回 `403 SUBSCRIPTION_NOT_FOUND`，请求未进入 handler 中的 `BillingCacheService.CheckBillingEligibility()`，因此流量卡兜底逻辑没有机会执行。

## 代码修改

- `backend/internal/server/middleware/api_key_auth.go`
  - 订阅型 group 找不到 active subscription 时，不再直接 403。
  - 仅当订阅加载出现非 `ErrSubscriptionNotFound` 错误时返回 `500 SUBSCRIPTION_LOAD_FAILED`。
  - 找不到订阅时不写入 subscription context，让 handler 的统一计费入口决定余额或 OpenAI 流量卡是否可兜底。
- `backend/internal/server/middleware/api_key_auth_google.go`
  - 同步移除 Google/Gemini 风格入口的订阅缺失硬拒绝。
  - 只有存在 subscription 时才执行订阅状态和窗口维护。
- `backend/internal/server/middleware/api_key_auth_test.go`
  - 新增普通 API Key 中间件回归测试：订阅缺失应放行到统一计费入口。
  - 新增 Google 风格中间件回归测试：订阅缺失应放行到统一计费入口。

## TDD 验证

红灯：

- `go test -count=1 -tags=unit ./internal/server/middleware -run 'TestSimpleModeBypassesQuotaCheck/standard_mode_defers_missing_subscription_to_unified_billing'`
  - 修改前失败：期望 200，实际 403。
- `go test -count=1 -tags=unit ./internal/server/middleware -run 'TestAPIKeyAuthGoogleDefersMissingSubscriptionToUnifiedBilling'`
  - 修改前失败：期望 200，实际 403。

绿灯：

- `go test -count=1 -tags=unit ./internal/server/middleware` 通过。
- `go test -count=1 -tags=unit ./internal/service` 通过。

## 部署

构建镜像：

- 源镜像：`sub2api-traffic-card-fix:20260627-221441`
- 镜像 digest：`sha256:299560875687ba0fc7c9b9703a5bece639a832c35720fb6ce47f8dd222483e22`

替换 18085：

- `sub2api-smtp-test:20260627-221441-traffic-card-fix`
- 保留 `sub2api-smtp-test-postgres` 和 `sub2api-smtp-test-redis`
- `http://127.0.0.1:18085/health` 返回 `{"status":"ok"}`

替换 18084：

- `sub2api-candidate:20260627-221441-traffic-card-fix`
- 保留 `sub2api-candidate-postgres` 和 `sub2api-candidate-redis`
- nginx 指向仍为 `127.0.0.1:18084`
- `http://127.0.0.1:18084/health` 返回 `{"status":"ok"}`
- `http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`
- `https://api.aaccx.pw/health` 返回 `{"status":"ok"}`

## 公网真实验证

用户：

- `2799523972@qq.com`
- `users.id=31`
- API Key：`api_keys.id=37`
- Key 脱敏：`sk-10f93...6695c6`
- 订阅：已软删除，无 active subscription
- 用户余额：0
- 请求前 OpenAI 流量卡余额：`15.0000000000`

第一次修复后验证：

- `GET https://api.aaccx.pw/v1/models` 使用该 Key 返回 200，证明订阅缺失不再在认证层硬拒绝。

真实模型请求：

- Endpoint：`https://api.aaccx.pw/v1/responses`
- Model：`gpt-5.5`
- 请求结果：HTTP 200
- 响应状态：`completed`
- 用量：`input_tokens=4689`，`output_tokens=7`，`total_tokens=4696`

扣费结果：

- 请求后 OpenAI 流量卡余额：`14.9959290000`
- 新增 `traffic_credit_ledger`：
  - `entry_type=deduction`
  - `amount_usd=0.0040710000`
  - `balance_after_usd=9.9959290000`
  - `request_id=client:94183a80-0012-4ada-a2fe-9e0af8129cc3`
- 新增 `usage_logs`：
  - `model=gpt-5.5`
  - `actual_cost=0.0040710000`
  - `billing_type=0`

说明：

- 本次 `/v1/chat/completions` + `gpt-4o-mini` 曾返回 502，日志显示已进入 handler，失败原因是上游账号返回 502 后没有更多可用账号；这不是订阅认证拦截。
- 改用当前 `/v1/models` 可见的 `gpt-5.5` 和 `/v1/responses` 后，公网真实请求成功，并完成流量卡扣费。

## 结论

订阅取消后，仅剩 OpenAI/GPT 流量卡的用户现在可以继续使用原订阅型 OpenAI API Key；请求会进入统一计费入口，并在真实模型请求成功后从流量卡扣费。
