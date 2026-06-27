# 18080 旧链路进程保留排查计划

## 目标

确认 18080 旧链路相关的应用容器、数据库、Redis、端口监听、Nginx 反代和前端内容是否仍在进程中保留或被公网链路引用。

## 排查范围

- Docker 容器状态：`sub2api`、`sub2api-postgres`、`sub2api-redis`、`sub2api-candidate*`。
- 端口监听：`18080`、`18084`、`8080`、`8317`。
- Nginx 配置引用：是否仍有 `127.0.0.1:18080` 或旧 upstream。
- HTTP 可达性：`127.0.0.1:18080`、`127.0.0.1:18084`、`127.0.0.1:8080` 的 `/health` 与前端 HTML 指纹。
- 容器运行环境：当前对外 app 连接的是候选 DB 还是旧 DB。

## 不做

- 不停止、不删除、不重启容器。
- 不修改 Nginx、Docker、数据库、Redis 或 Cloudflare Tunnel。
- 不输出或记录完整 API Key、内部 token、HMAC secret、SMTP 密码。
