# 自动 Key 白名单漏项审计结果

## 结论

- 已检查 Sub2API 网关实际注册路由与自动 Key 白名单。
- `/v1` 正式网关路由当前已全部覆盖：
  - `/v1/messages`
  - `/v1/messages/count_tokens`
  - `/v1/models`
  - `/v1/usage`
  - `/v1/responses`
  - `/v1/responses/*`
  - `/v1/chat/completions`
  - `/v1/embeddings`
  - `/v1/images/generations`
  - `/v1/images/edits`
- 没发现已注册的 `/v1/audio/*`、`/v1/files`、`/v1/batches`、`/v1/moderations` 等正式网关接口，因此这些不是“白名单漏放行”，而是当前后端未注册。
- 除中间件漏 `/v1/models` 外，又发现 service 层 `isFormalOpenAIRequestPath()` 也有一份重复白名单，同样漏了 `/v1/models`；已一并本地修复。

## 改动

- `backend/internal/server/middleware/effective_group.go`
  - 自动 Key endpoint policy 放行 `/v1/models`。
- `backend/internal/server/middleware/effective_group_test.go`
  - 自动 Key allow 列表新增 `/v1/models`。
- `backend/internal/service/effective_group_resolver.go`
  - resolver 路径推断白名单新增 `/v1/models`。
- `backend/internal/service/effective_group_resolver_test.go`
  - `ResolveEffectiveGroupForRequest()` 新增 `/v1/models` RED/GREEN 覆盖。

## 验证

- 中间件 RED：
  - `go test -count=1 -tags=unit ./internal/server/middleware`
  - `/v1/models` allow 用例失败。
- resolver RED：
  - `go test -count=1 -tags=unit ./internal/service -run TestEffectiveGroupResolver_RequestPathInfersOpenAI`
  - `/v1/models` 路径推断失败，返回 `NO_OPENAI_ENTITLEMENT`。
- GREEN：
  - `go test -count=1 -tags=unit ./internal/service ./internal/server/middleware ./internal/server/routes`
  - 全部通过。

## 运行态影响

- 本轮仍只修改本地代码和文档。
- 未构建镜像、未替换或重启 `sub2api-candidate`、未改数据库、未改 Redis、未改 nginx。
- 公网 `https://api.aaccx.pw/v1/models` 需发布新镜像后生效。
