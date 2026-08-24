# main 同步与公网容器发布

## Git

- 本地 `main` 已提交 `f153d0f1a`：`refactor: 精简商品退款边界并移除旧余额逻辑`。
- 已推送到私有 GitHub `fork/main`，本地 `main`、`fork/main` 和 `fork/HEAD` 当前一致。
- 工作区在部署验证后保持干净；本次发布使用的是该提交源码。

## 验证

- `go test ./...` 通过。
- `pnpm typecheck` 通过。
- 前端 Vitest 全量为 200 个测试文件、1392 个测试全部通过。
- Docker 构建 `deploy-sub2api:latest` 成功，镜像摘要：`sha256:6bcbeea8b70b742b3cec742c8aaf9b8c6d3227f9f33fd8b653d89746c6637a98`。

## 发布

- 使用 `docker-compose.dev.yml` 与 `docker-compose.18082.yml` 组合配置，仅执行 `sub2api` 服务 `--no-deps --force-recreate`。
- 公网应用容器 `sub2api-official-18082` 已替换并为 `healthy`。
- PostgreSQL、Redis 未重建，原数据卷和账号凭证密钥挂载保持不变，两个依赖容器均为 `healthy`。
- `http://127.0.0.1:18082/health`、`http://127.0.0.1:8080/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200；`https://aaccx.pw/usage-guide` 返回 HTTP 200。

本次没有重建数据库、Redis、Nginx 或 Cloudflare Tunnel。
