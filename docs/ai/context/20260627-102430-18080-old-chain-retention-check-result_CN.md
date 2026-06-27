# 18080 旧链路进程保留排查结果

## 结论

旧 `18080` 公网应用链路没有继续对外服务：`127.0.0.1:18080` 无监听，旧 `sub2api` app 容器为 `Exited (0)`，Nginx 当前生效配置全部指向 `127.0.0.1:18084`。

但旧链路相关的数据库和 Redis 容器仍在运行，并且被两个预览容器连接：

- `sub2api-preview`，IP `172.18.0.5`，监听 `127.0.0.1:18081`，`/health` 返回 200。
- `sub2api-main-existingdb-preview`，IP `172.18.0.6`，监听 `127.0.0.1:18083`，根页面返回 200。

这说明旧 DB/Redis 不是公网 18084 链路的一部分，但仍被本机预览进程保留使用。

## 关键证据

- `sub2api-candidate`：Up healthy，端口 `127.0.0.1:18084->8080/tcp`。
- `sub2api`：`Exited (0)`，无端口映射。
- `sub2api-postgres`：Up healthy。
- `sub2api-redis`：Up healthy。
- `lsof 18080`：无监听进程。
- `curl http://127.0.0.1:18080/health`：连接失败，HTTP `000`。
- `curl http://127.0.0.1:18084/`：HTTP `200`。
- `curl http://127.0.0.1:8080/purchase`：HTTP `200`。
- Nginx 当前生效 server 配置中 `aaccx-root.conf` 和 `cliproxy.conf` 的 Sub2API 反代均为 `127.0.0.1:18084`；`18080` 只出现在 `backups/` 目录里的旧备份配置。

## 数据库与 Redis 保留情况

- 当前 18084 app 环境指向：
  - `DATABASE_HOST=sub2api-candidate-postgres`
  - `REDIS_HOST=sub2api-candidate-redis`
- 旧 `sub2api-postgres` 当前计数：
  - `users=32`
  - `keys=22`
  - `migrations=188`
- 候选 `sub2api-candidate-postgres` 当前计数：
  - `users=48`
  - `keys=40`
  - `migrations=191`
- 旧 `sub2api-postgres` 的活跃连接来自：
  - `172.18.0.5` -> `sub2api-preview`
  - `172.18.0.6` -> `sub2api-main-existingdb-preview`
- 旧 `sub2api-redis` 也有来自 `172.18.0.5` 和 `172.18.0.6` 的客户端连接。

## 前端内容保留情况

- `18080` 不再提供任何前端页面。
- `18084` 和 nginx `8080` 提供当前候选前端资源，资产指纹包含：
  - `index-CXmPznNo.js`
  - `pkg-vue-BqGtxt06.js`
  - `index-nffSQZgD.css`
- `18083` 仍由 `sub2api-main-existingdb-preview` 提供预览前端，资产指纹包含：
  - `index-BS-WXf6A.js`
  - `pkg-vue-BqGtxt06.js`
  - `index-DQzRIYzN.css`
- `18082` 也有独立预览前端，但它连接自己的 `sub2api-main-preview-postgres/redis`，不是旧 18080 DB。
- `18081` 的 `sub2api-preview` 健康检查可用，但根页面返回 404。

## 风险判断

- 公网请求不会落到旧 `18080` app，因为没有监听且 Nginx 不再指向它。
- 旧 DB/Redis 仍在内存和进程中保留，并被预览容器使用；如果误以为旧 DB 已完全停用，后续清理会影响 `18081/18083` 预览环境。
- 旧 `weishaw/sub2api:latest` 镜像和已退出的 `sub2api` 容器仍保留在本机，但不属于运行中公网链路。

## 本次未做

- 未停止、删除、重启任何容器。
- 未修改 Nginx、Docker、数据库、Redis 或 Cloudflare Tunnel。
- 未输出或记录完整 API Key、内部 token、HMAC secret、SMTP 密码。
