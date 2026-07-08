# 自动 Key 白名单漏项审计计划

## 背景

- 已修复中间件自动 Key endpoint policy 漏放行 `/v1/models`。
- 用户要求继续检查是否还有白名单漏项。

## 审计范围

- Sub2API 网关实际注册的模型/兼容 API 路由：
  - `/v1/*`
  - `/v1beta/*`
  - `/antigravity/*`
  - 裸路径 `INVALID_BASE_URL` 拦截
- 自动 Key 相关白名单：
  - `backend/internal/server/middleware/effective_group.go`
  - `backend/internal/service/effective_group_resolver.go`

## 初步发现

- 中间件 policy 已覆盖当前 `/v1` 组所有已注册正式网关路径。
- service 层 `isFormalOpenAIRequestPath()` 仍有一份重复路径判断，漏了 `/v1/models`。
- 当前请求链路经中间件会传入 `forcePlatform=openai`，因此公网 `/v1/models` 的直接故障已由中间件修复覆盖；但 resolver 直接路径推断仍应补齐，避免后续复用时再次出现同类问题。

## 实施

1. 在 `effective_group_resolver_test.go` 中新增 `/v1/models` 路径推断用例，先确认 RED。
2. 在 `effective_group_resolver.go` 的 `isFormalOpenAIRequestPath()` 中新增 `/v1/models`。
3. 运行 service/middleware/routes 目标测试与 `git diff --check`。
4. 新建结果文档并更新 `AGENTS.md`。
