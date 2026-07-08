# 套餐 F：198 元订阅本地实现结果

## 结果

- 已在本地代码新增套餐 F：`198 元订阅池`。
- 新分组名：`codex-pool-179-usd`。
- 基础价：`198.00` 元人民币。
- 日额度：`179 USD`。
- 有效期：30 天。
- 排序：`198`，前端购买页会按现有索引规则显示为“阅读订阅套餐F”。

## 代码改动

- 新增 `backend/migrations/161_seed_codex_198_subscription_plan.sql`：
  - 幂等创建或更新 `groups.name='codex-pool-179-usd'`。
  - 幂等创建或更新 `subscription_plans.name='198 元订阅池'`。
  - 不写入 `account_groups`，不绑定上游账号。
- 更新 `backend/migrations/auth_identity_payment_migrations_regression_test.go`：
  - 新增 migration 回归测试，断言 198 元套餐字段与不绑定上游账号约束。
- 更新 `frontend/src/views/user/__tests__/PaymentView.spec.ts`：
  - 新增六档套餐 fixture。
  - 验证 198 元套餐显示为“阅读订阅套餐F”，日限额 `179刀`，价格 `¥198`。
  - 验证选择 F 档后创建 subscription 订单，payload 包含 `amount=198`、`plan_id=6`。

## TDD 记录

- 后端 RED：`go test -count=1 ./migrations -run TestMigration161SeedsCodex198SubscriptionPlanWithoutAccountBinding` 失败于 migration 文件不存在。
- 后端 GREEN：新增 `161_seed_codex_198_subscription_plan.sql` 后，目标迁移测试通过。
- 前端 RED：`pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts --testNamePattern "198|F"` 失败于页面只渲染到套餐 E。
- 前端 GREEN：新增六档 fixture 后，目标测试通过。

## 验证

- `cd backend && go test -count=1 ./migrations`：通过。
- `cd frontend && pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts`：32 个测试通过。
- `git diff --check`：通过。

## 未做事项

- 未构建镜像。
- 未部署公网 18084。
- 未修改公网 DB、Redis、nginx 或 Cloudflare Tunnel。
- 未给公网运行态 `codex-pool-179-usd` 绑定 `cliproxy-local-openai` 上游账号。

## 发布提醒

发布新镜像后，必须单独验收运行态：

- 确认公网 DB 已应用 `161_seed_codex_198_subscription_plan.sql`。
- 确认 `subscription_plans` 中 `198 元订阅池` 为 `for_sale=true`。
- 确认 `groups.name='codex-pool-179-usd'` 存在且 `daily_limit_usd=179`。
- 通过后台/运维路径把 `cliproxy-local-openai` 绑定到新 group，并刷新调度快照。
- 用 F 套餐用户或测试订阅真实请求 `/v1/responses`，确认 `usage_logs.group_id` 落到 `codex-pool-179-usd`。
