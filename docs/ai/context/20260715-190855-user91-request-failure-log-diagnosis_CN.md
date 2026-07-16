# 1772475120@qq.com 请求失败日志诊断

## 结论

- 目标用户为 `users.id=91 / 1772475120@qq.com`，当前有效入口 Key 为 `api_keys.id=151`，账号状态 `active`，用户余额 `0.71000000`，并发 `5`。
- 用户已有有效订阅 `user_subscriptions.id=118 / group_id=2 / codex-pool-19-usd`，有效期为北京时间 `2026-07-15 16:26:29+08` 到 `2026-08-14 16:26:29+08`，当前日用量约 `0.0748864 USD`；没有流量卡。
- 失败主因不是余额、套餐、Key 失效或站点不可用，而是客户端请求配置和模型名问题：
  - 北京时间 `17:07:51-17:10:04`，同一用户/Key 大量请求 `POST /v1/responses/responses`，路径重复了一层 `responses`，Sub2API 转发后上游返回 `404`，入口对用户表现为 `502`。应用日志中该类 `openai.forward_failed` 约 `108` 条。
  - 北京时间 `17:49:23`、`17:49:38`、`17:57:53`、`17:58:11`、`17:58:17`、`17:58:20`，用户请求 `gpt-3.5-turbo`，上游返回 `502` 后当前组只有 `accounts.id=1` 可尝试，排除该账号后 `account_select_failed: no available accounts`，入口返回 `502`。
  - 北京时间 `18:01:13` 与 `18:04:25`，用户请求 `gpt-5.2`，同样因上游 `502` 后无其他可用账号，入口返回 `502`。
  - 北京时间 `18:04:21`，用户请求 `gpt-5.3-codex`，上游 `502` 后无其他可用账号，入口返回 `502`。
  - 北京时间 `18:04:24`，用户请求 `gpt-5.3-codex-spark`，上游明确返回 `400 The 'gpt-5.3-codex-spark' model is not supported when using Codex with a ChatGPT account.`，入口返回 `400`。
- 同一 IP `112.224.224.50` 还有 `/v1/usage` 的 `401`（北京时间 `16:32:13`、`17:10:20`、`17:30:03`）和 `/api/status` 的 `404`；这些失败发生在鉴权未解析出用户前，只能按同 IP 关联，不能严格证明就是 `users.id=91` 的已认证 API 请求。结合该用户当天从同 IP 登录、创建 Key 和发起模型请求，较可能是用户调试时使用了旧 Key、缺少认证或错误健康检查路径。

## 证据

- 数据库用户状态：
  - `users.id=91`，邮箱 `1772475120@qq.com`，`status=active`，`balance=0.71000000`，`total_recharged=30.00000000`。
  - 历史 Key：`api_keys.id=122` 和 `150` 已 `deleted_at`；当前有效 Key 为 `api_keys.id=151`，`last_used_at=2026-07-15 18:03:09+08`。
  - 订阅订单 `payment_orders.id=225` 为余额支付套餐订单，`COMPLETED`，金额 `29.00`，`plan_id=1`，`subscription_group_id=2`。
- 成功请求：
  - `usage_logs` 中该用户共有 `22` 条成功落库记录，全部发生在北京时间 `2026-07-15 17:11:04` 到 `18:04:31`，24 小时内账面费用约 `0.07793965 USD`。
  - 北京时间 `18:01-18:04` 间，`curl/7.81.0` 从 `112.224.224.50` 对 `/v1/chat/completions` 和 `/v1/responses` 的多条 `gpt-5.6-sol / gpt-5.5 / gpt-5.6-terra / gpt-5.6-luna / gpt-5.4 / gpt-5.4-mini / codex-auto-review` 请求均返回 `200` 并落库。
- 失败请求：
  - `openai.forward_failed`：`path=/v1/responses/responses`，`user_id=91`，`api_key_id=151`，`error="upstream error: 404"`，入口完成日志为 `status_code=502`。
  - `openai_gateway_handler.go` / `openai_chat_completions.go`：`model=gpt-3.5-turbo`、`gpt-5.2`、`gpt-5.3-codex` 出现 `upstream_status=502` 后 `account_select_failed`，错误为 `no available accounts`。
  - `openai_chat_completions.forward_failed`：`model=gpt-5.3-codex-spark`，错误为上游明确不支持该模型。

## 建议

- 客户端 Base URL 只保留 `https://api.aaccx.pw/v1`，不要配置成 `https://api.aaccx.pw/v1/responses`；否则 Responses SDK/客户端再拼 `/responses` 会变成 `/v1/responses/responses`。
- 不要用 `gpt-3.5-turbo`、`gpt-5.2`、`gpt-5.3-codex`、`gpt-5.3-codex-spark` 这类当前上游/账号不支持的模型名做正式请求；近期成功的模型包括 `gpt-5.6-sol`、`gpt-5.6-luna`、`gpt-5.6-terra`、`gpt-5.5`、`gpt-5.4`、`gpt-5.4-mini`、`codex-auto-review`。
- `/v1/usage` 返回 `401` 时先确认使用的是后台当前未删除的 Key，并使用 `Authorization: Bearer <key>`。

## 本轮影响

- 本轮只读查询 PostgreSQL、Docker 应用日志、Nginx access/error 日志。
- 未修改数据库、Redis、容器、Nginx、Cloudflare、业务代码或用户/Key/订阅状态。
