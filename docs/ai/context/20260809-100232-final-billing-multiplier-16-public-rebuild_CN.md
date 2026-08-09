# 18082 16 倍隐藏倍率公网重建发布结果

## 目标

基于当前工作区源码重新构建应用镜像，并将隐藏最终计费倍率为 `16x` 的 18082 应用容器更新至公网链路。

## 发布

- 使用 `docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml build sub2api` 成功构建 `deploy-sub2api:latest`。
- 使用 `docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --force-recreate --no-deps sub2api` 仅替换应用容器。
- 新应用容器 ID 为 `bf1cf2749213fdd5db98c770dca82da9d0a82f627ca8825039ffb08adb441dfb`，镜像 ID 为 `sha256:16e6ea063b52b6ee73f644b2ad72f49754ebd541691c8e72cfff79cd44206a9b`，状态为 `running healthy`。
- 运行时环境变量保持 `BILLING_FINAL_MULTIPLIER=16`。
- PostgreSQL 容器 ID 保持 `d94d74cddbcb30fd0481c1f20b81cda63a1ea65d5ed6e4c92811c72ce846d7cf`，Redis 容器 ID 保持 `d6ea60b580181b4d084fef022192b623e5db3fa44caa567b186cceda4e00cd66`；Nginx 和 Cloudflare Tunnel 未重建或修改。

## 验证

- `http://127.0.0.1:18082/health` 返回 HTTP 200。
- `https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200。
- 模型广场展示价格仍不叠加隐藏最终倍率；历史用量与余额未改写。
