# 候选环境数据库从 20260718 备份恢复结果

## 结论

已将候选环境 PostgreSQL/Redis 从 `deploy/backups/20260718-100232/` 恢复。

## 恢复来源

- PostgreSQL：`deploy/backups/20260718-100232/postgres.dump`
- Redis：`deploy/backups/20260718-100232/redis.rdb`

## 关键过程

- 先尝试使用 `20260720-180750` 部署前备份恢复，但恢复后 `users/accounts/api_keys/user_subscriptions` 均为 0，说明该备份不是期望的完整业务数据状态。
- 改用 `20260718-100232` 备份恢复。
- 为避免旧 schema 与备份 schema 互相污染，先清空 `public` schema，再恢复完整 dump。
- Redis 已用同组 `redis.rdb` 替换并重启。

## 验证结果

恢复后关键计数：

- `users`：116
- `accounts`：1
- `api_keys`：150
- `user_subscriptions`：95

关键账号：

- `xiaobianfuai@gmail.com` 存在，角色为 `admin`，状态为 `active`。

服务健康：

- `sub2api-candidate` healthy
- `sub2api-candidate-postgres` healthy
- `sub2api-candidate-redis` healthy
- `cliproxyapi-local-dev` healthy
- `http://127.0.0.1:18084/health` 返回 200
- `https://aaccx.pw/dashboard` 返回 200

## 备注

当前恢复结果匹配管理员邮箱预期。未推送代码，未修改 DNS。
