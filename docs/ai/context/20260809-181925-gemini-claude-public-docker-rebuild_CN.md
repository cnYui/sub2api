# Gemini 与 Claude 渠道公网镜像重建

## 目标

基于当前工作区重新构建应用镜像，并替换公网 `18082` 应用容器，使新增 Gemini 与 Claude 渠道配置及监控逻辑在公网实例生效。

## 执行范围

- 使用 `deploy/docker-compose.dev.yml` 与 `deploy/docker-compose.18082.yml` 构建 `deploy-sub2api:latest`。
- 新镜像摘要：`sha256:f2f422b244fe4f9a792ad71d08ac97a25eb09432d9e67ac873538c7a81c984f3`。
- 使用 `docker compose up -d --no-deps --force-recreate --no-build sub2api` 仅替换 `sub2api-official-18082`。
- 复用现有 `data-18082`、外部网络和账号凭证 Secret 挂载。
- PostgreSQL、Redis、Nginx、Cloudflare Tunnel 与数据卷未重建或修改。

## 核验

- 应用容器新 ID 为 `6db906c7103f759aaae1ee9e9247ae3fd3d122179731d519c392c26b3b9316a2`，状态为 `healthy`。
- 应用日志确认监控调度器加载 14 条任务。
- `/health`：`127.0.0.1:18082`、`127.0.0.1:8080`、`aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 均返回 HTTP 200。
- `sub2api-public-nginx-local nginx -t` 通过。
- Gemini 监控 ID 13 与 Claude 监控 ID 14 均为 `models` 模式、启用、1800 秒间隔；新容器首轮历史均为 `operational`。
- Redis `sched:acc:1165` 与 `sched:acc:1166` 调度快照存在，未匹配到 `api_key`、`access_token`、`refresh_token` 或 `sk-` 明文标记。
