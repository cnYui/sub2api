# 代码冗余治理 P1/P2/P3 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: 使用 test-driven-development 执行；生产代码改动前先写失败测试。步骤使用 checkbox 追踪。

**目标：** 在 `codex/code-redundancy-p0-auto-key` 分支继续完成 P1/P2/P3 冗余治理，优先保证行为等价和可验证。

**架构：** 大范围治理拆成独立切片：后端窗口策略、路由 helper、部署脚本 target、migration 模板、认证公共 helper、handler/gateway 工具、前端组件与 Settings 拆分。每个切片只改自己文件集，先测后改，避免把认证、计费和前端大拆混在一次不可回滚补丁里。

**技术栈：** Go + Gin + testify；Vue 3 + TypeScript + Vitest；Shell；项目上下文文档保存在 `docs/ai/context/`。

---

## 执行顺序

1. P1 usage window policy：新增纯函数包，先替换展示与计费入口最明显重复。
2. P2 OpenAI-only 路由 helper：收敛 `gateway.go` 中 embeddings/images 的重复 gate。
3. P1 部署脚本 target：新增 common/target 基础库，给脚本提供 dry-run 可见目标。
4. P3 套餐 seed 模板：新增生成脚本与 dry-run 测试，不改历史 migration。
5. P1 API Key 认证核心：先抽协议无关校验 helper，让普通与 Google 中间件共享 IP ACL、用户状态和 group 可用性判断。
6. P2/P3 前端组件和 Settings：先抽 API/状态 helper 与测试，不一次性搬空大型 SFC。
7. P3 gateway service 工具层：先抽响应头与上游错误/SSE 判定纯工具，逐步替换调用点。

## 验证命令

- 后端窗口/认证/路由：`cd backend && go test -count=1 -tags=unit ./internal/service ./internal/repository ./internal/handler/quotaview ./internal/server/middleware ./internal/server/routes`
- 前端：`cd frontend && pnpm run typecheck && pnpm vitest run`
- Shell：`bash -n deploy/*.sh deploy/lib/*.sh`
- 文档与空白：`git diff --check`

## TDD 检查点

- 每个新增 helper 必须有单测。
- 每个替换点先有行为锁定测试或已有测试覆盖。
- 失败测试必须先跑出预期失败，再写生产代码。
- 若某个大型拆分无法在当前批次验证等价，先只抽纯 helper，不移动主 SFC 主体。

## 未触碰原则

- 不连接公网 DB/Redis。
- 不重启或替换 18084 应用容器。
- 不修改已发布 migration。
- 不提交密钥、token、SMTP 密码或完整用户 API Key。
