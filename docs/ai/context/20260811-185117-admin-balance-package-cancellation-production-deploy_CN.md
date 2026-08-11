# 管理员取消余额套餐入口公网发布

## 发布范围

- 基于当前工作区构建 `deploy-sub2api:latest`，镜像摘要为 `sha256:fee7afdf593f5750488f5a820de3a4e076ff1c4750db52a2afa3a4d54e920d6b`。
- 使用 `docker compose --env-file deploy/.env -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --no-deps --force-recreate sub2api` 仅替换 `sub2api-official-18082`。
- 发布内容包含管理员订单页的“取消套餐”入口，以及仅停止余额套餐权益、不发起支付退款的管理接口。

## 运行边界

- PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷未重建或修改。
- 应用数据卷继续挂载为 `deploy/data-18082`。
- 账户凭证加密 secret 继续挂载到 `/run/secrets/account_credentials_encryption_key`。
- 合并后的生产 Compose 实际配置为 `BILLING_FINAL_MULTIPLIER=18`；本次发布未改变该配置。

## 核验

- 新应用容器 ID：`734e9cb145e2d541aaf87a12f08418e86922714daee478ba4acfacb14232d67f`，状态为 `healthy`。
- `docker exec sub2api-public-nginx-local nginx -t` 通过。
- `http://127.0.0.1:18082/health`、`http://127.0.0.1:8080/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200。
- 公网 `POST /api/v1/admin/payment/orders/625/cancel-balance-package` 在不携带认证信息时返回 `401 UNAUTHORIZED`，确认路由已发布且受认证保护；没有执行任何套餐取消操作。
- 容器启动日志确认后台任务与 HTTP 服务均已正常启动，且上线后已有正常 `200` 的模型转发请求。
