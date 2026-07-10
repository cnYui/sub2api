# xinlise 误当测试账号后的恢复结果

## 事件

用户要求给测试账号加余额并确认无套餐，但未重新确认邮箱；按前文上下文误把 `xinlise@gmail.com` 当作测试账号处理。

误操作事务最终成功写入了三项：

- `user_subscriptions.id=98/user_id=69/group_id=12` 从 `active + deleted_at=NULL` 改为 `expired + deleted_at=2026-07-10 21:02:27.059627+08`
- `users.id=69` 余额从 `0` 加到 `200`
- 插入 `redeem_codes.id=22/code=ADM-XIN-20260710210227/type=admin_balance/value=200/used_by=69`

执行前已有备份：

- Postgres：`deploy/backups/20260710-220013-sub2api-candidate-before-xinlise-balance-test-prep.dump`
- Redis：`deploy/backups/20260710-220013-sub2api-candidate-redis-before-xinlise-balance-test-prep.rdb`

## 恢复

用户指出账号搞错后，已立即用事务恢复：

- `user_subscriptions.id=98` 恢复为 `status='active'`、`deleted_at=NULL`
- `users.id=69` 余额减回 `0`
- 删除误插入的 `redeem_codes.id=22`
- 清理 Redis：
  - `billing:balance:69`
  - `billing:sub:69:12`
  - `billing:sub:69:8`

## 复核

- `xinlise@gmail.com` 当前 `users.id=69/status=active/balance=0/total_recharged=0`
- 当前只有一个未删除 active 套餐：
  - `user_subscriptions.id=98/group_id=12/codex-pool-179-usd/status=active/deleted_at=NULL/expires_at=2026-08-09 16:43:25+08`
- 旧订阅：
  - `id=88/group_id=8/status=expired/deleted_at=2026-07-10 16:27:50+08`
- API Key `99/codex` 与 `102/佳一老师` 均 active 且 `group_id=NULL`
- `account_groups` 已确认 `group_id=12` 绑定 `account_id=1/cliproxy-local-openai/status=active/schedulable=true`
- 误审计记录已不存在
- `curl /health` 返回 `{"status":"ok"}`
- 使用 `api_key_id=102` 调 `/v1/models` 返回 HTTP 200，模型数 13；该检查不产生 usage 扣费

## 后续要求

后续给“测试账号”加余额或撤套餐前，必须先让用户明确邮箱或用户 ID；不得再从上一段上下文推断账号。
