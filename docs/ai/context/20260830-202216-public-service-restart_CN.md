# 公网服务重启记录

## 目标

按要求重新启动当前项目公网应用服务，并确认公网链路恢复。

## 执行

- 检查发现应用容器原本为 `running/healthy`，但 Cloudflare Tunnel 进程已退出，三个公网健康端点返回 Cloudflare `530`。
- 使用既有配置 `D:\CodeWorkSpace\sub2api\deploy\cloudflared-windows-aaccx.yml` 和凭证启动 Tunnel `${OPS_TUNNEL_ID}`，进程 ID 为 `21604`，成功注册 4 条连接。
- 执行 `docker restart sub2api-official-18082`，仅重启应用容器；PostgreSQL、Redis、Nginx、Tunnel 配置和数据卷未重启或修改。

## 核验

- 应用容器恢复为 `running/healthy`，继续使用既有 `deploy-sub2api` 镜像，运行时 `BILLING_FINAL_MULTIPLIER=18`。
- `http://127.0.0.1:18082/health` 返回 HTTP `200`。
- `http://127.0.0.1:8080/health` 返回 HTTP `200`。
- `https://aaccx.pw/health` 返回 HTTP `200`。
- `https://www.aaccx.pw/health` 返回 HTTP `200`。
- `https://api.aaccx.pw/health` 返回 HTTP `200`。
- `https://aaccx.pw/usage-guide` 返回 HTTP `200`。

## 边界

- 未重建或替换应用镜像。
- 未重启 PostgreSQL、Redis 或 Nginx。
- 未修改数据库、Redis、用户数据、订单、余额、计费倍率、业务配置或代码。
