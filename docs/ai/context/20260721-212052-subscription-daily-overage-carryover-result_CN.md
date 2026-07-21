# 订阅日额度超额顺延结果

## 结果

- 已实现“日额度削减后超额顺延”：
  - 不改写 `usage_logs` / `usage_facts` 的真实发生时间。
  - 订阅日窗口跨天时按 `carryover = max(旧日窗口用量 - daily_limit_usd × 已跨自然日数, 0)` 写入新日窗口。
  - 超额超过一天额度时会继续占用后续自然日额度，直到抵完。
  - `daily_limit_usd IS NULL` 的无限额订阅不产生 carryover。
  - 管理员手动重置额度仍然强制清零，不承接 carryover。
- 已覆盖三条链路：
  - 请求前热路径：过期日窗口用 carryover 判断是否已超限。
  - 成功结算落库：午夜后第一笔成功请求写入 `carryover + 本次成本`。
  - Dashboard quota：今日额度展示取“今天真实事实用量”和“当前订阅日窗口有效占用”的较大值，因此不造日志也能显示 carryover。
- 已补本地 dev compose 的 `sub2api-cliproxy-local` 外部网络声明，避免重建 `sub2api-dev` 后丢失 CLIProxyAPI 内网链路。

## 本地运行态

- 已在本地备份 PostgreSQL custom dump：
  - `D:\CodeWorkSpace\sub2api\deploy\backups\sub2api-dev-before-172-173-20260721-211541.dump`
  - 已用 `pg_restore -l` 验证可读。
- 已重建本地 `sub2api-dev`，访问地址仍是 `http://127.0.0.1:8080`。
- 本地容器状态：
  - `sub2api-dev` healthy，端口 `127.0.0.1:8080`
  - `sub2api-postgres-dev` healthy
  - `sub2api-redis-dev` healthy
  - `cliproxyapi-local-dev` healthy，端口 `127.0.0.1:8317`
- 已应用本地迁移：
  - `170_reduce_codex_subscription_daily_limits.sql`
  - `171_enable_user_error_request_view_default.sql`
  - `172_backfill_subscription_entitlements_and_usage_windows.sql`
  - `173_backfill_image_usage_cost_breakdown.sql`
- 已清理本地 Redis `billing:sub:*` 订阅缓存；实际为 0 个旧 key。

## 数据核对

- active limited subscriptions：64
- 当前超过新日额度的 active limited subscriptions：2
- 当前总日超额：约 424.1495712 USD
- 超额全部出现在 `codex-pool-19-usd`，新日额度 15 USD。
- `xiaobianfuai@gmail.com` 当前绑定 `codex-pool-local-unlimited`：
  - `daily_limit_usd = NULL`
  - `daily_usage_usd ≈ 39.8084382`
  - 前端应显示“已用 / 不限额”，不应显示 0 上限。
- `subscription_entitlement_periods` active periods：103
- 图片回填按迁移 173 口径已执行：
  - 有图片 token 的 `gpt-image-2` 记录已回填图片分项成本。
  - 缺图片 token 的旧记录不伪造 token，只在可归因时标记 `billing_incomplete`。

## 验证

- `go test -count=1 ./internal/service ./internal/repository ./internal/server`
- `go test -tags=unit -count=1 ./internal/service ./internal/repository`
- `go test -tags=integration -count=1 ./internal/repository -run 'TestUserSubscriptionRepoSuite/TestRefreshExpiredUsageWindows|TestUserSubscriptionRepoSuite/TestCalibrateActiveDailyUsageWindows|TestUsageBillingRepositoryApply_SubscriptionBilling|TestUsageLogRepository_GetUserDashboardQuota'`
- `go test -count=1 ./migrations`
- Docker build 内部前端 `pnpm run build` 通过；仅有既有 chunk size / Browserslist 警告。
- `GET http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`。
- `sub2api-dev` 容器内访问 `https://cliproxyapi:8317/health` 返回 404，但 DNS/TLS/网络连通，404 是目标服务无该路径。

## 说明

- 本轮只改本地开发环境，未触碰公网。
- 没有新增 schema 迁移；carryover 规则直接使用现有 `daily_usage_usd` 和 `daily_window_start`。
- 不建议把超额移动成明天的 usage log，因为会污染真实审计时间线；当前实现是额度债务语义。
