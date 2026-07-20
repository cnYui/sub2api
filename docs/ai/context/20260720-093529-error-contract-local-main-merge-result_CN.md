# 错误契约本地 Main 合并结果

## 合并结果

- Sub2API 本地 `main` 已先归档本地集成与 Compose 修复提交 `b2656595`，再以合并提交 `5c9dd9fb` 纳入错误契约功能提交 `fc22ff45`。
- `AGENTS.md` 冲突已手动合并：保留共享 Docker 网络、部署与调查结论，并补充模型 API 上游错误契约已本地合并的状态。
- CLIProxyAPI 本地 `main` 已快进到 `a66ddb3a`，包含账号池全冷却的 429 事实和凭据不可用的 503 内部分类。

## 追踪内容

- 原本未提交的 Redis Compose 启动修复、验证脚本和 11 份本地集成/调查上下文已纳入 `b2656595`。
- 错误契约规范、实现、回归测试和结果文档已纳入 `5c9dd9fb`。

## 验证

- `node deploy/verify-redis-compose-command.mjs` 通过。
- `go test ./internal/domain/errorcontract ./internal/handler ./internal/service` 通过。
- `pnpm build` 通过，包含 TypeScript 检查。
- CLIProxyAPI 合并后 `go test ./sdk/...` 与 `go build ./cmd/server` 通过。

前端构建仍有既有的动态导入、Browserslist 数据库与包体积警告，未阻止构建。此前全量前端测试的 10 个失败位于未改动的用量、图表和分页偏好模块，本次未重复运行全量测试。

## 运行态边界

本次只完成本地 Git 跟踪和合并，未推送、未部署，未改动数据库、Redis、容器、Nginx、Cloudflare Tunnel 或公网流量。
