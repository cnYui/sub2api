# Purchase 页面移除当前订阅展示结果

## 变更

- 已在分支 `codex/remove-purchase-active-subscription-20260626` 修改购买页。
- `frontend/src/views/user/PaymentView.vue` 已移除订阅列表底部的“当前订阅”compact 展示区。
- 仍保留 `activeSubscriptions` 传给 `SubscriptionPlanCard`，用于套餐卡续费态和支付后刷新。
- 已移除只服务于该展示区的 `getDaysRemaining`、`platformAccentBarClass`、`platformBadgeLightClass`。

## 测试

- 新增 `PaymentView` 用例：当订阅 store 有有效订阅时，购买页不渲染 `payment.activeSubscription` 和订阅分组名，但套餐卡仍能拿到 `activeSubscriptions`。

## 验证

- `cd frontend && npm run test:run -- src/views/user/__tests__/PaymentView.spec.ts`
- 结果：16 个测试通过。
