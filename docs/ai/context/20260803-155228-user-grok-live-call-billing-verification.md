# 用户 Grok 真实调用与扣费核验

时间：2026-08-03 15:52:28（Asia/Tokyo）

## 执行内容

已通过管理员界面为 `2799523972@qq.com` 增加 5 美元余额。变更前余额为 `0.00000000`，变更后调用前余额为 `5.00000000`。

使用该用户提供的 API Key 发起了一次最小成本的 `POST /v1/chat/completions` 调用：

- 请求模型：`grok-4.20-0309-non-reasoning`
- HTTP 状态：`200`
- 响应模型：`grok-4.20-0309-non-reasoning`
- 响应内容：`GROK_OK`

API Key 明文及上游凭据均未记录。

## 路由证据

此次调用对应 `usage_logs.id = 1`，记录显示：

- `user_id = 2`、`api_key_id = 1`
- `account_id = 1`（Grok 账号）
- `group_id = 3`（Grok 分组）
- `channel_id = 3`
- 请求、上游及记录模型均为 `grok-4.20-0309-non-reasoning`
- 入站端点为 `/v1/chat/completions`，上游端点为 `/v1/responses`

因此本次是成功返回结果的真实 Grok 路由，而非本地模拟响应。

## 扣费结果

调用后用户余额变为 `4.99988815`，实际扣减 `0.00011185` 美元。该数值与本条用量日志的 `actual_cost` 和 `total_cost` 均完全相同，证明余额扣费已经生效。

本次用量日志的倍率字段为：

- 分组/用户实际计费倍率 `rate_multiplier = 1.0000`
- 账号统计倍率 `account_rate_multiplier = 6.0000`
- 用户扣费与标准成本比例 `actual_cost / total_cost = 1.0`

`user_group_rate_multipliers` 中不存在该用户或该分组的覆盖记录，分组本身倍率也为 `1.0`。因此，本次没有执行 10 倍扣费，而是执行了 1 倍扣费。

## 计费语义

`backend/internal/service/billing_service.go` 将 `actual_cost` 计算为 `total_cost * rateMultiplier`；`backend/internal/service/gateway_usage_billing.go` 使用该 `actual_cost` 直接扣除用户余额。账号倍率 `6.0` 被用于账号口径统计，不能替代用户或分组的计费倍率。

`billing_usage_entries` 没有新增分录；本次标准余额计费由用量处理流程直接更新 `users.balance`，余额变化和用量日志是本次扣费成立的可复核证据。
