# xunskyler@gmail.com 套餐到期只读核验

## 结论

- 当前公网事实源为 `sub2api-candidate-postgres`，对应 18084 公网链路。
- `xunskyler@gmail.com` 对应 `users.id=19`，用户状态仍为 `active`，未软删除。
- 该用户只有 1 条订阅：`user_subscriptions.id=21/group_id=2/codex-pool-19-usd`，每日额度 19 USD。
- 套餐已于北京时间 `2026-07-13 10:01:08.657+08` 到期；系统在 `2026-07-13 10:01:19.815+08` 将状态更新为 `expired`。
- 查询时数据库时间为北京时间 `2026-07-14 19:11:46+08`，已过期约 1 天 9 小时；不存在新的订阅或续购订单。

## 补充判断

- 用户仍有 active API Key，不代表套餐仍有效。
- 套餐到期后的成功请求改走流量卡计费：相关 `usage_logs.subscription_id` 为空、`billing_type=0`。
- 两张 10 USD 流量卡中，一张已耗尽，另一张在本次快照仅剩 `0.0011115500 USD`，虽然有效期到 `2027-07-03`，但额度已接近耗尽。
- 最后一条成功模型用量记录时间为北京时间 `2026-07-14 14:23:09+08`。

## 本次操作

- 只执行 Docker 状态检查、PostgreSQL schema 和数据查询、容器日志只读检索。
- 未修改数据库、Redis、容器、Nginx、公网链路、用户、套餐、API Key 或额度。
- 未输出完整 API Key、内部 token 或其他凭据。
