# 欠费提示修正主分支同步与公网重部署

时间：2026-08-20

## Git 同步

- 本地 `main` 提交 `336bc0393`：移除余额套餐订阅页过时的“首周额度不足以抵销欠费，后续额度已暂停”提示，并更新双语文案、回归测试、上下文记录和 `AGENTS.md` 项目记忆。
- 已推送到私有远程 `fork/main`，远程由 `fbef1e420` 快进到 `336bc0393`。
- 工作区中既有的 `deploy/docker-compose.18082.yml` 最终倍率改动、订单 679 诊断文档及其他未提交文件未纳入本次提交。

## 构建与替换

- 使用当前工作区构建 `deploy-sub2api:latest`，Docker 镜像 manifest 为 `sha256:962fcfdd387478c678110eb4067e052e8bbae1f2f26b14fc771b5c4319346aab`。前端生产构建通过；仅保留既有 Vite/Browserslist 和大 chunk 警告。
- 使用 `docker compose --env-file deploy/.env -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --no-deps --force-recreate --no-build sub2api` 仅重建 `sub2api-official-18082` 应用容器。复用原凭证 Secret 挂载；PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷未重建。
- 应用运行态 `BILLING_FINAL_MULTIPLIER=16`，凭证文件仍挂载到 `/run/secrets/account_credentials_encryption_key`，容器状态为 `healthy`。

## 验证

- `127.0.0.1:18082/health`：200。
- 本地 Nginx `127.0.0.1:8080/health`：200。
- `aaccx.pw/health`、`www.aaccx.pw/health`、`api.aaccx.pw/health`：均为 200。
- `https://api.aaccx.pw/usage-guide`：200。
