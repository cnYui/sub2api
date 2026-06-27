# 停止旧 18080 DB/Redis 与本地预览容器计划

## 目标

按用户要求关闭旧链路相关容器：

- `sub2api-postgres`
- `sub2api-redis`
- `sub2api-preview`，本地端口 `127.0.0.1:18081`
- `sub2api-main-existingdb-preview`，本地端口 `127.0.0.1:18083`

## 操作顺序

1. 复核当前公网链路仍为 `sub2api-candidate -> sub2api-candidate-postgres/sub2api-candidate-redis`。
2. 先停止依赖旧 DB/Redis 的预览 app：
   - `sub2api-preview`
   - `sub2api-main-existingdb-preview`
3. 再停止旧 DB/Redis：
   - `sub2api-postgres`
   - `sub2api-redis`
4. 验证：
   - `18081`、`18083` 不再监听。
   - `sub2api-postgres`、`sub2api-redis` 为 Exited。
   - `18084`、`8080` 仍返回 200。

## 不做

- 不停止 `sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis`。
- 不修改 Nginx、Cloudflare Tunnel、CLIProxyAPI 或数据库数据。
- 不删除容器、镜像或 volume。
