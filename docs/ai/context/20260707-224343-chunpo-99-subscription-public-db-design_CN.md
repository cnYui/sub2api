# chunpo.wu@uni-konstanz.de 99 元套餐公网库操作设计

## 背景

- 用户请求：将 `chunpo.wu@uni-konstanz.de` 添加为 99 元套餐档位，需要直接修改当前公网运行数据库。
- 当前公网运行栈只读确认：`sub2api-candidate` 应用容器运行在 `127.0.0.1:18084->8080`，数据库为 `sub2api-candidate-postgres`。
- 目标用户只读确认：`users.id=72`，邮箱 `chunpo.wu@uni-konstanz.de`，状态 `active`，未软删除。
- 目标套餐只读确认：`subscription_plans.id=6`，`name='99 元订阅池'`，`price=99.00`，`validity_days=30`，对应 `groups.id=8` / `codex-pool-89-usd`，每日限额 `89 USD`。
- 用户当前无 `user_subscriptions` 记录；已有 active 自动分组 API Key `api_keys.id=95`，`group_id=NULL`。
- `codex-pool-89-usd` 已绑定上游账号 `cliproxy-local-openai`。

## 方案

采用最小数据变更：只在 `user_subscriptions` 新增一条 active 订阅。

- `user_id=72`
- `group_id=8`
- `status='active'`
- `starts_at=NOW()`
- `expires_at=NOW() + INTERVAL '30 days'`
- `assigned_by=NULL`
- `notes='手动分配：chunpo.wu@uni-konstanz.de 99 元订阅池，2026-07-07'`
- usage 字段保持 `0`
- 窗口字段保持 `NULL`，与现有服务 `AssignSubscription` 新建订阅行为一致，首个计费请求会按当前代码初始化窗口

## 不做事项

- 不伪造 `payment_orders` 或支付流水，避免把一次人工分配伪装成外部支付。
- 不修改用户现有 API Key；自动分组 Key 会在请求时解析到有效套餐。
- 不改 nginx、容器、镜像、Postgres/Redis 拓扑。

## 风险与回滚

- 写库前先备份 `sub2api-candidate-postgres` 到 `deploy/backups/`，权限 `600`。
- 写入使用单事务，先锁定目标用户，再校验用户、分组、套餐和同分组未软删除订阅不存在。
- 如需回滚，只需软删除新增的 `user_subscriptions` 记录，或从备份恢复数据库。

## 验收

- 查询用户存在一条未软删除 active `codex-pool-89-usd` 订阅。
- `expires_at` 为写入时间后 30 天。
- 用户 active 自动分组 Key 仍存在。
- 删除 `billing:sub:72:8` 缓存，避免旧空结果干扰。
- `18084/health`、`8080/health`、`api.aaccx.pw/health` 均返回 200。
