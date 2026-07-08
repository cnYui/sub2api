# 只保留 /v1/* 为正式 API 的下一步操作说明

## 当前状态

- Nginx 生效配置已经修改并 reload：
  - `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`
  - `/opt/homebrew/etc/nginx/servers/cliproxy.conf`
- Nginx 现在会对以下裸路径返回 `400 INVALID_BASE_URL`：
  - `/models`
  - `/responses`
  - `/responses/*`
  - `/chat/completions`
  - `/embeddings`
  - `/images/generations`
  - `/images/edits`
  - `/backend-api/codex/responses`
  - `/backend-api/codex/responses/*`
- 后端源码已经在当前工作区修改，但没有构建镜像、没有替换 `sub2api-candidate` 容器。
- 当前公网 8080 入口已被 Nginx 拦截；直连 `127.0.0.1:18084` 仍取决于正在运行的旧应用镜像。
- 没有执行 `git add`、`git commit`、Docker build 或容器 promote。

## 刚才为什么开始编译

原意是验证后端补丁能在运行镜像对应的基线 `83cf82584` 上独立通过，再考虑构建只包含本次补丁的候选镜像，避免把当前工作区里其它未提交/后续提交一起发布。

问题在执行方式：我同时启动了多组 `go test`，而临时 worktree 没有热缓存，Go 重新编译 `ent` 等大包，造成 CPU/内存压力。这一步应该先征得确认，并且只能串行低并发执行，例如：

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/server/routes
```

以后这类重编译、Docker build、容器替换都应先明确提示影响，再执行。

## 不应该直接提交到 main 的原因

当前 `main` 和工作区包含多个与本任务无关的改动或提交，例如重复订阅防护、支付相关、前端文案和历史上下文文档。直接在 main 提交会把“API 入口规范化”与其它业务改动混在一起，后续审查、回滚、发布都会变得不清晰。

更安全的做法是单独分支：

```bash
git switch -c codex/formal-v1-only-api
```

然后只 stage 本任务相关文件：

```bash
git add \
  backend/internal/server/routes/gateway.go \
  backend/internal/server/routes/gateway_test.go \
  backend/internal/server/middleware/effective_group.go \
  backend/internal/server/middleware/effective_group_test.go \
  backend/internal/service/effective_group_resolver.go \
  backend/internal/service/effective_group_resolver_test.go \
  backend/internal/handler/endpoint.go \
  backend/internal/handler/endpoint_test.go \
  backend/internal/handler/stream_error_event.go \
  backend/internal/handler/stream_error_event_test.go \
  backend/internal/service/openai_gateway_service.go \
  backend/internal/service/openai_gateway_service_test.go \
  docs/ai/context/20260708-200126-formal-v1-only-api-next-steps_CN.md
```

Nginx 配置不在 git 仓库里，不能靠 commit 记录；需要在结果文档中记录最终 `nginx -T` 片段和验证结果。

## 推荐的下一步

推荐先停在 Nginx-only 防线：公网 8080 已经可以拦截裸路径，先不要立刻替换 18084 应用容器。

接下来按这个顺序做：

1. 确认本机没有残留 `go test` / `compile` / `docker build` 进程。
2. 创建单独分支，不直接提交到 main。
3. 只保留并 review 本任务相关代码 diff，确认没有混入支付、订阅、前端等无关改动。
4. 串行低并发跑目标测试：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/server/routes
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/server/middleware -run 'TestDefaultAutomaticKeyEndpointPolicy|TestResolveEffectiveGroup'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run 'TestInboundIsResponses|TestOpenAIHandleStreamingAwareError|TestNormalizeInboundEndpoint|TestGetUpstreamEndpoint|TestResponsesSubpathSuffix'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestEffectiveGroupResolver_RequestPath|TestOpenAIResponsesRequestPathSuffix|TestOpenAIBuildUpstreamRequestPreservesCompactPathForAPIKeyBaseURL'
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
```

5. 测试通过后再提交到 `codex/formal-v1-only-api` 分支。
6. 如果需要后端直连 18084 也立刻返回 400，再从运行镜像基线 `83cf82584` 或已确认分支构建独立 hotfix 镜像；构建和容器替换前必须再次确认。

## 可选方案

- **方案 A：只保留 Nginx 拦截，暂不发后端镜像。** 公网行为先规范，风险最低；直连 18084 仍不是最终状态。
- **方案 B：走代码分支 + PR/审查。** 最规范，适合把后端也做成长期保证。
- **方案 C：紧急 hotfix 镜像。** 只从 `83cf82584` 套本次补丁构建并替换 18084 应用容器；适合必须马上禁用直连 18084 的场景，但需要明确停机/重启窗口。

## 后续执行规则

- 不再并行跑多组 Go 测试。
- 不从 dirty main 直接构建公网镜像。
- 不在未确认的情况下执行 Docker build、容器替换或数据库操作。
- 所有重编译命令默认加 `GOMAXPROCS=2` 和 `go test -p=1`。
