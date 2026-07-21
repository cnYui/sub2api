# 计费错误运行态数据库快照封存结果

## 背景

当前 `sub2api-candidate` 运行态存在 OpenAI 用量计费与展示错误：用户即使继续使用额度，部分用量也可能无法在用户端或管理端正确计费和查看。该状态对后续审计、补录和修复验证很重要，因此先封存当前 PostgreSQL 与 Redis 快照。

## 快照对象

- PostgreSQL 容器：`sub2api-candidate-postgres`
- Redis 容器：`sub2api-candidate-redis`
- 应用容器：`sub2api-candidate`
- 备份时间批次：`20260721-162852`

## 本地封存文件

- 目录：`/Users/wujianxiang/CodeSpace/sub2api/deploy/backups/20260721-162852-sub2api-billing-error-runtime-snapshot`
- Zip：`/Users/wujianxiang/CodeSpace/sub2api/deploy/backups/20260721-162852-sub2api-billing-error-runtime-snapshot.zip`
- Zip 大小：`95M`
- Zip SHA-256：`c8dbee661a4343627860ba888ffe7ae98325dc335773a5655a356204572b7e0f`

Zip 内包含：

- `postgres-sub2api-candidate-20260721-162852.dump`
- `redis-sub2api-candidate-20260721-162852.rdb`
- `postgres-restore-list.txt`
- `redis-check-rdb.txt`
- `SHA256SUMS`

## 校验

- PostgreSQL dump 使用 `postgres:18-alpine` 内的 `pg_restore --list` 校验可读。
- Redis RDB 使用 `redis:8-alpine` 内的 `redis-check-rdb` 校验通过。
- Zip 使用 `unzip -tq` 校验通过。

## 处理边界

- 该备份只保存在本地，不提交到 GitHub。
- `deploy/backups/` 已被 `.gitignore` 排除，避免运行态数据进入远端仓库。
- 本次只做快照封存，不修改数据库、Redis、容器、Nginx 或公网路由。
