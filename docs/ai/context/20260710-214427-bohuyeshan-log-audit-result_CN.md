# bohuyeshan@163.com 日志问题只读排查结果

## 结论

- 用户 `bohuyeshan@163.com` 对应运行态 `users.id=65/status=active`，账号本身有效。
- 该用户只有一把 active API Key：`api_keys.id=84/name=oc/group_id=NULL`，最近一次成功使用在 `2026-07-10 16:07:46+08`。
- 当前 active 订阅为 `user_subscriptions.id=83/group_id=8/codex-pool-89-usd`，周期 `2026-07-07 13:41:44+08` 到 `2026-08-06 13:41:44+08`，未删除、未过期。
- 直接问题是今天日额度已满：`daily_usage_usd=89.1596930000/89 USD`，超出 `0.1596930000 USD`。
- `2026-07-10 16:07:47+08` 后续请求 `/v1/responses gpt-5.6-sol` 被本地计费准入拦截，容器日志为 `openai.billing_eligibility_check_failed`，错误 `DAILY_LIMIT_EXCEEDED daily usage limit exceeded`，HTTP `403`。
- `ops_error_logs` 中该用户无记录；额度拦截只出现在容器 stdout。
- Redis 中 `billing:sub:65:8` 不存在，DB 是当前事实来源。
- 本轮只读排查，未修改 DB、Redis、nginx、容器、代码或用户 Key。

## 今日用量

- `2026-07-10` 共 450 条成功 usage，全部 `billing_type=1` 且扣到 `subscription_id=83`，总费用 `89.1596930000 USD`。
- 按模型：
  - `gpt-5.6-sol`：335 次，`63.9893470000 USD`
  - `gpt-5.4`：46 次，`19.2823680000 USD`
  - `gpt-5.6-terra`：69 次，`5.8879780000 USD`
- 最后一条成功 usage：`2026-07-10 16:07:37+08`，`request_id=client:5d6ea14d-3182-44fc-8ea5-7b4cbc7a8663`，`gpt-5.6-sol`，费用 `0.6963390000 USD`，累计到 `89.1596930000 USD`。
- 下一条请求 `2026-07-10 16:07:47+08` 命中 `DAILY_LIMIT_EXCEEDED`，未新增 usage。
- 当前 DB 时间 `2026-07-10 20:43:42+08` 时，距下一次自然日窗口 `2026-07-11 00:00:00+08` 约 `03:16:17`。

## 今日日志中的其它异常

- `gpt-5.6-luna`：`2026-07-10 13:42:29+08` 到 `14:58:15+08`，`openai.account_select_failed/no available accounts` 9 次；同窗口有 `upstream_status=404`，符合当前全站已定位的 `gpt-5.6-luna` 上游模型不可用问题。
- `gpt-5.6-terra`：`2026-07-10 15:53:46+08` 一次 `openai.forward_failed`，上游返回 `This model is compatible only with 24h extended prompt caching`。
- `gpt-5.6-sol`：`2026-07-10 13:41:49+08` 一次 `stream usage incomplete: missing terminal event`。
- `gpt-5.4`：`2026-07-10 13:55:53+08` 一次同账号重试，`upstream_status=401`，后续仍有大量成功 usage。

## 订单状态

- 该用户只有一笔订阅订单：`payment_orders.id=134`，`99 元订阅池`，`status=COMPLETED`，`paid_at=2026-07-07 13:41:44+08`。
- 当前有 1 个 active 未过期订阅，因此购买新订阅会命中现有单 active 订阅保护；本轮未发现该用户有新的 pending/completed 订阅订单。

## 建议

- 对用户说明：当前不是 Key 失效，也不是订阅过期，而是今天 89 USD 日额度已用完；等 `2026-07-11 00:00:00+08` 日窗口刷新后可继续用。
- 如果用户仍在用 `gpt-5.6-luna`，让他改用 `gpt-5.6-sol` 或 `gpt-5.6-terra`；`luna` 当前上游真实调用不可用。
- 若用户希望今天继续使用，需要管理员侧明确业务动作：退款/撤销旧订阅后重买，或人工调整额度。执行任何写库动作前必须先备份 Postgres 和 Redis。
