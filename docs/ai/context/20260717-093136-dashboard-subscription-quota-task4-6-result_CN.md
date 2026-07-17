# Dashboard 套餐额度实时展示 Task 4-6 结果

## 背景

本轮继续执行 `docs/ai/context/20260716-155243-dashboard-subscription-quota-realtime-implementation-plan_CN.md` 的 Task 4、5、6，基于 Task 3 已写入的不可变权益周期事实，实现 Dashboard quota 读模型、轻量接口、前端展示和可见页面轮询。

## 已完成

- 后端新增 `UserDashboardQuota` 读模型，API 字段为 `period_mode`、`today_usage_usd`、`today_limit_usd`、`period_usage_usd`、`period_limit_usd`、`period_days`、`period_starts_at`、`period_expires_at`。
- 精确周期模式统一为 `entitlement_period`；历史 active 订阅但没有不可变额度快照时返回 `rolling_30d_legacy`；无有效订阅时返回 `none`，分母为 0。
- `GetUserDashboardStats()` 初始返回中包含必需的 `quota` 块；新增 `GET /api/v1/usage/dashboard/quota` 只返回轻量 quota 投影。
- quota 分子按 `actual_cost` 聚合：优先计入 `usage_facts` 的 `pending`、`settling`、`settled`、`debt`，fact payload 优先读取 `usage_log.ActualCost/usage_log.actual_cost`，再兼容 `effects.actual_cost`，最后补充没有同 `(request_id, api_key_id)` fact 的历史 `usage_logs`，避免结算完成后重复计入。
- 前端用户 Dashboard 消费卡改为“套餐额度”，展示今日和本期/近 30 天的 `使用 / 额度`，不再显示实际/标准价对比；超额不钳制。
- Dashboard 首次加载仍请求完整 stats、图表和最近使用；后续仅在页面可见时每 15 秒请求 `/usage/dashboard/quota`，隐藏时停止轮询，恢复前台或 focus 时立即刷新，并用同一个 in-flight promise 合并连续事件。

## 关键取舍

- 没有为历史订阅伪造权益周期，因为历史数据缺少不可变 `daily_limit_usd` 快照；这类 active 订阅明确降级为 `rolling_30d_legacy`。
- `daily_limit_usd` 为空或无套餐时分母按 0 展示，分子仍统计套餐、余额和流量卡产生的实际消费。
- 本轮不改变实际扣费、流量卡预授权、订阅购买限制、退款金额、退款状态机、生产数据库、Redis、Nginx、容器或部署配置。

## TDD 记录

- 先将后端 API 契约、handler/repository 测试和前端组件/页面测试调整为计划约定的 `entitlement_period`、`today_usage_usd`、`period_usage_usd`，当前实现按预期失败。
- 新增页面可见性测试，确认旧实现恢复前台并连续 focus 会发出 3 条 quota 请求；随后改为 in-flight 合并后通过。
- 新增 fact payload 边界测试，确认只有 `usage_log.actual_cost` 有值、`effects.actual_cost=0` 时旧 SQL 会少算费用；修复后通过。

## 验证

已通过：

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run 'TestUsageLogRepository_GetUserDashboardQuota|TestSubscriptionEntitlementPeriodRepository|TestSubscriptionService_Grant'
GOMAXPROCS=2 go test -p=1 -count=1 ./internal/handler -run 'TestDashboard(StatsIncludesQuota|QuotaReturnsCurrentUserQuota)'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/server -run TestAPIContracts
pnpm --dir frontend exec vitest run src/components/user/dashboard/__tests__/UserDashboardStats.spec.ts src/views/user/__tests__/DashboardView.spec.ts
pnpm --dir frontend typecheck
pnpm --dir frontend build
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
```

`pnpm --dir frontend build` 仍输出既有 Vite 动态导入不会拆 chunk、Browserslist 数据较旧和部分 chunk 超过 500 kB 的警告；构建退出码为 0。

未通过但与本次改动无直接关联：

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository
```

失败集中在未触碰的 repository 套件，表现为测试之间共享历史数据导致数量和唯一约束预期不成立，包括 `TestGroupRepoSuite/TestList`、`TestUserRepoAPIKeyGroupFilterSuite/TestZeroGroupIDNoFilter`、`TestUserRepoSuite` 的列表/排序/admin/email 用例，以及 `TestUserSubscriptionRepoSuite` 的重复 email 创建用例。本轮新增的 quota 目标 integration 已单独通过。

## 未做

- 未提交 commit、未 stage。
- 未部署、未构建镜像、未修改运行态数据库或缓存。
