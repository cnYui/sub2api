# 18082 Codex 使用方法公网发布

## 发布范围

将本地 `main` 提交 `b0765e243c62048c6b7ee6dcfdf69c7318536784` 重新构建为 `deploy-sub2api:latest`，并替换公网链路的 `sub2api-official-18082` 应用容器。

本次提交包含更新后的 Codex 接入教程和 5 张步骤截图。

## 执行结果

- 新应用镜像：`sha256:988589cc03f5ec8b6e9d24d27bcb77fad86dd718b2c73ddf55c2b1a6f4d2d877`。
- 新应用容器：`e53c5dea4052c3b5da22bff2d51ac04181f3381313ab2f3a57c9d0bf2f63e223`，健康状态为 `healthy`。
- 使用 `docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --no-deps --force-recreate sub2api` 替换应用容器。
- PostgreSQL 和 Redis 的容器 ID、创建时间和启动时间未变化，均保持 `healthy`。
- 未重建 Nginx、Cloudflare Tunnel、数据库、Redis 或数据卷。

## 公网核验

- `docker exec sub2api-public-nginx-local nginx -t` 通过，Nginx upstream 为 `host.docker.internal:18082`。
- `http://127.0.0.1:18082/health`、`http://127.0.0.1:8080/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health` 和 `https://api.aaccx.pw/health` 均返回 HTTP 200。
- 公网 `/login` 及其入口脚本返回 HTTP 200。
- 公网 `UsageGuideView-BYiLTdh6.js` 返回 HTTP 200，包含新版 `https://api.aaccx.pw/v1` 配置文案和 Codex 第 4 步内容。
- 5 张 `codex-latest-step-*` 静态资源均经 `https://aaccx.pw/assets/` 返回 HTTP 200。

## 构建提示

构建通过；保留项目既有的 Browserslist 数据过期、动态导入分包和大 chunk 警告，未对这些非本次发布问题做改动。
