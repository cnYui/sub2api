# 3876129758@qq.com API Key 真实模型请求测试结果

## 结论

- `3876129758@qq.com` 的最近使用 active Key 可正常访问模型。
- 本次使用 `api_keys.id=106`，名称 `Codex++`，脱敏 `sk-6abb393...0cbd`。
- 请求 `https://api.aaccx.pw/v1/responses`，模型 `gpt-5.5`，HTTP 返回 `200`，响应状态 `completed`，无 error。
- 新增用量日志 `usage_logs.id=67410`，扣费归属 `subscription_id=90`、`group_id=4/codex-pool-49-usd`，未扣流量卡。

## 测试前状态

- 用户：`users.id=56`，`email=3876129758@qq.com`，状态 `active`。
- active Key：
  - `api_keys.id=105`，名称 `Codex`，脱敏 `sk-bba5b55...5164`，最近使用时间 `2026-07-08 17:46:42+08`。
  - `api_keys.id=106`，名称 `Codex++`，脱敏 `sk-6abb393...0cbd`，最近使用时间 `2026-07-08 19:04:51+08`。
- 当前未删除 active 订阅：`user_subscriptions.id=90`，`group_id=4/codex-pool-49-usd`，有效期到 `2026-08-07 15:53:48+08`。
- 旧订阅 `user_subscriptions.id=76` 已为 `expired + deleted_at=2026-07-08 16:58:50.722897+08`，未参与本次扣费。
- 分组日限额 `49 USD`；请求前订阅今日用量为 `0.2819010000 USD`。
- OpenAI 流量卡可用余额为 `0 USD`。
- 请求前最新 `usage_logs.id=67399`。

## 请求结果

- 请求方式：非流式 `POST /v1/responses`。
- 请求模型：`gpt-5.5`。
- 响应 ID：`resp_02a3415a53bbd5c9016a4e302abcc4819baddad44d13a10f05`。
- HTTP 状态码：`200`。
- 响应状态：`completed`。
- 响应 usage：
  - `input_tokens=4686`
  - `cached_tokens=4352`
  - `output_tokens=5`
  - `total_tokens=4691`

## 落库核对

- 本次 curl 对应 `usage_logs.id=67410`：
  - `user_id=56`
  - `api_key_id=106`
  - `model=gpt-5.5`
  - `requested_model=gpt-5.5`
  - `stream=false`
  - `input_tokens=334`
  - `cache_read_tokens=4352`
  - `output_tokens=5`
  - `total_cost=0.0039960000`
  - `group_id=4`
  - `subscription_id=90`
  - `billing_type=1`
  - `inbound_endpoint=/v1/responses`
  - `user_agent=curl/8.7.1`
  - `created_at=2026-07-08 19:10:35.677064+08`
- 请求后 `api_keys.id=106` 的 `last_used_at` 更新为 `2026-07-08 19:10:33.793348+08`。
- 请求后订阅 `user_subscriptions.id=90` 今日用量为 `0.2858970000 USD`，与本次 `total_cost=0.0039960000` 增量一致。

## 运行态影响

- 本轮没有手工修改数据库业务数据。
- 本轮没有构建镜像、替换容器、重启服务、修改 nginx、修改 Redis。
- 真实模型请求自然新增了用量日志，并推进了订阅用量。
- `http://127.0.0.1:18084/health` 返回 `200`。
- `https://api.aaccx.pw/health` 返回 `200`。
