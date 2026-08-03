# 本地分支审计与 18082 重部署

## 分支审计

- 当前分支：`main`。
- 本地仅有额外分支 `codex/official-upstream-migration`。
- `main...codex/official-upstream-migration` 的提交差异为 `7 0`：该分支没有任何未进入 `main` 的提交，且落后 `main` 7 个提交。
- 因此本次无需执行 Git 合并，也未改动或删除任何分支。

## 部署

- Compose 项目：`sub2api-official-18082`。
- 配置文件：`deploy/docker-compose.dev.yml`、`deploy/docker-compose.18082.yml`。
- 应用镜像：`sha256:aee45a707552545f1b80e0efb9b5115e1ec4a9be9505e339d9451537f639cbf2`。
- 仅强制重建 `sub2api` 容器；PostgreSQL 与 Redis 复用原有健康容器和持久化数据。

## 验证

- `sub2api-official-18082` 状态为 `healthy`。
- `GET /health` 与 `GET /purchase` 均返回 HTTP 200。
