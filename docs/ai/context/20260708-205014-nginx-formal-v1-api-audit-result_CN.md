# Nginx 正式 /v1 API 路径核查结果

> 2026-07-08 20:50 JST，只读核查；未改 Nginx、未改 Docker、未构建、未提交。

## 结论

- 当前公网 Nginx 入口已经按“只保留 `/v1/*` 为正式模型 API”的方向拦截裸模型 API。
- 不能表述为“所有请求都往 `/v1` 发送”：`/api/*`、控制台页面、静态资源、`/health` 等仍会按配置代理或服务页面，这是正常非模型 API/前端流量。
- Nginx 不是把裸 `/responses` rewrite/redirect 到 `/v1/responses`，而是直接返回 `400 INVALID_BASE_URL`。
- 直连后端 `127.0.0.1:18084` 仍有遗漏：`/models`、`/chat/completions`、`/embeddings` 返回前端 HTML 200，不符合“后端也禁用裸路径”的目标；公网入口当前已由 Nginx 拦住，但后端直连仍需继续修。

## 已核对配置

- 当前容器：`sub2api-candidate:20260708-203429-cc845e468-formal-v1-api`，`127.0.0.1:18084->8080`，状态 healthy。
- `nginx -t` 通过。
- 生效配置文件：
  - `/opt/homebrew/etc/nginx/servers/cliproxy.conf`
  - `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`
- 两个 server 均包含裸模型路径拦截：`/models`、`/responses`、`/responses/*`、`/chat/completions`、`/embeddings`、`/images/generations`、`/images/edits`、`/backend-api/codex/responses`、`/backend-api/codex/responses/*`。

## 8080 Nginx 入口验证

Host `api.aaccx.pw` 与 `aaccx.pw` 下列路径均返回 `400 INVALID_BASE_URL`：

- `GET /models`
- `POST /responses`
- `POST /responses/compact`
- `POST /chat/completions`
- `POST /embeddings`
- `POST /images/generations`
- `POST /images/edits`
- `POST /backend-api/codex/responses`
- `POST /backend-api/codex/responses/compact`

正式路径仍会进入后端鉴权，不是 `INVALID_BASE_URL`：

- `POST /v1/responses` 返回 `401 API_KEY_REQUIRED`
- `POST /v1/chat/completions` 返回 `401 API_KEY_REQUIRED`

其他入口：

- `api.aaccx.pw /health` 返回 200。
- `api.aaccx.pw /dashboard` 返回前端 200。
- `aaccx.pw /dashboard` 返回前端 200。
- `aaccx.pw /health` 返回 yui.web 的 404 页面；这不是模型 API 问题，但如果希望主域名 health 也统一为 200，需要单独加 location。

## 18084 直连验证

直连 `127.0.0.1:18084`：

- 已返回 `400 INVALID_BASE_URL`：`/responses`、`/responses/compact`、`/images/generations`、`/images/edits`、`/backend-api/codex/responses`、`/backend-api/codex/responses/compact`。
- 仍返回前端 HTML 200：`/models`、`/chat/completions`、`/embeddings`。
- `/v1/responses` 与 `/v1/chat/completions` 返回 `401 API_KEY_REQUIRED`，说明正式 `/v1` 路由可达。

## 后续建议

- Nginx 暂不需要继续改；公网入口已经满足裸模型 API 400。
- 后端需要补一个小修：让真实 server router 在前端 SPA fallback 之前拦截 `/models`、`/chat/completions`、`/embeddings`，并用真实 router 测试覆盖这些裸路径。
