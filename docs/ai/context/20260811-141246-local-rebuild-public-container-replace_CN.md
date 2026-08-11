# 本地 Docker 重建与公网应用容器替换

## 执行口径

用户要求本地重新 Docker 后再替换公网 Docker，同时不能影响公网依赖服务。本次将“本地重新 Docker”按应用镜像本地重建执行，不重启 Docker Desktop 引擎；重启引擎会同时停止公网应用、Nginx、数据库和 Redis，与“不影响公网服务”冲突。

## 执行结果

- 重建命令使用 `deploy/docker-compose.dev.yml` 与 `deploy/docker-compose.18082.yml`，只构建 `sub2api` 服务。
- 新镜像为 `deploy-sub2api:latest`，镜像摘要 `sha256:c517dacbeb1b3a27a8ab3ad93769ab6e40242e9f2c336efa6ac2cd3187d2fcfc`。
- 通过 `docker compose ... up -d --no-deps --force-recreate sub2api` 仅替换 `sub2api-official-18082`。
- `BILLING_FINAL_MULTIPLIER=16` 保持不变，账号凭证加密 secret 挂载仍存在。
- PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷未重建或修改。

## 健康检查

以下端点替换后均返回 HTTP 200：

- `http://127.0.0.1:18082/health`
- `http://127.0.0.1:8080/health`
- `https://aaccx.pw/health`
- `https://www.aaccx.pw/health`
- `https://api.aaccx.pw/health`
- `https://aaccx.pw/usage-guide`

应用容器已进入 `healthy`；数据库、Redis 和 Nginx 的容器启动时间保持为原值。
