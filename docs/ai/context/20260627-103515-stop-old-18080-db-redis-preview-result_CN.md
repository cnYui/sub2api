# 停止旧 18080 DB/Redis 与本地预览容器结果

## 结论

已按用户要求停止旧链路相关容器：

- `sub2api-preview`
- `sub2api-main-existingdb-preview`
- `sub2api-postgres`
- `sub2api-redis`

## 验证结果

- `sub2api-preview`：`Exited (0)`，原 `127.0.0.1:18081` 不再监听。
- `sub2api-main-existingdb-preview`：`Exited (0)`，原 `127.0.0.1:18083` 不再监听。
- `sub2api-postgres`：`Exited (0)`。
- `sub2api-redis`：`Exited (137)`。
- 当前公网 app `sub2api-candidate` 仍为 `Up ... healthy`，端口 `127.0.0.1:18084->8080/tcp`。
- 当前公网 DB `sub2api-candidate-postgres` 仍为 `Up ... healthy`。
- 当前公网 Redis `sub2api-candidate-redis` 仍为 `Up ... healthy`。
- `http://127.0.0.1:18084/health` 返回 HTTP `200`。
- `http://127.0.0.1:8080/health` 返回 HTTP `200`。

## 未影响范围

- 未停止 `sub2api-candidate`。
- 未停止 `sub2api-candidate-postgres`。
- 未停止 `sub2api-candidate-redis`。
- 未修改 Nginx、Cloudflare Tunnel、CLIProxyAPI 或数据库数据。
- 未删除容器、镜像或 volume。

## 保留提醒

`sub2api-main-preview`、`sub2api-main-preview-postgres`、`sub2api-main-preview-redis` 仍在运行，对应本地 `127.0.0.1:18082`，它们使用独立 preview DB/Redis，不属于旧 18080 DB/Redis。
