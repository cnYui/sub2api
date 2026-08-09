# 公网应用容器重建与替换

## 目标

按管理员要求，将当前工作区构建为新的公网应用镜像，并替换 `sub2api-official-18082`。

## 执行范围

- 使用 `deploy/docker-compose.dev.yml` 与 `deploy/docker-compose.18082.yml` 构建 `deploy-sub2api:latest`。
- 通过 `docker compose up -d --no-deps --force-recreate sub2api` 仅替换应用容器。
- 复用当前生产密钥文件挂载、18082 数据目录、外部 Docker 网络与现有环境配置。
- 未重建或修改 PostgreSQL、Redis、Nginx、Cloudflare Tunnel 及其数据卷。

## 结果

- 新镜像 ID：`sha256:99730d92b42caae2babafeb7d951cbfc46b15ab6e5284baf902a12a2b4ad5474`。
- 新 `sub2api-official-18082` 容器已替换并进入 `healthy`。
- `sub2api-official-18082-postgres` 与 `sub2api-official-18082-redis` 保持原容器持续运行且均为 healthy。
- 构建完成前端生产构建和后端嵌入式二进制构建；构建中只有既有的前端大 chunk 与 Browserslist 数据提示，无构建失败。

## 核验

| 目标 | 结果 |
| --- | --- |
| `sub2api-public-nginx-local nginx -t` | 通过 |
| `http://127.0.0.1:18082/health` | HTTP 200 |
| `http://127.0.0.1:8080/health` | HTTP 200 |
| `https://aaccx.pw/health` | HTTP 200 |
| `https://www.aaccx.pw/health` | HTTP 200 |
| `https://api.aaccx.pw/health` | HTTP 200 |
