# 公网服务恢复记录

## 目标

按要求继续恢复当前项目公网服务。

## 执行

- 检查确认 Docker Server `29.1.3` 正常。
- 应用容器 `sub2api-official-18082` 为 `running/healthy`，PostgreSQL、Redis 和 `sub2api-public-nginx-local` 均在运行；本地 `18082` 和 `8080` 健康检查已正常。
- 检查发现 `cloudflared.exe` 进程缺失，因此三个公网域名返回 Cloudflare `530`。
- 使用既有配置 `D:\CodeWorkSpace\sub2api\deploy\cloudflared-windows-aaccx.yml` 和凭证启动 Tunnel `${OPS_TUNNEL_ID}`，进程 ID 为 `37684`。
- Tunnel 成功注册 4 条 QUIC 连接；启动日志中的 WLAN DNS `192.168.1.1` 仍有一次 `server misbehaving`，但 Cloudflare DNS、UDP、TCP 和 API 预检均通过。

## 核验

- `http://127.0.0.1:18082/health` 返回 HTTP `200`。
- `http://127.0.0.1:8080/health` 返回 HTTP `200`。
- `https://aaccx.pw/health` 返回 HTTP `200`。
- `https://www.aaccx.pw/health` 返回 HTTP `200`。
- `https://api.aaccx.pw/health` 返回 HTTP `200`。

## 边界

- 未重启应用、PostgreSQL、Redis 或 Nginx。
- 未修改 Cloudflare 配置、DNS、代码、业务数据或数据卷。
