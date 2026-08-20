# 最终计费倍率调整为 17

时间：2026-08-20

## 变更

按管理员要求，将生产 `sub2api-official-18082` 的 `BILLING_FINAL_MULTIPLIER` 从运行态 `16` 调整为 `17`。

仅修改 `deploy/docker-compose.18082.yml` 中应用容器环境变量，并重建该应用容器。模型分组倍率、用户余额、订单、退款、历史用量、PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷均不修改。

## 验证

构建镜像 `deploy-sub2api:latest` 的 manifest 为 `sha256:68bf16de670bd9f886914917f2cd6ef1a5f3d5fa479bf9d242fcc6d83d211256`。替换容器后确认应用为 `healthy`，回读运行态环境变量为 `BILLING_FINAL_MULTIPLIER=17`；凭证 Secret 挂载保持不变。

`127.0.0.1:18082/health`、本地 Nginx `127.0.0.1:8080/health`、`aaccx.pw/health`、`www.aaccx.pw/health`、`api.aaccx.pw/health` 和 `https://api.aaccx.pw/usage-guide` 均返回 HTTP 200。PostgreSQL、Redis、Nginx 的启动时间未变化，未重建。
