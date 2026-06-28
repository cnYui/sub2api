# 79 元套餐基础价修正结果

## 结果

- 已将 `79 元订阅池` 的数据库基础价设计改为 `79.00` 元。
- 用户 `/purchase` 继续复用现有手续费和支付链路：基础价 `79` 叠加运行态 `recharge_fee_rate=1` 后展示与支付 `79.79` 元。
- 管理员 `/admin/orders/plans` 改为展示人民币基础价 `¥79.00`，列文案改为“基础价”，不展示含手续费价。

## 数据库迁移处理

- `156_seed_codex_79_subscription_plan.sql` 的目标 seed 价格已从 `79.79` 修正为 `79.00`。
- 新增 `157_fix_codex_79_subscription_plan_base_price.sql`，用于把已经应用旧 156 的数据库中 `codex-pool-69-usd / 79 元订阅池` 的 `79.79` 修正为 `79.00`。
- 因 156 已在本地 preview 库应用过，已为 156 新旧 checksum 增加兼容白名单，避免下次启动时因已应用迁移文件内容变化触发 checksum 阻断。

## 验证

- `go test -count=1 -tags=unit ./migrations` 通过。
- `go test -count=1 -tags=unit ./internal/repository` 通过。
- `npm test -- --run src/components/payment/__tests__/SubscriptionPlanCard.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/admin/orders/__tests__/AdminPaymentPlansView.spec.ts` 通过，27 个测试通过。
- `npm run typecheck` 通过。

## 运行态说明

本次只修改代码和迁移文件，未直接改写 18084 公网候选库或其他运行态数据库。运行态价格会在包含 157 迁移的新版本启动并应用迁移后修正；如需立即修公网库，需要单独确认执行数据库更新。

