# /purchase 新增 49 元订阅套餐结果

## 结论

- 已在分支 `codex/purchase-249-299-plans` 继续新增 49 元公共 Codex 订阅套餐。
- 49 元周限额按 29 元 `76 USD/周` 到 199 元 `520 USD/周` 的线性曲线外推，结果 `128.235...`，按现有整数 USD 口径取 `128 USD/周`。
- 49 元 28 天周期总额度为 `512 USD`，1% 手续费后应付金额为 `49.49` 元。
- 购买和退款复用现有订阅订单、权益快照、手续费与退款 quote 流程，没有新增独立状态机。

## 代码改动

- 新增 migration `backend/migrations/179_seed_codex_49_subscription_plan.sql`：
  - `codex-pool-128-usd` / `49 元订阅池` / `price=49.00` / `weekly_limit_usd=128`
  - 只 seed `groups` 和 `subscription_plans`，不修改历史订单、历史权益段和 usage facts，不绑定上游账号。
- 后端公共 Codex 周额度映射和 Dashboard 当前套餐周额度查询加入 `codex-pool-128-usd`。
- 前端 `/purchase` 套餐额度映射和展示名加入 49 元档位。
- 测试覆盖 migration、后端额度快照、退款 quote、前端额度映射、购买页周额度展示和 1% 手续费展示。

## 测试

- 先写测试并确认红灯：
  - `go test ./migrations -run TestMigration179SeedsCodex49SubscriptionPlan` 因 migration 文件不存在失败。
  - `pnpm vitest run src/utils/__tests__/subscriptionQuota.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/payableAmount.spec.ts` 因前端缺少 `codex-pool-128-usd` 映射失败。
- 实现后聚焦测试通过：
  - `go test ./internal/service -run "TestPublicCodexWeeklyLimitsIncludeLinearExtendedHighTiers|TestAdminSubscriptionRefundQuoteUsesCodex49TierQuota"`
  - `go test ./migrations -run TestMigration179SeedsCodex49SubscriptionPlan`
  - `pnpm vitest run src/utils/__tests__/subscriptionQuota.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/payableAmount.spec.ts`
- 全量验证通过：
  - `backend`: `go test ./...`
  - `frontend`: `pnpm typecheck`
  - `frontend`: `pnpm lint:check`
  - `frontend`: `pnpm test:run`
  - `frontend`: `pnpm build`
- 前端测试/构建仍存在既有 console error、Vue stub warning、Browserslist 过期提示、Vite chunk/dynamic import 警告和 Node `DEP0190` 提示，退出码均为 0。

## 运行态

- 未执行 Docker、Compose、Nginx 或公网数据库操作。
- 未重启任何服务。
- 未触碰公网当前 `18080` 与 `18086` 端口对应运行态。
