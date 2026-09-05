# 公网服务恢复记录

## 目标

按要求启动当前项目并恢复既有公网服务链路。

## 原因

检查发现 Docker CLI 无法连接 `dockerDesktopLinuxEngine`，Docker Desktop Linux 引擎未运行；本机应用、数据库和缓存容器因此未处于运行状态，公网域名返回 Cloudflare `530`。

## 执行

- 启动已安装的 Docker Desktop，确认 Docker Server `29.1.3` 就绪。
- 复用当前 `deploy/docker-compose.18082.yml` 对应的既有容器和数据卷；Docker Desktop 自动恢复 `sub2api-official-18082`、`sub2api-official-18082-postgres`、`sub2api-official-18082-redis` 和 `sub2api-public-nginx-local`。
- 确认应用继续使用既有 `deploy-sub2api` 镜像并绑定 `127.0.0.1:18082`；PostgreSQL 和 Redis 均为 healthy，Nginx 继续监听 `127.0.0.1:8080`。
- 执行 `docker exec sub2api-public-nginx-local nginx -t`，Nginx 配置语法检查通过。
- 使用既有配置 `D:\CodeWorkSpace\sub2api\deploy\cloudflared-windows-aaccx.yml` 和凭证 `C:\Users\yui\.cloudflared\${OPS_TUNNEL_ID}.json` 启动 Cloudflare Tunnel `${OPS_TUNNEL_ID}`。进程 ID 为 `13928`，成功注册 4 条连接，日志位于 `D:\CodeWorkSpace\sub2api\deploy\logs\cloudflared-aaccx-20260830-083644.out.log`。

## 核验

- `http://127.0.0.1:18082/health` 返回 HTTP `200`，内容为 `{"status":"ok"}`。
- `http://127.0.0.1:8080/health` 返回 HTTP `200`，内容为 `{"status":"ok"}`。
- `https://aaccx.pw/health` 返回 HTTP `200`。
- `https://www.aaccx.pw/health` 返回 HTTP `200`。
- `https://api.aaccx.pw/health` 返回 HTTP `200`。
- `https://aaccx.pw/usage-guide` 返回 HTTP `200`。

## 边界

- 未重新构建或替换应用镜像。
- 未重建 PostgreSQL、Redis、Nginx、Cloudflare Tunnel 配置或任何数据卷。
- 未修改数据库、Redis、用户数据、订单、余额、计费倍率、业务配置或代码；本次仅新增恢复记录并更新 `AGENTS.md` 索引。
