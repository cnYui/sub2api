# 149/199 元订阅套餐本地实现结果

## 结果

- 已用 `149 元 / 每日 135 USD` 与 `199 元 / 每日 179 USD` 替代上一轮本地未发布的 `198 元 / 每日 179 USD` 方案。
- 当前新增套餐为：
  - `149 元订阅池`，group `codex-pool-135-usd`，基础价 `149.00`，日额度 `135 USD`，`sort_order=149`。
  - `199 元订阅池`，group `codex-pool-179-usd`，基础价 `199.00`，日额度 `179 USD`，`sort_order=199`。
- 前端排序为：A=29，B=39，C=59，D=79，E=99，F=149，G=199。

## 代码改动

- 删除未提交的 `backend/migrations/161_seed_codex_198_subscription_plan.sql`。
- 新增 `backend/migrations/161_seed_codex_149_199_subscription_plans.sql`：
  - 用 `VALUES` 循环幂等创建或更新两档 group 和 plan。
  - 只写 `groups` 和 `subscription_plans`。
  - 不写入 `account_groups`，不绑定上游账号。
- 更新 `backend/migrations/auth_identity_payment_migrations_regression_test.go`：
  - 断言 migration 包含 `codex-pool-135-usd`、`149 元订阅池`、`codex-pool-179-usd`、`199 元订阅池`、`149.00::NUMERIC`、`199.00::NUMERIC`。
  - 断言 migration 不包含 `INSERT INTO account_groups` 或 `UPDATE account_groups`。
- 更新 `frontend/src/views/user/__tests__/PaymentView.spec.ts`：
  - 七档订阅 fixture 覆盖 149/199 两档。
  - 验证“阅读订阅套餐F”对应 `149/135刀`。
  - 验证“阅读订阅套餐G”对应 `199/179刀`。
  - 验证选择 G 档时创建 `amount=199/plan_id=7` 的 subscription 订单。

## TDD 记录

- 后端 RED：`go test -count=1 ./migrations -run TestMigration161SeedsCodex149And199SubscriptionPlansWithoutAccountBinding` 失败于 `161_seed_codex_149_199_subscription_plans.sql` 不存在。
- 后端 GREEN：替换 migration 后，`go test -count=1 ./migrations -run 'TestMigration161|TestMigration15[4-7]'` 通过。
- 前端 RED：`pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts --testNamePattern "149|199|plan G|plan F"` 失败于页面仍显示旧 `198/F`。
- 前端 GREEN：七档 fixture 改为 `149/F` 与 `199/G` 后，目标测试通过。

## 验证

- `cd backend && go test -count=1 ./migrations`：通过。
- `cd frontend && pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts`：通过。
- `git diff --check`：通过。

## 未做事项

- 未构建镜像。
- 未部署公网 18084。
- 未修改公网 DB、Redis、nginx 或 Cloudflare Tunnel。
- 未给公网运行态 `codex-pool-135-usd` 与 `codex-pool-179-usd` 绑定 `cliproxy-local-openai` 上游账号。

## 发布提醒

发布新镜像后，必须单独验收运行态：

- 确认公网 DB 已应用 `161_seed_codex_149_199_subscription_plans.sql`。
- 确认 `149 元订阅池` 与 `199 元订阅池` 为 `for_sale=true`。
- 确认 `codex-pool-135-usd.daily_limit_usd=135`，`codex-pool-179-usd.daily_limit_usd=179`。
- 通过后台/运维路径把 `cliproxy-local-openai` 绑定到两个新 group，并刷新调度快照。
- 用新套餐用户或测试订阅真实请求 `/v1/responses`，确认 `usage_logs.group_id` 落到对应新 group。
