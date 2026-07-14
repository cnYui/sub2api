# 405045701@qq.com 最近请求诊断

## 范围

- 排查时间：2026-07-13 13:17 JST（北京时间 12:17）。
- 运行态：`sub2api-candidate`、PostgreSQL、Redis、CLIProxyAPI。
- 本轮只读检查账号、Key、订阅、用量、应用日志、nginx 日志和健康状态；未修改数据库、Redis、Key、订阅、容器或服务配置。

## 结论

该用户的账号、当前 Key、29 元套餐和上游均正常。最近没有成功模型请求，问题仍在客户端凭据配置，不是套餐未生效或额度不足。

- 用户为 `users.id=96`，状态 `active`。
- 订阅 `user_subscriptions.id=100/group_id=2/codex-pool-19-usd` 为 `active`，有效期至 2026-08-09 18:47:33 +08；今日额度窗口从 2026-07-13 00:00:00 +08 开始，当前用量 `0/19 USD`。
- Key `124/Codex-used` 已于 2026-07-10 删除；Key `125/Codex_used` 和 `126/codex 2.0` 均为 active。
- Key `126` 最近一次鉴权发生在 2026-07-13 11:49:18 +08，对应来源 `180.169.129.68` 的 `GET /v1/usage`，HTTP 200、耗时 9ms。该端点只查询用量，不产生模型 usage，说明当前 Key 能鉴权，但不能代表客户端已经成功发起模型请求。
- 该用户 `usage_logs` 仍为 0，说明截至排查时没有任何携带当前有效 Key 的成功模型请求到达 Sub2API。
- 分组 `group_id=2` 状态 active、日限额 19 USD；已绑定 `account_id=1/cliproxy-local-openai`，账号 active、schedulable、并发 100。Sub2API `/health` 与 CLIProxyAPI `/healthz` 均为 200。

## 异常请求

另一个来源 `8.219.150.152` 仍在持续提交错误凭据：

- 当前可见日志从 2026-07-12 09:37:36 +08 到 2026-07-13 08:35:24 +08 共 51 次 401。
- 其中 `/v1/responses` 27 次，响应体 54 字节，对应 `INVALID_API_KEY`。
- Gemini 入口 24 次，客户端把一个 CLIProxyAPI 风格的默认内部凭据放在 query 参数中请求 Sub2API，该凭据不是 Sub2API 用户 Key，因此稳定返回 401。
- 客户端标识为 Windows Chrome 138；该 IP 在当前日志中没有成功模型请求。

由于运行态 `ops_monitoring_enabled=false`，401 鉴权早退没有写入 `ops_error_logs`，无法严格证明 `8.219.150.152` 一定属于 `user_id=96`。但它与当前有效 Key 的最近来源 `180.169.129.68` 不同，而且提交的是非 Sub2API 凭据；若这是用户自己的远程代理或客户端，就是当前失败的直接原因。

## 处理建议

- 在实际发起模型请求的远程主机或客户端中，替换为后台当前 active Key 的完整值，优先使用 `codex 2.0`，不要使用已删除的 `Codex-used`、列表页掩码或 CLIProxyAPI 内部默认 Key。
- Base URL 使用 `https://api.aaccx.pw/v1`。
- OpenAI 请求使用 `Authorization: Bearer <完整 Sub2API Key>`；不要把 Key 放在 query 参数。
- 修改后重启或重新加载客户端。无需更换套餐、重置额度或修改服务端订阅。
