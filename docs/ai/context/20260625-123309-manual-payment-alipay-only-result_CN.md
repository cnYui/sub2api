# Purchase 手动支付仅保留支付宝结果

## 本次完成

- `frontend/src/components/payment/ManualPaymentDialog.vue`
  - 删除微信支付 tab、支付宝 tab、微信二维码渲染分支。
  - 删除 `activeMethod` 状态和 `tabClass` 切换样式逻辑。
  - 默认直接展示支付宝二维码。
  - 支付宝二维码资源改为 `@/assets/payment/manual-alipay.png`。
- `frontend/src/components/payment/__tests__/ManualPaymentDialog.spec.ts`
  - 测试改为断言只存在支付宝二维码。
  - 断言微信 tab、微信二维码和支付宝 tab 都不再渲染。
- `frontend/src/assets/payment/manual-alipay.png`
  - 使用用户提供的 `/Users/wujianxiang/Downloads/1782348874015.png`。
  - SHA256 一致：`bbf3220c0d983c67532ead6ea92c0cc0f92cae28699b7dced265a7725fa2236b`。

## TDD 记录

1. 先修改 `ManualPaymentDialog.spec.ts`。
2. 运行 `pnpm vitest run src/components/payment/__tests__/ManualPaymentDialog.spec.ts`。
3. 旧实现失败在 `manual-payment-tab-wxpay` 仍存在，符合预期。
4. 修改组件和二维码资源后再次运行同一测试，通过。

## 验证

- `pnpm vitest run src/components/payment/__tests__/ManualPaymentDialog.spec.ts`：通过，2 个测试通过。
- `pnpm typecheck`：通过，`vue-tsc --noEmit` 无错误。
- `git diff --check`：通过，无空白错误。

## 备注

- 未删除旧的 `frontend/src/assets/payment/manual-wxpay.jpg`，避免这次前端页面调整引入无关资产删除；当前组件已不再引用它。
- 未处理当前工作区已有的其它支付接口改动和无关删除状态。
