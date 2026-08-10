# main 合并与 Codex 教程公网重新发布

## 提交

- 当前工作区原本已在 `main`，教程改动直接提交为 `78ae21ccb`（`feat: 更新 Codex CC Switch KIMI 接入教程`）。
- 提交包含 10 步页面文案、10 张截图、资源测试和两份上下文记录；未包含密钥文件或构建临时产物。

## 发布

- 基于提交 `78ae21ccb` 构建 `deploy-sub2api:latest`，镜像摘要为 `sha256:ecb8a3aae441a8204a8b32d152a7901e4f138c1e3c985134ed42a7034c41257b`。
- 使用 `docker compose --env-file deploy/.env -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --no-deps --force-recreate sub2api` 仅替换 `sub2api-official-18082`。
- 应用容器 ID 为 `014a1a9cde627c5c501a25abffbd26dd5b1682ad607de0dbe82b7acdbd9c3787`，状态 `healthy`；运行时 `BILLING_FINAL_MULTIPLIER=16`，账号凭证加密 secret 挂载保持存在。
- PostgreSQL、Redis、Nginx、Cloudflare Tunnel、数据卷未重建或修改。

## 核验

- `docker exec sub2api-public-nginx-local nginx -t` 通过。
- `http://127.0.0.1:18082/health`、`http://127.0.0.1:8080/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200。
- `https://aaccx.pw/usage-guide` 返回 HTTP 200。
