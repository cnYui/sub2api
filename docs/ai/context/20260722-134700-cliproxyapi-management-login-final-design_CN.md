# CLIProxyAPI 管理登录最终设计

## 问题

- `https://127.0.0.1:8317/management.html#/login` 能加载页面。
- 登录时报 `HTTP 404: 未找到 Management API`。
- 后端 `/v0/management/config` 返回 404。

## 根因

CPA 源码只有在以下任一条件成立时才注册 `/v0/management/*`：

- `remote-management.secret-key` 非空。
- `MANAGEMENT_PASSWORD` 环境变量非空。
- 本地 TUI 启动密码非空。

当前配置中 `remote-management.secret-key` 是空值，因此管理 API 没有注册。页面 200 只是静态资源可用，不代表管理 API 可用。

## 最终方案

采用运行时环境变量启用管理 API：

- 在 `D:\CodeWorkSpace\CLIProxyAPI-private\docker-compose.sub2api-local.yml` 增加：
  - `MANAGEMENT_PASSWORD: ${CLI_PROXY_MANAGEMENT_PASSWORD:-}`
- 本轮启动 `cliproxyapi-local-dev` 时在当前 PowerShell 会话注入 `CLI_PROXY_MANAGEMENT_PASSWORD`。
- 不把明文密码、bcrypt 校验值或管理密钥写入 `docs/ai/context/`。
- 不修改 Sub2API、DB、Redis、Nginx、Cloudflare Tunnel 或 CPA auth 文件。

## 端口边界

- CPA 继续只发布到 `127.0.0.1:8317`。
- 公网用户只访问 Sub2API：`https://api.aaccx.pw/v1`。
- CPA 管理页只作为本机管理入口，不直接公网暴露。

## 验收标准

- `https://127.0.0.1:8317/healthz` 返回 200。
- `https://127.0.0.1:8317/management.html` 返回 200。
- 未带管理密码访问 `/v0/management/config` 返回 401，而不是 404。
- 带管理密码访问 `/v0/management/config` 返回 200。
- `sub2api-dev` 容器内访问 `https://cliproxyapi:8317/v1/models` 未带 CPA key 返回 401，说明网络和 TLS 可达。
- `https://api.aaccx.pw/health` 返回 200。
- 公网 `/v1/models` 未带用户 key 返回 403/401，属于 Sub2API 鉴权保护。

## 回滚

- 如果 CPA 更新异常，恢复启动前备份：
  - `D:\CodeWorkSpace\CLIProxyAPI-private\backups\20260722-134519-before-management-secret-enable\config.yaml`
- 然后重新启动 `cliproxyapi-local-dev`。
- 不需要改 Sub2API 数据库或 Redis。
