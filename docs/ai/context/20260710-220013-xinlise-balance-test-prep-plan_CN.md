# xinlise 测试购买退款流程准备计划

## 目标

为 `xinlise@gmail.com` 准备一个可用于测试“余额购买套餐并退款”的干净账号状态：

- 确认当前是否仍有套餐。
- 若仍有未删除 active 套餐，按管理员撤销语义写库为 `expired + deleted_at`。
- 确认没有 `user_allowed_groups` 残留。
- 确认 API Key 没有固定套餐分组。
- 给用户余额增加一笔测试金额。
- 清理相关 Redis 计费缓存。

## 当前只读发现

- 用户 `users.id=69`，余额 `0.00000000`。
- `user_allowed_groups` 当前为空。
- API Key `99/codex` 与 `102/佳一老师` 均为 active 且 `group_id=NULL`。
- 订阅表仍有 `user_subscriptions.id=98/group_id=12/codex-pool-179-usd/status=active/deleted_at=NULL`，所以当前并不是“没有套餐”状态。
- 旧订阅 `id=88/group_id=8/codex-pool-89-usd` 已是 `expired + deleted_at`。

## 执行方案

- 写库前重新备份 Postgres 与 Redis。
- 在一个 SQL 事务内：
  - 条件更新 `user_id=69` 当前未删除、未过期、active 的订阅为 `status='expired'`，并写入 `deleted_at=NOW()`。
  - 给 `users.balance` 增加 `200.00`，用于测试余额购买。
  - 写一条 `redeem_codes.type='admin_balance'` 的 used 审计记录，备注本次为测试购买退款流程。
- 清 Redis：
  - `billing:balance:69`
  - `billing:sub:69:12`
- 复核用户余额、订阅、授权、API Key、缓存与健康检查。

## 风险控制

- 不修改退款订单状态。
- 不删除 API Key。
- 不修改上游账号、分组绑定、nginx、Cloudflare Tunnel。
- 如果执行时 active 订阅已被管理员页面撤销，条件更新会自然返回 0，不重复写删除时间。
