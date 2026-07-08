# 后端直连裸模型 API 路径修复计划

> 2026-07-08 20:56 JST。范围：修复直连 `127.0.0.1:18084` 时裸模型 API 被前端 SPA fallback 返回 HTML 200 的问题；不修改 Nginx、不构建 Docker、不重启容器。

## 背景

- Nginx 8080 入口已经对裸 `/models`、`/responses`、`/chat/completions`、`/embeddings`、`/images/*`、`/backend-api/codex/responses*` 返回 `400 INVALID_BASE_URL`。
- 后端 `RegisterGatewayRoutes()` 已注册裸路径 `invalidBaseURLHandler`。
- 生产直连 18084 仍发现 `/models`、`/chat/completions`、`/embeddings` 返回前端 HTML 200。

## 根因

生产镜像带 `embed` tag，`SetupRouter()` 在注册业务路由前先挂载 `frontendServer.Middleware()`。该 middleware 只 bypass 了 `/responses*`、`/images/*`、`/backend-api/*` 等路径，未 bypass `/models`、`/chat/completions`、`/embeddings`，导致它们在到达 `invalidBaseURLHandler` 前被 SPA fallback 吞掉。

## 方案

- 在 `backend/internal/web/embed_on.go` 的 `shouldBypassEmbeddedFrontend()` 中补齐裸模型 API：
  - `/models`
  - `/chat/completions`
  - `/embeddings`
- 不新增新的后端错误格式；继续复用现有 `routes.invalidBaseURLHandler`。
- 不改变 `/dashboard`、`/purchase` 等控制台 SPA fallback。
- 不改变正式 `/v1/*`、`/api/*`、`/v1beta/*`、`/antigravity/*`。

## TDD 验证

1. 修改 `backend/internal/web/embed_test.go`，在 embedded frontend bypass 测试中加入 `/models`、`/chat/completions`、`/embeddings`。
2. 先运行低并发目标测试，预期失败，证明当前 middleware 会拦截这些路径。
3. 修改 `shouldBypassEmbeddedFrontend()`。
4. 重新运行低并发目标测试，预期通过。
5. 运行 gateway 裸路径测试，确认 `INVALID_BASE_URL` 仍保持。
6. 如不构建 Docker，则用本地测试证明代码修复；18084 运行态需后续重新构建发布后才能体现。

## 验证命令

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=embed ./internal/web -run 'TestFrontendServerMiddleware/skips_api_routes'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/server/routes -run TestGatewayRoutesRejectBareOpenAICompatiblePaths
```
