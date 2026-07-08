# chunpo.wu@uni-konstanz.de 99 元套餐公网库操作结果

## 结果

- 已在当前公网 `sub2api-candidate-postgres` 为 `chunpo.wu@uni-konstanz.de` 新增 99 元订阅。
- 新增订阅：`user_subscriptions.id=87`
- 用户：`users.id=72`
- 套餐分组：`groups.id=8` / `codex-pool-89-usd`
- 套餐商品：`subscription_plans.id=6` / `99 元订阅池`
- 状态：`active`
- 有效期：`2026-07-07 21:44:49.201821+08` 到 `2026-08-06 21:44:49.201821+08`
- 当前用量：daily/weekly/monthly 均为 `0`
- 用户现有 active 自动分组 Key 保留：`api_keys.id=95`，`group_id=NULL`

## 执行方式

- 写库前备份：`deploy/backups/20260707-224343-sub2api-candidate-before-chunpo-99-subscription.dump`
- 备份大小约 `35M`，权限 `600`
- 事务内校验用户、套餐、分组和无重复未软删除订阅后插入。
- 未创建或伪造 `payment_orders`，因为本次是人工分配套餐，不是外部支付回调。
- 未修改 nginx、容器、镜像、Postgres/Redis 拓扑。

## 验收

- `user_subscriptions` 复核：`id=87`、`status=active`、`expires_at - starts_at = 30 days`
- 同用户同分组未软删除订阅数：`1`
- active 自动 Key 数：`1`
- Redis `billing:sub:72:8` 缓存不存在，未发现旧缓存干扰
- `http://127.0.0.1:18084/health` 返回 `200`
- `http://127.0.0.1:8080/health` 返回 `200`
- `https://api.aaccx.pw/health` 返回 `200`
- `https://aaccx.pw/dashboard` 返回 `200`

## 注意

- 未做真实模型请求，原因是避免消耗用户额度，也避免在命令或文档中处理完整 API Key。
- 后续该用户的自动 Key 请求 OpenAI 路由时，应由 effective group 解析到 `codex-pool-89-usd`。
