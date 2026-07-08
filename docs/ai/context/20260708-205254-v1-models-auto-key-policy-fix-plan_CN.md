# /v1/models 自动 Key 策略修复计划

## 背景

- `3876129758@qq.com` 使用当前 active 自动 Key 请求 `https://api.aaccx.pw/v1/models` 返回 403。
- 认证已识别 Key，错误为 `AUTO_KEY_UNSUPPORTED_ENDPOINT`，说明失败点在 Sub2API 自动 Key endpoint policy。
- 同一 Key 请求 `/v1/responses` 已返回 200，订阅、额度和 CLIProxyAPI 上游不是本问题根因。

## 目标

- 自动 Key 应支持正式 OpenAI 兼容模型列表接口 `/v1/models`。
- 裸 `/models` 继续拒绝，保持“模型 API 只使用 `/v1/*`”的约束。
- 不改计费逻辑、不改 CLIProxyAPI、不改数据库、不重启或替换公网容器。

## 实施步骤

1. 先在 `backend/internal/server/middleware/effective_group_test.go` 的正式 `/v1` 自动 Key allow 列表加入 `/v1/models`，确认目标测试 RED。
2. 在 `backend/internal/server/middleware/effective_group.go` 的 `automaticKeySupportsOpenAIEndpoint()` 中放行 `/v1/models`。
3. 运行 `go test -count=1 -tags=unit ./internal/server/middleware`。
4. 运行 `git diff --check`。
5. 更新本轮结果文档和 `AGENTS.md`，说明本地代码已修复但公网需部署后生效。
