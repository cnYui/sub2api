# 2466439791@qq.com API Key 公网联通性测试结果

## 结论

`2466439791@qq.com` 已生成 API Key，当前 active Key 可通过公网入口 `https://api.aaccx.pw/v1` 连通模型服务。

本次未修改用户、API Key、订阅、分组、上游账号或计费配置。

## 用户与 Key 状态

- 用户：`users.id=38`
- 邮箱：`2466439791@qq.com`
- 用户状态：`active`
- API Key：`api_keys.id=43`
- Key 名称：`Codex-used`
- Key 掩码：`sk-263e6...ec751d`
- Key 状态：`active`
- 绑定分组：`groups.id=2` / `codex-pool-19-usd`
- 订阅：`user_subscriptions.id=56`，状态 `active`
- 订阅到期：`2026-07-22 15:38:30.544797+08`
- 当前订阅日用量：`0.0000000000 / 19.00000000 USD`（请求前）

## 公网验证

`GET https://api.aaccx.pw/v1/models`

- HTTP：`200`
- 模型数量：`10`
- 样例模型：`gpt-5.5`、`gpt-5.4`、`gpt-5.4-mini`、`gpt-5.3-codex`、`gpt-5.3-codex-spark`

`POST https://api.aaccx.pw/v1/chat/completions`

- 模型：`gpt-5.4-mini`
- HTTP：`200`
- 响应内容：`pong`
- 响应头请求 ID：`c48efb00-39a3-4ae8-a5d0-a87b06223fb1`

## 记账落库验证

`usage_logs` 已写入该用户该 Key 的请求记录：

- `usage_logs.id=8536`
- `request_id=client:5bc457ed-2d89-4f6f-9082-772bd1abd6c4`
- `user_id=38`
- `api_key_id=43`
- `account_id=1`
- `group_id=2`
- `subscription_id=56`
- `model=gpt-5.4-mini`
- `requested_model=gpt-5.4-mini`
- `input_tokens=315`
- `output_tokens=5`
- `actual_cost=0.0002587500`
- `duration_ms=1495`
- `created_at=2026-06-22 17:17:30.375627+08`
