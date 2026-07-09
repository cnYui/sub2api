# Docker 镜像与构建缓存清理结果

时间：2026-07-09 09:32 JST

## 操作范围

- 删除停止态旧应用容器：
  - `sub2api-candidate-before-promote-20260709-083942`
  - `sub2api-candidate-before-window-refresh-20260707-084458`
- 执行 `docker builder prune -af` 清空 Build Cache。
- 执行 `docker image prune -af` 删除未被容器引用的旧镜像。
- 未清理 Docker volumes。
- 未停止、删除或重建当前运行容器。

## 清理结果

- 容器从 5 个变为 3 个，剩余容器均为运行态：
  - `sub2api-candidate`
  - `sub2api-candidate-redis`
  - `sub2api-candidate-postgres`
- 镜像从 20 个变为 3 个：
  - `sub2api-candidate:20260709-083942-f2cb7e705-new-subscription-plans`
  - `postgres:18-alpine`
  - `redis:8-alpine`
- Build Cache 从 `20.81GB` 变为 `0B`。
- Images 从 `24.23GB` 变为 `724.3MB`。
- `http://127.0.0.1:18084/health` 返回 `{"status":"ok"}`。

## Local Volumes 说明

Docker Local Volumes 是 Docker 管理的本机持久化目录，生命周期独立于 container 和 image；删除容器或镜像不会自动删除 volume。它常用于数据库、Redis 或应用数据持久化。

当前 `docker system df -v` 显示 12 个 local volumes，总大小约 `831MB`，其中只有匿名 volume `97d3544bd4fb3e047dfc96fd3e104b9a3def77435a546b220c8ef02ec23eecb7` 仍被 `sub2api-candidate-postgres` 链接，但大小为 `0B`。当前实际运行数据主要通过 bind mount 挂到宿主机目录：

- `sub2api-candidate`：`.worktrees/codex-sub2api-candidate-rehearsal-20260626/deploy/candidate/data -> /app/data`
- `sub2api-candidate-postgres`：`.worktrees/codex-sub2api-candidate-rehearsal-20260626/deploy/candidate/postgres_data -> /var/lib/postgresql/data/pgdata`
- `sub2api-candidate-redis`：`.worktrees/codex-sub2api-candidate-rehearsal-20260626/deploy/candidate/redis_data -> /data`

`deploy_postgres_data`、`sub2api-main-preview-pgdata`、`deploy_sub2api_data` 等 `LINKS=0` 的 volume 目前没有容器引用，可能是旧 18080/preview 链路残留数据。未确认备份和历史用途前，不建议直接执行 `docker volume prune`。
