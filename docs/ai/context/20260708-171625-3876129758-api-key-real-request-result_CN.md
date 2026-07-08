# 3876129758@qq.com API Key 真实请求验证结果

时间：2026-07-08 17:16 JST / 2026-07-08 16:16 东八区

## 请求

- 用户：`3876129758@qq.com`
- Key：`api_keys.id=75`，仅确认掩码 `sk-c7b2c6c...5607`
- 入口：`POST https://api.aaccx.pw/v1/responses`
- 模型：`gpt-5.5`
- 请求内容：要求模型用中文回复真实请求测试 OK

## 响应

- HTTP 状态：`200`
- Responses ID：`resp_085b470fa3d84f36016a4e0759f74c81918b3273ae81ca6378`
- Responses 状态：`completed`
- 模型回复：`收到。真实请求测试 OK。`
- 返回 usage：`input_tokens=4704`、`cached_tokens=4352`、`output_tokens=11`、`total_tokens=4715`

## 落库与扣费

- 请求前该用户最后 `usage_logs.id=65958`
- 新增 `usage_logs.id=66004`
- `request_id=client:4ca6f238-3893-4640-b423-c7e1f0f31034`
- `model/requested_model=gpt-5.5`
- `inbound_endpoint=/v1/responses`
- `upstream_endpoint=/v1/responses`
- `total_cost=0.0042660000 USD`
- `actual_cost=0.0042660000 USD`
- `billing_mode=token`
- 扣费订阅：`user_subscriptions.id=90`
- 扣费分组：`groups.id=4 / codex-pool-49-usd`
- 订阅日用量更新后：`0.4444975000 / 49.00000000 USD`
- 流量卡流水：本次 `request_id` 对应 `traffic_credit_ledger` 为 0 条，未走流量卡兜底。

## 备注

- 该用户当前有两个 active 订阅：`id=76/codex-pool-19-usd` 和 `id=90/codex-pool-49-usd`。
- 这次自动 Key 的真实请求实际扣到了截图对应的 `codex-pool-49-usd`。
- 全程未在文档或回复中记录完整 API Key。
