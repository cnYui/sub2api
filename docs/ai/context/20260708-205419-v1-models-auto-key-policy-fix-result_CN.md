# /v1/models 自动 Key 策略修复结果

## 结论

- 已在本地代码修复自动 Key 请求 `/v1/models` 被 Sub2API 中间件拒绝的问题。
- 根因是 `automaticKeySupportsOpenAIEndpoint()` 漏放行正式 OpenAI 兼容路径 `/v1/models`。
- 修复后自动 Key 可解析有效 OpenAI group 并进入模型列表 handler。
- 裸 `/models` 仍不放行，继续符合“模型 API 只使用 `/v1/*`”的约束。
- 本轮未构建镜像、未替换 18084 容器、未重启服务、未改数据库、未改 nginx；公网仍需部署后生效。

## 改动

- `backend/internal/server/middleware/effective_group.go`
  - 在自动 Key OpenAI endpoint policy 中新增 `/v1/models` 放行。
- `backend/internal/server/middleware/effective_group_test.go`
  - 在正式 `/v1` 自动 Key allow 列表中加入 `/v1/models` 回归测试。

## TDD 验证

- RED：
  - 命令：`go test -count=1 -tags=unit ./internal/server/middleware`
  - 结果：`TestDefaultAutomaticKeyEndpointPolicyAllowsOnlyFormalV1OpenAIEndpoints/allow_/v1/models` 失败，`supported=false`。
- GREEN：
  - 命令：`go test -count=1 -tags=unit ./internal/server/middleware`
  - 结果：通过。

## 后续

- 若要让用户当前公网请求恢复，需要构建并只替换 `sub2api-candidate` 应用容器，再用该用户 active Key 脱敏方式验收 `GET https://api.aaccx.pw/v1/models`。
