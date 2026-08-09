# 18082 最终计费倍率调整为 20 倍发布结果

## 变更

- 将 `deploy/docker-compose.18082.yml` 的 `BILLING_FINAL_MULTIPLIER` 从 `18` 调整为 `20`。
- 后续模型请求按 `标准成本 × 分组倍率 × 20` 结算。
- 未修改分组倍率、账户统计倍率、图片或视频独立倍率、模型广场展示价格、历史用量记录和用户余额。

## 应用发布

- 使用 `docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --force-recreate --no-deps sub2api` 仅替换应用容器。
- 替换前应用容器 ID 为 `002de1431f2fd6d036a6caf4f4904c9bf7e3d5b5071fb949d32569cf5dca4f04`，环境变量为 `BILLING_FINAL_MULTIPLIER=18`。
- 替换后应用容器 ID 为 `2d80dfd724d1348255c78333f9a4b91a292b575690d5b2122ac84b8c7bf4d418`，环境变量为 `BILLING_FINAL_MULTIPLIER=20`，最终状态为 `running healthy`。
- PostgreSQL 容器 ID 保持 `d94d74cddbcb30fd0481c1f20b81cda63a1ea65d5ed6e4c92811c72ce846d7cf`，Redis 容器 ID 保持 `d6ea60b580181b4d084fef022192b623e5db3fa44caa567b186cceda4e00cd66`，均未重建。

## 公网链路恢复

- 应用容器和 `sub2api-public-nginx-local` 在 2026-08-08 01:04:31 UTC 同时收到正常终止信号并以退出码 `0` 停止，非配置校验或应用崩溃。
- 应用容器使用已有配置重新启动后，连续 30 秒保持 `running healthy`。
- Nginx 配置文件未修改，先通过 `nginx -t` 后启动原有 Nginx 容器；其本地 `http://127.0.0.1:8080/health` 返回 200。
- Cloudflared 进程在发布后不存在，已通过既有 `start-cloudflared-windows-aaccx.ps1` 恢复；复用原 Tunnel ID、凭证和配置，未重建 Tunnel。

## 验证

- Compose 渲染配置包含 `BILLING_FINAL_MULTIPLIER: "20"`。
- `go test -run '^TestLoadBillingFinalMultiplierFromEnvironment$' -count=1 ./internal/config` 通过。
- `http://127.0.0.1:18082/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200。
