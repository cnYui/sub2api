# 订阅取消后流量卡兜底修复计划

时间：2026-06-27 22:14 JST

## 背景

公网实测 `2799523972@qq.com` 的 API Key：

- 用户余额为 0。
- 订阅已软删除。
- 仍有 OpenAI 流量卡 15 USD。
- 请求 `https://api.aaccx.pw/v1/chat/completions` 返回 `403 SUBSCRIPTION_NOT_FOUND`。
- 流量卡未扣费，`usage_logs` 和 `billing_usage_entries` 未写入。

根因：认证中间件在订阅型 group 下找不到 active subscription 时直接 403，未进入 handler 的 `BillingCacheService.CheckBillingEligibility()`。

## 目标

让订阅已取消但仍有 OpenAI 流量卡的用户，可以用原订阅型 OpenAI API Key 发起请求，并由统一计费入口和后置账单路径使用流量卡扣费。

## 非目标

- 不改变无流量卡用户的拒绝行为。
- 不让非 OpenAI 平台使用 OpenAI 流量卡。
- 不手工修改用户余额、流量卡、订阅或 API Key。
- 不记录完整 API Key。

## 实现策略

1. 在 `backend/internal/server/middleware/api_key_auth.go` 中，订阅型 group 找不到 active subscription 时不再直接拒绝，而是保留 `subscription=nil` 继续执行后续通用计费检查。
2. 后续 `BillingCacheService.CheckBillingEligibility()` 会按非订阅模式检查余额；余额不足时，如果平台为 OpenAI 且存在流量卡，则放行。
3. handler 里再次调用同一入口；后置账单由 `shouldBillWithTrafficPack()` 和 `TrafficPackCost` 执行真实扣费。
4. 同步调整 `api_key_auth_google.go` 的同类订阅缺失硬拒绝，避免 Google 风格入口行为分叉。
5. 先补单元测试，确认当前代码红灯，再实现最小改动。

## 验证

1. 单元测试：
   - `go test -count=1 -tags=unit ./internal/server/middleware`
   - `go test -count=1 -tags=unit ./internal/service`
2. 构建镜像并替换 18085 测试应用容器。
3. 在 18085 对同类状态做健康检查。
4. 构建/标记并只替换 18084 `sub2api-candidate` 应用容器，保留 DB/Redis/nginx。
5. 使用 `2799523972@qq.com` 的真实公网 API Key 重新发起最小请求，确认：
   - HTTP 200。
   - `user_traffic_credits.remaining_usd` 下降。
   - `traffic_credit_ledger` 新增 `deduction`。
   - `usage_logs`/`billing_usage_entries` 新增记录。
