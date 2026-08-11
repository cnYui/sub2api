# WorkBuddy 教程公网 Docker 发布

## 发布范围

- 基于当前工作区构建 `deploy-sub2api:latest`，镜像摘要为 `sha256:3baf4276113ae86264440c6c0ced6b390bcba1f540ffd35d7a5b0a4b067ce3b5`。
- 使用 `docker compose --env-file deploy/.env -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --no-deps --force-recreate sub2api` 仅替换 `sub2api-official-18082`。
- 构建中包含 WorkBuddy 使用方法主题及四张截图；构建保留既有 Browserslist、动态导入和 chunk 大小警告。

## 运行状态

- 新应用容器 ID：`ab068d26c2b280d170a1f0c691aad4e435fa3aa92623b6e6f0724bd68fc7c4b9`，状态为 `healthy`。
- `BILLING_FINAL_MULTIPLIER=16`；账号凭证加密 secret 继续以只读方式挂载到 `/run/secrets/account_credentials_encryption_key`。
- PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷未重建或修改；PostgreSQL、Redis 均为 `healthy`。

## 核验

- `docker exec sub2api-public-nginx-local nginx -t` 通过。
- `http://127.0.0.1:18082/health`、`http://127.0.0.1:8080/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200。
- `https://aaccx.pw/usage-guide` 返回 HTTP 200。
- 四张 WorkBuddy 静态截图均通过 `https://aaccx.pw/assets/` 返回 `200 image/png`。
