# xinlise 199 元套餐 503 与新套餐上游绑定修复计划

## 背景

- 用户 `xinlise@gmail.com` 已在 `2026-07-10 16:43:25+08` 支付完成 `199 元订阅池`，订单 `payment_orders.id=153`。
- 新订阅为 `user_subscriptions.id=98/group_id=12/codex-pool-179-usd`，状态 `active`，每日额度 `179 USD`，当前用量 `0`。
- 应用日志显示该用户两把 Key（`api_key_id=99/codex`、`api_key_id=102/佳一老师`）请求 `/v1/responses` 时进入 `group_id=12`，模型包含 `gpt-5.4`、`gpt-5.4-mini`、`gpt-5.5`，统一在账号选择阶段返回 `openai.account_select_failed: no available accounts`，HTTP 503。
- 运行态 `account_groups` 中 `group_id=11/codex-pool-135-usd` 与 `group_id=12/codex-pool-179-usd` 均未绑定任何账号；这是 149/199 套餐发布后的运行态绑定遗漏。

## 必须修复的问题

- 将现有可用上游账号 `accounts.id=1/cliproxy-local-openai` 绑定到两个新订阅池：
  - `group_id=11/codex-pool-135-usd`
  - `group_id=12/codex-pool-179-usd`
- 绑定优先级沿用已有 69/89 套餐绑定的 `priority=50`。

## 执行步骤

1. 创建 Postgres 运行态备份到 `deploy/backups/`。
2. 用 `pg_restore -l` 验证备份可读。
3. 幂等插入：
   - `(account_id=1, group_id=11, priority=50)`
   - `(account_id=1, group_id=12, priority=50)`
4. 复核两个分组均有 active 上游账号绑定。
5. 复核 `xinlise@gmail.com` 当前 active 订阅仍指向 `group_id=12`，API Key 自动分组保持 `NULL`。
6. 复核 `18084/health` 为 200。

## 不做的事

- 不修改用户订单、退款状态、订阅周期、额度或用量。
- 不改 `accounts.id=1` 的凭据、状态、模型映射和冷却状态。
- 不重启容器；账号绑定来自数据库，当前问题不需要重启。
