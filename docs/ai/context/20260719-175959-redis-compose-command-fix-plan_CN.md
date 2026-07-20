# Redis Compose 启动命令修复计划

## 根因

现有四份 Docker Compose 将 Redis 启动命令写成多行 shell：

```text
redis-server
--save 60 1
--appendonly yes
```

容器实际执行时，第一行 `redis-server` 已进入前台运行，后续参数被当作永远不会执行的独立命令。因此持久化参数和 `--requirepass` 均未生效，但 `REDISCLI_AUTH` 仍被设置，健康检查会产生 AUTH 错误。

## 修改范围

- 新增 `deploy/verify-redis-compose-command.mjs`，使用 `docker compose config --format json` 验证实际渲染结果。
- 修改：
  - `deploy/docker-compose.yml`
  - `deploy/docker-compose.local.yml`
  - `deploy/docker-compose.dev.yml`
  - `deploy/docker-compose.candidate.yml`

## 方案

- Redis 容器显式接收 `REDIS_PASSWORD`。
- 使用单条 `sh -c` 脚本：`exec redis-server ...`。
- 使用 `$$` 让密码条件表达式保留到容器内执行，避免 Compose 在宿主机提前展开。
- 有密码时同时启用服务端 `requirepass` 和客户端 `REDISCLI_AUTH`；无密码时两者均为空。

## 验证

1. 修改前运行验证脚本，确认失败。
2. 修改四份 Compose 后再次运行，确认全部通过。
3. 重新创建本地 Redis 容器。
4. 验证 `CONFIG GET requirepass` 为非空、`PING=PONG`、`DBSIZE=0`，日志无 AUTH 错误。
