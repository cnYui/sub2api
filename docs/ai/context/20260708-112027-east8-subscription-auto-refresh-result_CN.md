# 东八区订阅日用量自动刷新实现结果

时间：2026-07-08

## 结论

已在本地分支 `codex/east8-subscription-auto-refresh` 完成东八区订阅日用量自动刷新重构代码实现，未提交 git，未部署公网。

核心结果：

- 已删除 `CheckBillingEligibility` 请求准入路径里的 DB 写入式窗口刷新调用；准入只做只读窗口归一化，避免 00:00 后被昨天额度误拒。
- 已新增完成时记账窗口自愈：订阅扣费使用 `usage_logs.created_at` / `UsageBillingCommand.CompletedAt` 作为完成时间，跨零点请求按完成时间计入当天额度。
- 已新增 active 订阅 daily window 后台校准仓储与 scheduler：多实例通过既有 leader lock 模式串行执行，分批推进 stale active 订阅，按今天 `usage_logs.total_cost` 聚合写回，并失效对应订阅 billing cache；单轮结束仍有 stale remaining 时输出 `ALERT` 日志。
- 已修 `/v1/usage` 自动 Key：`DefaultAutomaticKeyEndpointPolicy` 支持 `/v1/usage`，订阅响应块返回前做只读窗口归一化，不把昨天用量当成今天展示。
- 已将 Wire 接线生成到 `backend/cmd/server/wire_gen.go`，server cleanup 会停止 `SubscriptionUsageWindowScheduler`。

## 关键实现边界

- 后台校准只更新 `deleted_at IS NULL`、`status=active`、`expires_at > now` 且 `daily_window_start IS NULL OR daily_window_start < today_start` 的订阅。
- 后台校准不覆盖已经是今天窗口的订阅，避免与完成时扣费并发时丢用量。
- 校准 SQL 使用 `FOR UPDATE SKIP LOCKED` 和 batch size，支持重复执行与多实例分批推进。
- 完成时扣费在订阅窗口 stale 或 NULL 时，将本次费用作为新窗口起点；如果已是当前窗口，则正常累加。
- API 准入路径不再通过 service port 暴露 `RefreshExpiredUsageWindows`，旧私有 API 惰性刷新函数已从 `BillingCacheService` 删除。
- 仓储层历史单条 `RefreshExpiredUsageWindows` 方法仍保留给既有仓储测试覆盖，但不再作为 service 接口能力被请求准入路径调用。

## 测试与修正

新增/调整测试覆盖：

- `UsageBillingCommand.Normalize()` 自动补 `CompletedAt`，且不影响 dedup fingerprint。
- 订阅扣费在 stale daily window 下按完成时间推进到今天，并只记录本次费用。
- 订阅扣费在当前 daily window 下继续累加。
- 后台校准使用今天 `usage_logs` 聚合，忽略昨天和执行上界之后的日志。
- 后台校准不覆盖当前窗口。
- 后台校准尊重 batch size，并返回剩余 stale 数。
- scheduler 非 leader 跳过，leader 执行校准并失效更新订阅的 Redis billing cache。
- stale window 准入不调用 DB 写入式 refresh，且当前窗口真实超限仍拒绝。
- 自动 Key 可访问 `/v1/usage`。
- `/v1/usage` stale daily window 展示归零且不修改原 subscription。

顺手修正全量 integration 测试口径：

- `billing_cache_integration_test.go` 的有效订阅缓存样本补齐 daily/weekly/monthly window 字段。
- `group_repo_integration_test.go` 的 `is_exclusive` 筛选测试不再假设 seed 数据为空，避免被 `traffic-pack-openai` seed 分组污染。

## 验证命令

已通过：

```bash
cd backend && go generate ./cmd/server
cd backend && go test -count=1 -tags=unit ./internal/service ./internal/server/middleware ./internal/handler
cd backend && go test -count=1 -tags=integration ./internal/repository
cd backend && go test -count=1 ./cmd/server
cd frontend && pnpm typecheck
git diff --check
```

说明：

- `go generate ./cmd/server` 曾因 Wire v0.7.0 工具依赖缺少 `github.com/google/subcommands` 的 go.sum entry 失败；已按 Go 提示执行 `go get github.com/google/wire/cmd/wire@v0.7.0`，新增 `github.com/google/subcommands v1.2.0 // indirect` 与对应 `go.sum`。
- `pnpm typecheck` 输出 pnpm 配置迁移 warning，但 `vue-tsc --noEmit` 退出码为 0。

## 未完成事项

- 尚未提交 git。
- 尚未构建 Docker 镜像。
- 尚未部署公网 18084。
- 尚未做公网运行态 stale active 订阅验证；发布时仍需按计划先备份 Postgres/Redis，再替换应用容器并观察 scheduler 日志与 stale 数量。
