# 79 元套餐基础价修正设计

## 背景

用户截图显示 79 元套餐在 `/purchase` 被展示为 `¥80.59元`。根因是 `subscription_plans.price` 已经被 seed 成 `79.79`，而用户购买页又根据运行态 `recharge_fee_rate=1` 对套餐价追加 1% 手续费，形成二次加费。

## 价格语义

- `subscription_plans.price` 存套餐基础价，不含支付手续费。
- `79 元订阅池` 的基础价应为 `79.00` 元。
- 用户 `/purchase` 的最终展示和实际下单实付价由基础价加手续费计算：`79.00 * 1.01 = 79.79`。
- 管理员 `/admin/orders/plans` 展示基础价，避免运营后台把含手续费价格误写回套餐配置。

## 修改范围

- 修正 `backend/migrations/156_seed_codex_79_subscription_plan.sql`，把 79 套餐 seed 价格改为 `79.00`。
- 更新迁移回归测试，要求 156 迁移写入 `price = 79.00`。
- 更新 `/purchase` 测试 fixture 和下单断言，确保 79 套餐创建订阅订单时传基础价 `79`。
- 补充套餐卡片测试，确保基础价 `79` 在 1% 手续费下展示 `¥79.79元` 和 `¥79元 + 1% 手续费`。
- 补充管理员 plans 页面测试，确保后台列表展示人民币基础价 `¥79.00`，不展示美元符号或含手续费价。

## 验证计划

- 先运行相关测试得到 RED，证明当前实现会失败。
- 实现最小修正后运行：
  - `go test -count=1 -tags=unit ./migrations`
  - `npm test -- --run src/components/payment/__tests__/SubscriptionPlanCard.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/admin/orders/__tests__/AdminPaymentPlansView.spec.ts`

