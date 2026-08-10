# Docker Desktop 重启后的公网服务恢复核验

## 操作

- 在完成 Codex 教程的新镜像发布后，按管理员要求执行 `docker desktop restart`。
- Docker Desktop 引擎恢复运行后，应用、PostgreSQL、Redis、Nginx 等既有容器自动启动；未再次重建镜像、数据卷或公网链路配置。

## 恢复结果

- `sub2api-official-18082` 恢复为 `healthy`，继续使用镜像 `sha256:ecb8a3aae441a8204a8b32d152a7901e4f138c1e3c985134ed42a7034c41257b`。
- PostgreSQL 与 Redis 均恢复为 `healthy`；Nginx 正常运行。
- 运行时 `BILLING_FINAL_MULTIPLIER=16`，账号凭证加密 secret 仍挂载至 `/run/secrets/account_credentials_encryption_key`。
- `docker exec sub2api-public-nginx-local nginx -t` 通过。
- `http://127.0.0.1:18082/health`、`http://127.0.0.1:8080/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 与 `https://aaccx.pw/usage-guide` 均返回 HTTP 200。
