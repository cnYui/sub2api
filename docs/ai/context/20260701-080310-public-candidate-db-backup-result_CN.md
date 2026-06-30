# 2026-07-01 公网候选数据库备份结果

## 目标

- 备份当前公网链路 `18084` 使用的数据层，供后续替换 `sub2api-candidate` 应用容器前回滚使用。
- 只做备份，不停止、不删除、不重建 `sub2api-candidate`、PostgreSQL、Redis 或 nginx。

## 备份对象

- PostgreSQL：`sub2api-candidate-postgres`，数据库 `sub2api`
- Redis：`sub2api-candidate-redis`

## 输出文件

- PostgreSQL dump：`deploy/backups/20260701-080310-sub2api-candidate-postgres-before-app-promote.dump`
  - 大小：`20478484` bytes
  - 权限：`600`
  - 格式：PostgreSQL custom format
- Redis RDB：`deploy/backups/20260701-080310-sub2api-candidate-redis-before-app-promote.rdb`
  - 大小：`52145` bytes
  - 权限：`600`

## 验证结果

- PostgreSQL：
  - `pg_dump -Fc` 成功退出。
  - `pg_restore -l` 可读取 dump，目录列表 `941` 行。
  - 候选库 `schema_migrations` 数量：`191`
  - 最新迁移：`155_seed_codex_subscription_plans_baseline.sql`
- Redis：
  - `redis-cli SAVE` 返回 `OK`。
  - `redis-check-rdb` 返回 `RDB looks OK`。
  - RDB 校验读取 `346` keys，`274` expires；`already expired` 受校验时刻影响，不作为备份完整性判断依据。
- 备份后公网入口：
  - `18084/health`：`{"status":"ok"}`
  - `8080/health`：`{"status":"ok"}`

## 容器状态

- `sub2api-candidate`：仍为 `sub2api-candidate:20260627-221441-traffic-card-fix`，healthy。
- `sub2api-candidate-postgres`：仍为 `postgres:18-alpine`，healthy。
- `sub2api-candidate-redis`：仍为 `redis:8-alpine`。

## 备注

- Redis 备份时 redis-cli 输出过默认用户无密码的 AUTH 提示，但同一命令随后返回 `OK`，RDB 已通过 `redis-check-rdb` 校验。
- 本轮没有打印 dump/RDB 内容，没有打印 env、密码、API Key、JWT secret、SMTP 密码或内部 token。
- `deploy/backups/` 下备份文件只用于回滚，不提交。
