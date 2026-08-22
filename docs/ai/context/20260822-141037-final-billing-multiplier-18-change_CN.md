# 18082 隐藏最终计费倍率调整为 18 倍

时间：2026-08-22

## 变更

按管理员要求，将生产 `sub2api-official-18082` 的隐藏最终计费倍率由 `17x` 调整为 `18x`。

仅修改 `deploy/docker-compose.18082.yml` 中的 `BILLING_FINAL_MULTIPLIER`，并使用既有 `deploy` Compose 项目执行 `up -d --no-deps --force-recreate sub2api`。模型分组倍率、用户余额、订单、退款、历史用量、PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷均未修改。

## 验证

- Compose 渲染配置包含 `BILLING_FINAL_MULTIPLIER=18`。
- 应用容器 `sub2api-official-18082` 已替换，容器 ID 为 `9a585a7d65af116fa6a7b890d5ae0c6947864daddedf8f2ae8ea91e075caa6fc`，状态为 `running (healthy)`。
- 运行态环境变量已回读为 `BILLING_FINAL_MULTIPLIER=18`，账号凭证 Secret 仍挂载到 `/run/secrets/account_credentials_encryption_key`。
- 应用镜像仍为 `sha256:368cdaec0987b33521f585d7ec6c7ca032ef4331549b19163735f35b6aa8e6bd`。
- `127.0.0.1:18082/health`、`127.0.0.1:8080/health`、`aaccx.pw/health`、`www.aaccx.pw/health`、`api.aaccx.pw/health` 和 `https://api.aaccx.pw/usage-guide` 均返回 HTTP 200。
- PostgreSQL、Redis、Nginx 的容器 ID 和启动时间未变化，未重建。
