# CLIProxyAPI 容器公网链路恢复计划

时间：2026-07-22 12:37:47 +09:00

## 目标

先恢复本机 Docker 内 cliproxyapi-local-dev，让当前映射公网的 Sub2API 能继续通过本地 8317/共享 Docker 网络访问 CPA。

## 边界

- 只操作 CLIProxyAPI 本地 Docker 容器和必要的 Compose 启动。
- 不修改 Sub2API 数据库、Redis、Nginx、Cloudflare Tunnel、用户余额、套餐或 CPA 账号文件。
- CPA 管理页 /v0/management/* 404 作为后续问题处理，不阻塞公网模型链路优先恢复。

## 备份

- 已有配置备份：D:\CodeWorkSpace\CLIProxyAPI-private\backups\20260722-122604-before-local-management-panel\config.yaml。
- 本轮启动前只读核对容器、端口和 health，不新增密钥文档。

## 验证

1. cliproxyapi-local-dev 容器处于 running/healthy。
2. 主机侧 https://127.0.0.1:8317/healthz 返回 200。
3. Sub2API 容器侧能访问 CPA /v1/models，未带 CPA 内部 key 时 401 属于预期，网络/TLS 可达即可。
4. 不把 CPA 8317 直接映射公网。

## 回滚

如启动后异常，停止本轮启动的 cliproxyapi-local-dev，并恢复上述备份配置后重新启动；不触碰数据卷。
