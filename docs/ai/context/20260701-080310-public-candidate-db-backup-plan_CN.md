# 2026-07-01 公网候选数据库备份计划

## 目标

- 备份当前公网链路使用的数据层，供后续替换 `sub2api-candidate` 应用容器前回滚使用。
- 公网当前链路仍为 `nginx 8080 -> sub2api-candidate 18084`。
- 只做备份，不停止、不删除、不重建应用、PostgreSQL、Redis 或 nginx。

## 范围

- PostgreSQL：`sub2api-candidate-postgres`，数据库 `sub2api`。
- Redis：`sub2api-candidate-redis`，导出当前 RDB 快照。

## 输出文件

- PostgreSQL dump：`deploy/backups/20260701-080310-sub2api-candidate-postgres-before-app-promote.dump`
- Redis RDB：`deploy/backups/20260701-080310-sub2api-candidate-redis-before-app-promote.rdb`

## 验证

- `pg_dump` 命令退出码为 0。
- PostgreSQL dump 文件存在、非空、权限为 `600`，并用 `pg_restore -l` 验证可读。
- Redis RDB 文件存在、非空、权限为 `600`。
- 备份后验证 `18084/health` 与 `8080/health` 仍返回 200。

## 敏感信息处理

- 不打印 dump/RDB 内容。
- 不打印 env、密码、API Key、JWT secret、SMTP 密码或内部 token。
- `deploy/backups/` 下备份文件不提交。
