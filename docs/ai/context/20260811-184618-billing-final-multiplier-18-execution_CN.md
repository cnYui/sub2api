# 隐藏最终计费倍率调整为 18 倍

## 目标与范围

- 按管理员要求，将生产 `sub2api-official-18082` 的隐藏最终计费倍率从 `16x` 调整为 `18x`。
- 唯一配置变更为 `deploy/docker-compose.18082.yml` 中的 `BILLING_FINAL_MULTIPLIER=18`。
- 未修改模型分组倍率、账户统计倍率、图片或视频独立倍率、用户余额、订单、历史用量与缓存数据。

## 执行

- 调整前，部署配置和运行容器环境变量均为 `BILLING_FINAL_MULTIPLIER=16`。
- 复用原有账号凭证密钥挂载，以 Compose 配置校验后执行 `up -d --no-deps --force-recreate sub2api`。
- 仅替换应用容器；PostgreSQL、Redis、Nginx、Cloudflare Tunnel 与数据卷均未重建或修改。
- 新容器 ID 为 `941f65b1033b4d267ac283aeae7c3f50eb8609334da47f0fb5ba57510faaa294`，状态为 `healthy`，运行时环境变量回读为 `BILLING_FINAL_MULTIPLIER=18`。

## 验证

- Compose 配置校验通过。执行时保留既有 `REDIS_PASSWORD` 未设置警告，未影响 Redis 现有运行配置。
- `docker exec sub2api-public-nginx-local nginx -t` 通过。
- `http://127.0.0.1:18082/health`、`http://127.0.0.1:8080/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200。
