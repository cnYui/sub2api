# 只保留 /v1/* 为正式模型 API 的规范化设计

## 背景

当前公网和后端都支持一批裸 OpenAI/Codex 兼容路径：

- `/responses`
- `/responses/*`
- `/chat/completions`
- `/embeddings`
- `/images/generations`
- `/images/edits`
- `/models`
- `/backend-api/codex/responses`
- `/backend-api/codex/responses/*`

这些路径来自历史兼容：部分 Codex/OpenAI 兼容客户端把 Base URL 配成 `https://api.aaccx.pw` 或 `https://aaccx.pw`，客户端再自行拼出裸 `/responses`。但当前对外规范目标是：模型 API 的正式 Base URL 统一为 `https://api.aaccx.pw/v1`，请求路径统一落在 `/v1/*`。

## 用户确认的目标

- 裸 OpenAI/Codex 路径立刻返回错误，不再兼容转发。
- HTTP 状态使用 `400 Bad Request`。
- Nginx 和后端都要禁用，避免绕过公网 Nginx 直连 `127.0.0.1:18084` 时仍可用。
- `/backend-api/codex/responses` 也一起禁用。
- 只保留 `/v1/*` 为正式模型 API。

## 保留范围

这些路径继续保留：

- `/v1/*`
- `/api/*`
- `/v1beta/*`
- `/antigravity/v1/*`
- `/antigravity/v1beta/*`
- Sub2API 控制台页面路径，例如 `/dashboard`、`/purchase`、`/usage-guide` 等。

## 禁用范围

这些路径在 Nginx 和后端都返回 `400`：

- `GET /models`
- `POST /responses`
- `POST /responses/*`
- `GET /responses`
- `POST /chat/completions`
- `POST /embeddings`
- `POST /images/generations`
- `POST /images/edits`
- `POST /backend-api/codex/responses`
- `POST /backend-api/codex/responses/*`
- `GET /backend-api/codex/responses`

说明：`GET /responses` 和 `GET /backend-api/codex/responses` 当前用于 WebSocket/Responses 兼容入口，也按同样规则禁用。

## 错误响应

建议统一 JSON：

```json
{
  "error": {
    "type": "invalid_request_error",
    "code": "INVALID_BASE_URL",
    "message": "Invalid API path. Use base_url=https://api.aaccx.pw/v1 and the corresponding /v1 endpoint."
  }
}
```

理由：

- `400` 明确表示客户端请求路径配置错误。
- 不使用 `301/302`，避免 POST/SSE/WebSocket 客户端出现方法变化、请求体丢失或不跟随跳转。
- 不使用 `404`，因为用户更需要看到如何修正 Base URL。

## Nginx 设计

### `aaccx-root.conf`

现状：

```nginx
location ~ ^/(responses(/.*)?|backend-api/codex/responses(/.*)?|chat/completions|embeddings|images/(generations|edits))$ {
    proxy_pass http://127.0.0.1:18084;
    ...
}
```

改为直接返回 `400` JSON。新增 `/models` 裸路径覆盖：

```nginx
location ~ ^/(models|responses(/.*)?|backend-api/codex/responses(/.*)?|chat/completions|embeddings|images/(generations|edits))$ {
    default_type application/json;
    return 400 '{"error":{"type":"invalid_request_error","code":"INVALID_BASE_URL","message":"Invalid API path. Use base_url=https://api.aaccx.pw/v1 and the corresponding /v1 endpoint."}}';
}
```

### `cliproxy.conf`

`api.aaccx.pw` 当前通用 `location /` 会把裸路径全部代理到 Sub2API。需要在通用 `location /` 之前增加同样的裸路径 `400` 规则，确保公网 `https://api.aaccx.pw/responses` 立刻报错。

## 后端设计

文件：`backend/internal/server/routes/gateway.go`

移除或改写以下裸路由：

- `r.POST("/responses", ...)`
- `r.POST("/responses/*subpath", ...)`
- `r.GET("/responses", ...)`
- `codexDirect := r.Group("/backend-api/codex")` 下的 `/responses` 路由
- `r.POST("/chat/completions", ...)`
- `r.POST("/embeddings", ...)`
- `r.POST("/images/generations", ...)`
- `r.POST("/images/edits", ...)`

新增一个统一的 `invalidBaseURLHandler`，用于裸路径返回 `400`。后端也补 `GET /models` 裸路径返回同样错误，避免直连 `18084/models?client_version=...` 继续变成 SPA/404。

不改 `/v1` group 内的任何正式路由。

## 测试设计

后端单测：

- `/v1/responses` 仍命中 handler，不返回 `400 INVALID_BASE_URL`。
- `/responses` 返回 `400 INVALID_BASE_URL`。
- `/responses/compact` 返回 `400 INVALID_BASE_URL`。
- `/backend-api/codex/responses` 返回 `400 INVALID_BASE_URL`。
- `/chat/completions` 返回 `400 INVALID_BASE_URL`。
- `/embeddings` 返回 `400 INVALID_BASE_URL`。
- `/images/generations` 返回 `400 INVALID_BASE_URL`。
- `/images/edits` 返回 `400 INVALID_BASE_URL`。
- `/models?client_version=...` 返回 `400 INVALID_BASE_URL`。

Nginx 验证：

- `nginx -t`
- `curl -H 'Host: api.aaccx.pw' http://127.0.0.1:8080/responses` 返回 `400`。
- `curl -H 'Host: aaccx.pw' http://127.0.0.1:8080/responses` 返回 `400`。
- `curl -H 'Host: api.aaccx.pw' http://127.0.0.1:8080/v1/responses` 未鉴权时仍走 Sub2API 正式路由。
- `curl http://127.0.0.1:18084/responses` 返回 `400`，证明后端也禁用。

## 不在本次处理

- 不清理 `sub_filter`、`Clear-Site-Data`、静态资源缓存策略。
- 不重构 Nginx `Connection: upgrade`、timeout、buffering。
- 不改前端使用指南文案，除非后续发现仍有裸路径文案。
- 不删除 `/v1beta/*` 或 `/antigravity/*`，因为它们是有前缀的正式兼容 API。

## 风险

- 当前仍有客户端在用裸 `/responses`，改动后会立即收到 `400`。这是用户明确选择的行为。
- 部分客户端可能只显示通用 API 错误，不展示完整 JSON message；用户需要按文档改 Base URL。
- 如果只发布代码不 reload Nginx，公网裸路径仍可能被 Nginx 转发；如果只改 Nginx 不发布代码，直连后端仍可能可用。因此必须两层一起验证。
