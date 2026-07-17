# 代码冗余治理第二阶段执行计划

## 背景

本轮基于 `main` 提交 `69815e737a2ec17dc445aea5619d2978787c48e1` 创建独立 worktree：

- worktree：`.worktrees/codex-code-redundancy-cleanup-phase2`
- 分支：`codex/code-redundancy-cleanup-phase2`

主工作区存在用户未提交修改和两份未跟踪上下文文档，本轮不读取为待改内容、不带入提交、不做清理。

## 修改边界

只实施四项确定性治理：

1. 账号弹窗唯一化：保留 `frontend/src/components/admin/account/` 为唯一生产实现，合并重复组件中仍有价值的测试模式、SSE 状态和统计字段，删除 `frontend/src/components/account/` 下重复弹窗与导出。
2. 设置响应统一：新增纯 mapper，统一 `GET /admin/settings` 与 `PUT /admin/settings` 响应字段语义，恢复 PUT 响应中的 `web_search_emulation_enabled`，统一 Ops 遮罩。
3. 旧直接计费链删除：移除仅测试可达的 legacy 用量扣费链、best-effort 用量日志写入器、兼容包装和过时测试，保留生产 `PersistUsageFact` durable settlement 入口。
4. 失效入口清理：删除根 `Makefile` 中不存在的 `datamanagementd` 目标和已移除 `tools/secret_scan.py` 目标。

## 不做事项

- 不处理创建/编辑账号大表单共用化。
- 不处理 Gemini 重试状态机统一。
- 不处理 OpenAI Handler 循环抽象。
- 不新增数据库迁移、配置键或 API 字段。
- 不修改 DB、Redis、容器、Nginx、公网链路。
- 不部署、不推送远端。

## 提交组织

计划按以下本地提交组织，便于分阶段回滚：

1. `docs: start code redundancy cleanup phase2`
2. `refactor(frontend): dedupe account modals`
3. `refactor(admin): unify settings response mapping`
4. `refactor(service): remove legacy usage billing path`
5. `chore: remove stale make targets`
6. `docs: finish code redundancy cleanup phase2`

## 验证计划

阶段验证：

- 前端目标 Vitest、typecheck、lint。
- 后端 `internal/handler/admin`、`internal/service`、`internal/repository` 相关测试。
- `rg` 确认重复弹窗、旧计费符号和失效 Makefile 目标已移除。

最终验证：

- `go test -p 1 ./...`
- `go test -tags=unit -p 1 ./...`
- `pnpm run test:run`
- `pnpm run typecheck`
- `pnpm run lint:check`
- `pnpm run build`
- `git diff --check`
- `git ls-files --others --exclude-standard docs/ai/context`

