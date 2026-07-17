# 代码冗余治理第二阶段结果

## 范围

本轮在独立 worktree `.worktrees/codex-code-redundancy-cleanup-phase2`、分支 `codex/code-redundancy-cleanup-phase2` 完成。基于 `main` 提交 `69815e737a2ec17dc445aea5619d2978787c48e1`，未修改主工作区脏文件，未触碰运行态 DB、Redis、容器、Nginx、公网链路，未部署、未推送远端。

## 本地提交

1. `7ec94d06a docs: start code redundancy cleanup phase2`
2. `80c8980e2 refactor(frontend): dedupe account modals`
3. `d39b146ed refactor(admin): unify settings response mapping`
4. `90efd5230 refactor(service): remove legacy usage billing path`
5. `b95c70468 chore: remove stale make targets`

## 完成内容

### 账号弹窗唯一化

- 保留 `frontend/src/components/admin/account/` 为唯一生产弹窗实现位置。
- `AccountTestModal` 合入 OpenAI `default/compact` 测试模式、请求 `mode` 字段和 SSE `status` 展示。
- `AccountStatsModal` 合入今日“账号计费/用户扣费”展示。
- 保留生产 `ReAuthAccountModal` 的原子 OAuth 凭证更新、`extra` 增量保留、服务端清错和返回完整账号语义。
- 删除 `frontend/src/components/account/` 下重复的 `AccountTestModal`、`AccountStatsModal`、`ReAuthAccountModal`、重复测试和重复 barrel exports。

### 设置响应唯一映射

- 新增 `backend/internal/handler/admin/setting_response_mapper.go`，将 `service.SystemSettings`、支付配置和运行态能力统一映射为 `dto.SystemSettings`。
- `GET /admin/settings` 与 `PUT /admin/settings` 保存后响应共用 mapper。
- PUT 响应恢复 `web_search_emulation_enabled`。
- GET/PUT 对 `ops_monitoring_enabled` 统一使用 `opsService.IsMonitoringEnabled()` 遮罩。
- OpenAI fast policy、平台额度和认证来源默认值仍保持原有独立加载逻辑，不改存储格式。

### 删除旧直接计费链

- 删除仅测试可达的 `recordUsageLegacy`、`postUsageBilling`、`applyUsageBilling`、best-effort 用量日志写入器及 legacy quota 计数器。
- 删除 `GatewayService.RecordUsage`、`OpenAIGatewayService.RecordUsage` 兼容包装；生产入口继续使用 `PersistUsageFact`。
- 删除导出包装 `BuildUsageBillingCommand`，包内统一调用 `buildUsageBillingCommand`。
- 将 `postUsageBillingParams` 和 effects 命名改为 durable settlement 语义。
- 测试改为用 `BuildUsageFact` 检查 payload，用 `PersistUsageFact` 检查 fact 持久化。
- 删除旧扣费顺序、best-effort usage log、detached legacy billing context 类过时测试。
- 补充 `UsageFactSettlementService` 成功路径测试，覆盖 apply billing、写 usage log、mark settled、执行 effects。

### 失效入口清理

- 从根 `Makefile` 删除不存在的 `datamanagementd` 构建/测试目标。
- 删除引用已移除 `tools/secret_scan.py` 的 `secret-scan` 目标。
- 未新增替代扫描器，未修改 GitHub Security Scan。

## 验证结果

通过：

- `pnpm exec vitest run src/components/admin/account/__tests__/AccountTestModal.spec.ts src/components/admin/account/__tests__/AccountStatsModal.spec.ts`
- `pnpm run typecheck`
- `pnpm run lint:check`
- `pnpm run build`
- `go test -p 1 -count=1 ./internal/handler/admin`
- `go test -p 1 -count=1 ./internal/handler/admin ./internal/server`
- `go test -p 1 -count=1 ./internal/service`
- `go test -tags=unit -p 1 -count=1 ./internal/service`
- `go test -p 1 -count=1 ./internal/repository`
- `go test -p 1 ./...`
- `go test -tags=unit -p 1 ./...`
- `go test -tags=integration -p 1 -count=1 ./internal/repository -run 'TestUsageBilling|TestUsageFact'`
- `go test -tags=integration -p 1 -count=1 ./internal/repository -run 'TestUserRepoSuite/TestList$'`
- `git diff --check`
- `rg` 确认重复账号弹窗、旧计费符号、失效 Makefile 目标均无命中。

未通过但判断为本阶段外既有问题：

- `go test -tags=integration -p 1 ./internal/repository`
  - 失败集中在 repository integration 全包共享 Postgres container 的测试隔离：部分测试使用 `testEntClient` 真实写库且不会回滚；失败 suite 的清理只删部分表并忽略 DELETE 错误，遇到兄弟测试遗留 FK 数据后列表数量和唯一邮箱断言被污染。
  - 证据：单独运行失败样例 `TestUserRepoSuite/TestList` 通过；本阶段相关的 `TestUsageBilling|TestUsageFact` integration 通过。
- `pnpm run test:run`
  - 失败 10 个，集中在未改动的 `UsageView` / `UsageTable` 历史图片行展示、`ModelDistributionChart` / `GroupDistributionChart` cost 格式化、`usePersistedPageSize` 默认值。
  - 本阶段改动相关的 admin account 弹窗目标测试通过，typecheck/lint/build 通过。

## 后续建议

- 若要让全量仓储 integration 可作为合并门禁，应单独治理 integration 测试隔离：优先统一使用事务回滚或每个 suite 使用独立 schema/container，避免全局 client 测试相互污染。
- 若要恢复前端全量 Vitest，应单独处理 Usage 图片历史行、图表 cost 缺省值和 page size 默认值测试/实现偏差，不混入本轮冗余治理提交。
