# CLIProxyAPI 本地管理页打不开处理计划

## 背景

- 用户在 Chrome 打开 `https://127.0.0.1:8317/management.html` 返回 HTTP 404。
- 当前容器 `cliproxyapi-local-dev` 正常运行，8317 只发布到 `127.0.0.1`。
- CPA 健康检查应使用 `https://127.0.0.1:8317/healthz`。

## 判断

- 404 不是证书问题；证书只会触发浏览器“不安全”提示。
- 当前 `D:\CodeWorkSpace\CLIProxyAPI-private\config.yaml` 中：
  - `remote-management.disable-control-panel: true`，会直接关闭 `/management.html`。
  - `remote-management.allow-remote: false`，Docker Desktop 下宿主机访问容器管理 API 可能被识别为非 localhost。

## 操作范围

- 只修改本地 CPA 配置文件 `D:\CodeWorkSpace\CLIProxyAPI-private\config.yaml`。
- 修改前备份配置。
- 重启 `cliproxyapi-local-dev`。
- 验证 `/healthz`、`/management.html` 和管理 API 的返回状态。
- 不修改 Sub2API、公网映射、数据库、Redis、Nginx 或 Cloudflare。

## 安全边界

- 不记录完整管理密钥、CPA API Key、HMAC 或任何 auth JSON 内容。
- CPA 端口当前仍仅绑定 `127.0.0.1:8317`，不会新增公网暴露。
