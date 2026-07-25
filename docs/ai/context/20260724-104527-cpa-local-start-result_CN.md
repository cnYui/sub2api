# CPA 本地容器启动结果

## 结果

- 已启动本地 CPA 容器 `cliproxyapi-local-dev`。
- 容器状态为 `running / healthy`。
- 端口保持为 `127.0.0.1:8317->8317/tcp`，未暴露到 `0.0.0.0`。
- 本地访问入口为 `https://localhost:8317/management.html` 或 `https://127.0.0.1:8317/management.html`。

## 验证

- `https://127.0.0.1:8317/management.html` 返回 `HTTP/1.1 200 OK`，`Content-Type: text/html; charset=utf-8`。
- `https://localhost:8317/management.html` 返回 `HTTP/1.1 200 OK`。
- `/v0/management/*` 管理接口已启用；未带管理 key 时返回 `401 missing management key`，符合保护预期。
- `http://127.0.0.1:18080/health` 返回 `{"status":"ok"}`。
- `http://127.0.0.1:18086/health` 返回 `{"status":"ok"}`。
- 当前 `sub2api-dev`、`sub2api-upstream-latest`、`sub2api-public-nginx-local` 仍保持原端口和健康状态，未切换公网链路。

## 备份

- 启动前已保存脱敏 Docker inspect 快照：
  - `backups/20260724-104128-cliproxyapi-local-dev-inspect-sanitized.json`
  - JSON 可读。
  - SHA-256：`A4D9D0D4D92DC66C70382AD3C66950FD2B051C11846630DE966F3B2EE42B5E9A`

## 注意

- CPA 当前使用本地自签 TLS 证书，证书 SAN 覆盖 `cliproxyapi`、`localhost`、`host.docker.internal` 和 `127.0.0.1`，但浏览器未信任本地 CA 时会显示证书安全提示。
- 当前容器日志显示本次启动加载到 `0` 个 auth 条目；本次只恢复 CPA 本地服务与控制台，不导入或恢复账号文件。

## 回滚

- 如需关闭本地 CPA：`docker stop cliproxyapi-local-dev`。
