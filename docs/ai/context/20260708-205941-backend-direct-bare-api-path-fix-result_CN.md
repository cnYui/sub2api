# 后端直连裸模型 API 路径修复结果

> 2026-07-08 20:59 JST。本轮只改后端源码与测试；未构建 Docker、未替换/重启 18084 容器、未改 Nginx/DB/Redis。

## 修复内容

- `backend/internal/web/embed_on.go`
  - 在 embedded frontend 的 `shouldBypassEmbeddedFrontend()` 中补齐裸模型 API path：
    - `/models`
    - `/chat/completions`
    - `/embeddings`
  - 这些请求不再被 SPA fallback 返回 HTML，而会继续进入后面已注册的 `invalidBaseURLHandler`。
- `backend/internal/web/embed_test.go`
  - 在 `TestFrontendServer_Middleware/skips_api_routes` 中新增上述三个裸路径的回归覆盖。

## TDD 记录

先新增测试后运行：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=embed ./internal/web -run 'TestFrontendServer_Middleware/skips_api_routes'
```

红灯符合预期，失败路径为：

- `/models`
- `/chat/completions`
- `/embeddings`

随后修改 `shouldBypassEmbeddedFrontend()` 后，同命令通过。

## 验证

已通过：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=embed ./internal/web -run 'TestFrontendServer_Middleware/skips_api_routes'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/server/routes -run TestGatewayRoutesRejectBareOpenAICompatiblePaths
GOMAXPROCS=2 go test -p=1 -count=1 -tags=embed ./internal/web
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/server/routes
git diff --check
```

## 运行态说明

- 源码已修复，但当前运行中的 `sub2api-candidate:20260708-203429-cc845e468-formal-v1-api` 仍是旧镜像。
- 因本轮按低负载要求未执行 Docker build/redeploy，直连 `127.0.0.1:18084` 的运行态需要后续发布新镜像后才会体现该修复。
- Nginx 8080 公网入口此前已经能把这些裸路径返回 `400 INVALID_BASE_URL`，本修复补的是后端直连防线。
