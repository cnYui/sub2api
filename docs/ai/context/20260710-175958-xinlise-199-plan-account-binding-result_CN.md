# xinlise 199 元套餐 503 账号绑定修复结果

## 结论

`xinlise@gmail.com` 新买 199 元套餐后使用 503 的根因是运行态新套餐分组没有绑定上游账号，不是用户 Key、订阅、额度、退款状态或模型本身损坏。

## 证据

- 用户 `users.id=69/status=active`。
- 订单 `payment_orders.id=153` 在 `2026-07-10 16:43:25+08` 支付完成，`plan_id=9`，`subscription_group_id=12`，`amount=199.00`，`pay_amount=200.99`。
- 新订阅 `user_subscriptions.id=98/group_id=12/codex-pool-179-usd` 为 active，周期 `2026-07-10 16:43:25+08` 到 `2026-08-09 16:43:25+08`。
- 容器日志在 `2026-07-10 16:44-16:52+08` 显示：
  - `api_key_id=102/佳一老师`：`gpt-5.4` 82 次、`gpt-5.4-mini` 12 次、`gpt-5.5` 37 次，均为 `group_id=12`、`openai.account_select_failed: no available accounts`。
  - `api_key_id=99/codex`：`gpt-5.5` 85 次，均为 `group_id=12`、`openai.account_select_failed: no available accounts`。
- 写入前 `account_groups` 中 `group_id=11/codex-pool-135-usd` 与 `group_id=12/codex-pool-179-usd` 均无账号绑定。

## 写入内容

写库前备份：

- `deploy/backups/20260710-175341-sub2api-candidate-before-149-199-account-binding.dump`
- 已用容器内 `pg_restore -l /tmp/20260710-175341-sub2api-candidate-before-149-199-account-binding.dump` 验证可读。

已幂等插入：

- `account_groups(account_id=1, group_id=11, priority=50)`
- `account_groups(account_id=1, group_id=12, priority=50)`

其中 `account_id=1` 为 `cliproxy-local-openai`，状态 `active`，`schedulable=true`。

## 验证

- `group_id=11/codex-pool-135-usd` 已绑定 `account_id=1/cliproxy-local-openai`，`priority=50`。
- `group_id=12/codex-pool-179-usd` 已绑定 `account_id=1/cliproxy-local-openai`，`priority=50`。
- `xinlise@gmail.com` 当前 active 订阅仍为 `subscription_id=98/group_id=12/codex-pool-179-usd`。
- `curl http://127.0.0.1:18084/health` 返回 HTTP 200，body 为 `{"status":"ok"}`。
- 使用该用户 `api_key_id=102` 做极小 `/v1/responses` 非流式验证请求：
  - HTTP 200
  - 返回 `model=gpt-5.5`
  - 无 error
  - 新增 `usage_logs.id=82131`，`account_id=1`、`group_id=12`、`subscription_id=98`、`total_cost=0.0039810000`
  - 新订阅 `daily_usage_usd` 更新为 `0.0039810000`

## 后续注意

- 149/199 元套餐发布时必须同步绑定上游账号，否则购买成功后会在账号选择阶段 503。
- 本次未修改用户订单、退款状态、订阅周期、额度、API Key、上游账号凭据、Redis、nginx 或容器。
