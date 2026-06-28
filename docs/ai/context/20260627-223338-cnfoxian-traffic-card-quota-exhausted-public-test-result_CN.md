# cnfoxian@gmail.com 套餐额度耗尽后流量卡公网实测结果

时间：2026-06-27 22:33 JST

## 目标

用户反馈 `cnfoxian@gmail.com` 今日套餐额度已用完，需要用该用户 API Key 发起一次公网真实模型请求，并确认其 10 USD OpenAI/GPT 流量卡是否真实扣费。

## 公网运行态确认

- 当前公网应用容器：`sub2api-candidate`
- 当前镜像：`sub2api-candidate:20260627-221441-traffic-card-fix`
- 端口：`127.0.0.1:18084->8080`
- 状态：healthy
- 说明：这是当前本地 `main` 工作树构建出的修复镜像，包含尚未提交的中间件流量卡兜底修复。

## 用户与 Key

- 用户：`cnfoxian@gmail.com`
- `users.id=40`
- 用户状态：`active`
- 用户余额：`0.00000000`
- API Key：`api_keys.id=54`
- Key 脱敏：`sk-eadc1...19c799`
- Key 状态：`active`
- Key 绑定分组：`groups.id=2 / codex-pool-19-usd`
- 分组类型：`subscription`
- 分组平台：`openai`

## 请求前快照

订阅：

- `user_subscriptions.id=64`
- 状态：`active`
- 今日窗口：`2026-06-27 00:00:00+08`
- 今日订阅用量：`19.0282566500`
- 今日订阅额度：`19.00000000`
- 结论：今日订阅额度已超过套餐日限额。

流量卡：

- `user_traffic_credits.id=30`
- 原始额度：`10.0000000000`
- 请求前剩余：`9.8976433500`
- 请求前 deduction 合计：`0.1023566500`

用量记录：

- 请求前 `usage_logs`：`560` 条
- 请求前 `actual_cost` 合计：`26.5060580500`

## 公网真实请求

请求：

- Endpoint：`https://api.aaccx.pw/v1/responses`
- Model：`gpt-5.5`
- Prompt：要求回复固定短文本
- `max_output_tokens=16`
- Authorization 使用该用户 API Key，但未打印或记录完整 Key。

响应：

```json
{
  "http_code": 200,
  "id": "resp_06e8357f410eba0b016a3fd11085188191810245b3a65adbff",
  "model": "gpt-5.5",
  "status": "completed",
  "usage": {
    "input_tokens": 4692,
    "cached_tokens": 4352,
    "output_tokens": 10,
    "total_tokens": 4702
  }
}
```

## 请求后快照

流量卡：

- 请求后剩余：`9.8934673500`
- 本次新增扣费：`0.0041760000`
- deduction 合计：`0.1065326500`
- 新增 ledger：
  - `traffic_credit_ledger.id=178`
  - `entry_type=deduction`
  - `amount_usd=0.0041760000`
  - `balance_after_usd=9.8934673500`
  - `request_id=client:d27e40ce-47d9-48d7-9337-339bda317f0e`

用量记录：

- 请求后 `usage_logs`：`561` 条
- 请求后 `actual_cost` 合计：`26.5102340500`
- 新增 usage log：
  - `usage_logs.id=25156`
  - `request_id=client:d27e40ce-47d9-48d7-9337-339bda317f0e`
  - `model=gpt-5.5`
  - `input_tokens=340`
  - `output_tokens=10`
  - `actual_cost=0.0041760000`
  - `billing_type=0`
  - `subscription_id=NULL`

订阅：

- 请求后 `daily_usage_usd` 仍为 `19.0282566500`
- 说明：本次请求没有继续增加订阅额度用量，而是走流量卡扣费。

## 结论

`cnfoxian@gmail.com` 在今日订阅额度已耗尽后，使用其公网真实 API Key 发起模型请求成功返回 200，并真实从 10 USD OpenAI/GPT 流量卡中扣费 `0.0041760000` USD。流量卡兜底链路已在公网生效。
