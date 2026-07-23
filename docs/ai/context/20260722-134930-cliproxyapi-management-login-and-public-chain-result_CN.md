# CLIProxyAPI 管理登录与公网链路更新结果

## 本轮操作

- 在 `D:\CodeWorkSpace\CLIProxyAPI-private\docker-compose.sub2api-local.yml` 增加 `MANAGEMENT_PASSWORD` 环境变量映射：
  - `MANAGEMENT_PASSWORD: ${CLI_PROXY_MANAGEMENT_PASSWORD:-}`
- 保持 CPA 端口只绑定本机：
  - `127.0.0.1:${CLI_PROXY_PORT:-8317}:8317`
- 使用用户提供的本地管理密码作为当前启动环境变量，重新创建并启动 `cliproxyapi-local-dev`。
- 未修改 Sub2API 数据库、Redis、Nginx、Cloudflare Tunnel、用户余额、套餐、usage facts 或 CPA auth 文件。

## 根因确认

- 页面 `/management.html` 可用，但 `remote-management.secret-key` 为空时 CPA 不注册 `/v0/management/*`。
- 登录失败的 404 是管理 API 未启用，不是密码错误。
- 启用 `MANAGEMENT_PASSWORD` 后，管理 API 进入正常鉴权流程。

## 验证结果

- `cliproxyapi-local-dev`：running/healthy，端口为 `127.0.0.1:8317->8317/tcp`。
- `https://127.0.0.1:8317/healthz`：200。
- `https://127.0.0.1:8317/management.html`：200。
- 未带管理密码访问 `https://127.0.0.1:8317/v0/management/config`：401。
- 带管理密码访问 `https://127.0.0.1:8317/v0/management/config`：200。
- `sub2api-dev` 容器内访问 `https://cliproxyapi:8317/v1/models`：401，符合未带 CPA key 的预期，说明网络和 TLS 可达。
- `http://127.0.0.1:8080/health`：200。
- `https://api.aaccx.pw/health`：200。
- `https://api.aaccx.pw/v1/models` 未带用户 key：403，符合 Sub2API 公网鉴权保护。

## 备份

- 启用前已备份 CPA 配置：
  - `D:\CodeWorkSpace\CLIProxyAPI-private\backups\20260722-134519-before-management-secret-enable\config.yaml`

## 后续注意

- 当前管理密码通过容器环境变量生效；如果未来手动重新创建 CPA 容器，需要继续提供 `CLI_PROXY_MANAGEMENT_PASSWORD`。
- 如需长期无环境变量启动，可后续把 `remote-management.secret-key` 设置为 bcrypt 校验值，但不要写入明文。
- 本轮未执行真实模型请求 smoke，因为对话中未提供可用于公网扣费验证的 Sub2API 用户 API Key。
