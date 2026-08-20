# 最终计费倍率调整为 17

时间：2026-08-20

## 变更

按管理员要求，将生产 `sub2api-official-18082` 的 `BILLING_FINAL_MULTIPLIER` 从运行态 `16` 调整为 `17`。

仅修改 `deploy/docker-compose.18082.yml` 中应用容器环境变量，并重建该应用容器。模型分组倍率、用户余额、订单、退款、历史用量、PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷均不修改。

## 验证

替换容器后确认应用为 `healthy`，回读运行态环境变量为 `BILLING_FINAL_MULTIPLIER=17`，并检查本地、Nginx 与三个公网健康端点。
