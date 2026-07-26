# Docker Desktop 恢复与镜像清理结果

时间：2026-07-26 09:47:00

## 故障原因

`18080` 与 `18086` 不可用并非 Sub2API 进程自身崩溃。Docker Desktop 前端进程残留，但 `com.docker.backend` 不存在，`docker-desktop` WSL2 发行版为 `Stopped`，`dockerDesktopLinuxEngine` 命名管道缺失，Docker CLI 无法访问任何容器。

## 恢复过程

重启 Docker Desktop 后端并重新启动 Docker Desktop。恢复初期，两套 PostgreSQL 因此前中断执行自动恢复和检查点；应用容器在数据库尚未就绪时按 `restart: unless-stopped` 重试。数据库恢复完成后，两个应用自动恢复，无需修改应用代码或数据库数据。

最终状态：

- `sub2api-dev`：`healthy`，`http://127.0.0.1:18080/health` 返回 `{"status":"ok"}`。
- `sub2api-upstream-latest`：`healthy`，`http://127.0.0.1:18086/health` 返回 `{"status":"ok"}`。
- `sub2api-postgres-dev` 与 `sub2api-upstream-postgres`：均为 `healthy`。

## 镜像清理

执行 `docker image prune -a -f`，仅删除未被任意容器引用的旧镜像，释放 `3.89GB`。

当前剩余 22 个镜像均被运行或保留容器引用，未强制删除。两个已停止容器仍保留，避免在用户未要求删除容器的情况下丢失其配置：

- `cliproxyapi-local-dev-before-shared-network-20260719-195935`
- `supabase_edge_runtime_SW`

本轮未清理构建缓存、卷或容器。
