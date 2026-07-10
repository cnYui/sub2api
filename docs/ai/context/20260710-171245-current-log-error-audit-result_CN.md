# 当前日志报错排查结果

- 时间：2026-07-10 17:12 JST / 应用日志时间 2026-07-10 16:12 +08 左右。
- 操作范围：只读查看 Docker 日志、nginx 日志、运行态 Postgres、CLIProxyAPI 本地上游探针；未改 DB、Redis、nginx、容器或 CLIProxyAPI 配置。

## 结论

- 当前不是 Sub2API 整站不可用。`sub2api-candidate`、Postgres、Redis 均 healthy，最近两小时 `/api/*` 和大量 `/v1/responses`、`/v1/chat/completions` 仍正常 200。
- 当前最主要的用户报错来源是 `gpt-5.6-luna`：最近两小时约 221 次 `/v1/responses gpt-5.6-luna` 返回 503，错误为 `no available accounts`；最近 6 小时 `gpt-5.6-luna` 有 319 次 503、4 次 502。
- 根因不是用户 Key、订阅或额度，而是模型可见性与真实上游能力不一致：CLIProxyAPI `/v1/models` 当前列出 `gpt-5.6-luna`，但直接调用 CLIProxyAPI `/v1/responses` 时：
  - `gpt-5.6-sol` 返回 200 completed。
  - `gpt-5.6-terra` 返回 200 completed。
  - `gpt-5.6-luna` 返回 404 `Model not found gpt-5.6-luna`。
- Sub2API 唯一上游账号 `accounts.id=1/cliproxy-local-openai` 因上游 404 给 `gpt-5.6-luna` 写入模型级冷却：`reason=upstream_404_model_not_found`，所以之后请求 `luna` 会在账号选择阶段直接 `no available accounts`。

## liyutong2883@gmail.com

- 用户 `users.id=12/status=active`。
- Active 订阅 `user_subscriptions.id=7/group_id=2/codex-pool-19-usd`，有效期到 `2026-07-17 00:00:00+08`。
- 今日窗口 `daily_window_start=2026-07-10 00:00:00+08`，`daily_usage_usd=6.2934636500/19 USD`，没有超日额度。
- API Key：
  - `api_keys.id=3`，旧 Key，最近成功使用到 `2026-07-10 11:30:36+08`。
  - `api_keys.id=117/name=5.6/status=active`，最近使用 `2026-07-10 15:05:32+08`。
- 失败模式：`2026-07-10 14:31:11+08` 到 `15:05:53+08`，`user_id=12/api_key_id=117/group_id=2` 连续请求 `/v1/responses` + `gpt-5.6-luna`，容器日志统计 181 次 `openai.account_select_failed`，错误均为 `no available accounts`。
- 同一用户今天较早使用 `gpt-5.6-sol` 有成功落库记录，例如 `usage_logs.id=80162`、`2026-07-10 13:27:29+08`、`total_cost=0.1594760000`。

## 其他并发问题

- `gpt-5.2` 最近两小时约 30 次 `/v1/responses` 502，错误也是 `no available accounts`。这是旧/不支持模型被客户端继续请求，不是全站故障。
- 少量 `DAILY_LIMIT_EXCEEDED`：最近两小时 `gpt-5.5` 约 12 次、`gpt-5.6-sol` 约 1 次，属于用户订阅日额度真实超限。
- 旧客户端或错误 Base URL 仍在打：
  - `/models?client_version=0.142.5` 返回 400，说明仍有人使用裸 `/models` 而不是 `/v1/models`。
  - `/v1/v1/usage` 返回 404，说明有人把 base_url 配成带 `/v1` 后又重复拼了 `/v1/usage`。
- nginx error.log 中 `12:52-13:04 JST` 有一段 `connect() failed (61: Connection refused)`，属于当时应用重启/替换窗口导致源站短暂不可达；当前容器已 healthy，最近日志不是这个问题。

## 建议

- 立即对用户说明：现在不要选 `gpt-5.6-luna`，改用 `gpt-5.6-sol` 或 `gpt-5.6-terra`。
- 运行态应尽快下架或隐藏 `gpt-5.6-luna`，直到 CLIProxyAPI 真实 `/v1/responses` 调用能 200；否则 `/v1/models` 继续展示会让用户持续误选。
- 后续根治应让模型列表来源做可用性校验：不能只因为 `/v1/models` 列出某模型就展示，还要避免展示当前上游实际 404 的模型。
