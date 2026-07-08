# xinlise@gmail.com API Key 真实模型请求测试结果

## 结论

- `xinlise@gmail.com` 的最近使用 active Key 可正常发起公网模型请求。
- 本次使用 `api_keys.id=99`，名称 `codex`，脱敏 `sk-f1acac8...9374`。
- 请求 `https://api.aaccx.pw/v1/responses`，模型 `gpt-5.5`，HTTP 返回 `200`，响应状态 `completed`，无 error。
- 新增用量日志 `usage_logs.id=67361`，扣费归属 `subscription_id=88`、`group_id=8/codex-pool-89-usd`，未扣流量卡。

## 测试前状态

- 用户：`users.id=69`，`email=xinlise@gmail.com`，状态 `active`。
- active Key：
  - `api_keys.id=99`，名称 `codex`，脱敏 `sk-f1acac8...9374`，测试前最近使用时间为 `2026-07-08 19:04:20+08`。
  - `api_keys.id=102`，名称 `佳一老师`，脱敏 `sk-7c22887...9178`，尚无最近使用时间。
- active 订阅：`user_subscriptions.id=88`，`group_id=8/codex-pool-89-usd`，有效期到 `2026-08-07 03:57:35+08`。
- 分组日限额 `89 USD`；测试前订阅今日用量为 `61.5989890000 USD`。
- OpenAI 流量卡可用余额为 `0 USD`。
- 请求前最新 `usage_logs.id=67357`。

## 请求结果

- 请求方式：非流式 `POST /v1/responses`。
- 请求模型：`gpt-5.5`。
- 响应 ID：`resp_09e1faa6cdff6989016a4e2f23e8648191ab1465293517a69c`。
- HTTP 状态码：`200`。
- 响应状态：`completed`。
- 响应 usage：
  - `input_tokens=4686`
  - `cached_tokens=4352`
  - `output_tokens=5`
  - `total_tokens=4691`

## 落库核对

- 本次 curl 对应 `usage_logs.id=67361`：
  - `user_id=69`
  - `api_key_id=99`
  - `model=gpt-5.5`
  - `requested_model=gpt-5.5`
  - `stream=false`
  - `input_tokens=334`
  - `cache_read_tokens=4352`
  - `output_tokens=5`
  - `total_cost=0.0039960000`
  - `group_id=8`
  - `subscription_id=88`
  - `billing_type=1`
  - `inbound_endpoint=/v1/responses`
  - `user_agent=curl/8.7.1`
  - `created_at=2026-07-08 19:06:12.940578+08`
- 请求后 `api_keys.id=99` 的 `last_used_at` 更新为 `2026-07-08 19:06:18.50948+08`。
- 请求后订阅 `user_subscriptions.id=88` 今日用量为 `61.7319380000 USD`。其中包含同一时间附近用户自己的 Codex Desktop 流式请求，不能全部归因于本次 curl。

## 同时发生的请求说明

- 请求后发现同用户同 Key 还新增 `usage_logs.id=67362`：
  - `stream=true`
  - `total_cost=0.1289530000`
  - `user_agent=Codex Desktop/0.142.5 ...`
  - `created_at=2026-07-08 19:06:16.310824+08`
- 该日志不是本次测试请求，而是同一时间用户自己的 Codex Desktop 流式请求。

## 运行态影响

- 本轮没有手工修改数据库业务数据。
- 本轮没有构建镜像、替换容器、重启服务、修改 nginx、修改 Redis。
- 真实模型请求自然新增了用量日志，并推进了订阅用量。
- `http://127.0.0.1:18084/health` 返回 `200`。
- `https://api.aaccx.pw/health` 返回 `200`。
