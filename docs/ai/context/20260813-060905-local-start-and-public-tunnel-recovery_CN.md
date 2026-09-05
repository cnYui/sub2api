# 本地启动与公网映射恢复

## 目标

恢复当前工作区对应的既有 Sub2API 生产容器，并使已有 Cloudflare Tunnel 公网链路重新可用。

## 执行内容

- 发现 Docker Desktop Linux 引擎未运行后，启动 Docker Desktop 并确认 Docker Server `29.1.3` 就绪。
- 复用既有 `sub2api-official-18082` Docker Compose 栈及其数据卷；Docker 自动恢复应用、PostgreSQL、Redis 和 `sub2api-public-nginx-local` 容器。
- 确认应用、PostgreSQL、Redis 均健康，应用继续绑定 `127.0.0.1:18082`。
- 执行 `docker exec sub2api-public-nginx-local nginx -t`，确认 Nginx 配置有效；Nginx 继续监听 `127.0.0.1:8080` 并反向代理到 `host.docker.internal:18082`。
- 使用既有 `D:\CodeWorkSpace\sub2api\deploy\start-cloudflared-windows-aaccx.ps1` 启动 Tunnel `${OPS_TUNNEL_ID}`。

## 核验

- `http://127.0.0.1:18082/health` 返回 `200 {"status":"ok"}`。
- `http://127.0.0.1:8080/health` 返回 `200 {"status":"ok"}`。
- `https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 `200 {"status":"ok"}`。

## 边界

- 未重建或替换应用镜像、PostgreSQL、Redis、Nginx、Cloudflare Tunnel 配置和任何数据卷。
- 未修改运行时业务配置、密钥、用户数据、订单或计费倍率。
