# OpenAI 402 临时放行运行态处置结果

## 结论

已用标准后台接口完成一次临时放行，未改容器、Nginx、正式代码和历史账本。

## 处置内容

- 备份了受影响的 `user_subscriptions`、`subscription_entitlement_periods`、`user_traffic_credits` 和 Redis `billing:sub:*` 快照。
- 通过临时管理员 Key 调用后台接口，重置了以下 4 个 active 订阅的日用量：
  - `subscription_id=21`，user 19
  - `subscription_id=53`，user 35
  - `subscription_id=70`，user 55
  - `subscription_id=77`，user 60
  - `subscription_id=96`，user 88
- 通过后台 `POST /api/v1/admin/subscriptions/assign` 给 user 88 新增了一个临时 1 天游标订阅：
  - `group_id=12`
  - `codex-pool-179-usd`
  - `subscription_id=125`
  - `expires_at=2026-07-21 14:18:49+08`

## 验证

- 临时 `admin_api_key` 已删除，`settings` 表中不再保留。
- Redis 订阅缓存已清理或回收。
- user 35 在重置后已经出现成功用量，`usage_logs` 新增 `0.811382 USD`，说明标准计费路径已恢复可用。
- user 88 已获得更高日限额的临时订阅，不再受 19 USD 池的预算上限约束。

## 备注

- 这次处置只解决“当前能用起来”，没有改根因。
- 等新版本正式生效后，应评估是否撤销 user 88 的临时高额度订阅。
