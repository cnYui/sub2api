# CLIProxyAPI 本地管理 API 缺失重建计划

## 背景

- 已将本地 CPA 配置调整为：
  - `remote-management.allow-remote: true`
  - `remote-management.disable-control-panel: false`
- `https://127.0.0.1:8317/management.html` 已返回 200，并下载到容器内 `/CLIProxyAPI/static/management.html`。
- 但 `/v0/management/*` 仍返回 404。

## 进一步判断

- 当前运行容器的二进制是本地 `dev` 构建，没有可识别的 `/v0/management` 路由字符串。
- 仓库当前源码已包含 `/v0/management` 路由注册逻辑。
- 因此问题不是页面资产，而是当前容器镜像内二进制不包含管理 API 路由。

## 操作

- 在 `D:\CodeWorkSpace\CLIProxyAPI-private` 中使用现有 `docker-compose.sub2api-local.yml` 重新构建 `cliproxyapi-local:dev`。
- 重新创建 `cliproxyapi-local-dev` 容器。
- 保留现有挂载：
  - `config.yaml`
  - `auths/`
  - `logs/`
  - `certs/runtime`
- 验证：
  - `/healthz` 200
  - `/management.html` 200
  - `/v0/management/config` 不再是 404，应进入鉴权态或带密钥成功

## 不做

- 不修改 Sub2API。
- 不修改数据库、Redis、Nginx、Cloudflare 或公网映射。
- 不记录完整管理密钥、CPA API Key 或 auth JSON。
