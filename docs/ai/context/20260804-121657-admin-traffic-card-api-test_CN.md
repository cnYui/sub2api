# 管理员流量卡 API 实测记录

## 测试范围

- 实例：`http://127.0.0.1:18082`
- 账户：`xiaobianfuai@gmail.com` 的管理员用户，`users.id=448`
- API Key：已使用用户提供的管理员 Key；密钥内容不写入本文档、日志或验证结果
- 请求：`POST /v1/chat/completions`
- 请求标识：`codex-traffic-card-test-20260804-1`
- 请求参数：模型 `gpt-5.6-terra`，单条文本消息，`max_tokens=1`，非流式

## 实测结果

- HTTP 状态：`403`
- 返回码：`INSUFFICIENT_BALANCE`
- 返回消息：`Insufficient account balance`
- 请求前有效 OpenAI 流量卡剩余：`40.0000000000 USD`
- 请求后有效 OpenAI 流量卡剩余：`40.0000000000 USD`
- 请求前后管理员流量卡流水数量：`4 -> 4`
- 请求前后管理员用量记录数量：`50516 -> 50516`
- 结论：请求在 API Key 认证预检阶段被拒绝，没有进入上游，也没有产生流量卡扣费。

## 状态核对

- 管理员普通余额：`0.00000000`
- 管理员 API Key 绑定用户：`users.id=448`，状态为 `active`
- 有效 OpenAI 流量卡：4 批，每批 `10 USD`，合计 `40 USD`
- 因此本次失败不是额度不存在或已过期，而是入口预检没有接受流量卡作为余额不足时的可用资格。

## 直接原因

`backend/internal/server/middleware/api_key_auth.go` 的非订阅分支在余额低于阈值时直接返回 `INSUFFICIENT_BALANCE`，未调用流量卡可用性检查。虽然 `backend/internal/service/billing_cache_service.go` 已实现 OpenAI 流量卡资格检查，且后续结算代码支持写入 `traffic_credit_ledger`，但请求尚未到达该结算阶段。

## 后续处理建议

应在 API Key 认证预检中复用服务端的 OpenAI 流量卡可用性判断，再进行一次最小真实请求，核对 `user_traffic_credits.remaining_usd` 与 `traffic_credit_ledger.entry_type='deduction'`。本次未修改生产代码，也未调整管理员余额或流量卡数据。
