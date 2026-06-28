# 79 元订阅套餐上架实施计划

## 目标

在当前本地主分支切出的 `codex/add-79-subscription-plan` 分支上，上架 `79.79 元 / 每日 69 USD / 30 天` 的订阅套餐，并复用现有购买、支付和履约链路。

## 步骤

1. 新增后端迁移回归测试：
   - 在 `backend/migrations/auth_identity_payment_migrations_regression_test.go` 中新增测试。
   - 先断言 `156_seed_codex_79_subscription_plan.sql` 存在且包含目标套餐配置。
   - 先运行测试，确认失败。

2. 新增迁移文件：
   - 创建 `backend/migrations/156_seed_codex_79_subscription_plan.sql`。
   - 参考 155 baseline 的幂等写法，插入或更新 group 与 subscription_plan。
   - 不绑定上游账号。

3. 扩展用户购买页测试：
   - 修改 `frontend/src/views/user/__tests__/PaymentView.spec.ts` 的 fixture，从四个套餐扩展到五个套餐。
   - 增加 79 套餐下单断言，确认仍走 `createOrder` 的订阅订单。

4. 运行验证：
   - `go test -count=1 -tags=unit ./migrations`
   - `npm test -- --run src/views/user/__tests__/PaymentView.spec.ts`
   - 如前端命令因依赖环境差异失败，记录实际输出并做最小修正。

5. 记录结果：
   - 在 `docs/ai/context/` 新增结果文档。
   - 不覆写历史上下文，不记录敏感信息。
