# 当前公网服务停止记录

- 时间：2026-08-08 10:04:52（Asia/Tokyo）
- 操作：停止 `sub2api-public-nginx-local` 与 `sub2api-official-18082` 容器，并终止 Cloudflare Tunnel 进程（PID 22188）。
- 保留：`sub2api-official-18082-postgres` 与 `sub2api-official-18082-redis` 未停止，数据未改动。
- 验证：本机端口 `8080`、`18082` 均无监听；`aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 的 `/health` 请求均返回 `502 Bad Gateway`，公网入口已不可用。
