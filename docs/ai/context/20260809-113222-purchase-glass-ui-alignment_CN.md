# 购买页半透明 UI 统一改造

## 背景

`/monitor` 的监控卡和 `/model-plaza?embedded=1` 的分组面板已经形成统一的玻璃层级：`rounded-2xl` 外形、半透明背景、`backdrop-blur`、低对比度边框和 `shadow-card`。购买页的商品卡仍使用实体渐变背景和较重的自定义阴影，支付方式、订单摘要与支付状态面板也使用不透明 `card`。

## 决策

- 商品卡改为与监控卡同形的半透明容器，保留原有中文文案、价格计算和点击事件。
- 商品卡的价格与明细放入 `rounded-xl` 的半透明内层，复用监控指标块的层级关系。
- 新增全局 `purchase-panel` 形状，供购买页和支付状态面板共用，避免多个支付状态继续复制不透明卡片样式。
- 支付方式按钮改为 `rounded-xl`、半透明底和 `backdrop-blur-sm`；选中态保留支付宝、微信、Stripe、Airwallex 的品牌色，仅降低底色不透明度。
- 不新增或删除用户可见文案，不修改金额、手续费、支付服务商绑定和订单流程。

## 改动范围

- `frontend/src/components/payment/PurchaseProductCard.vue`
- `frontend/src/components/payment/PaymentMethodSelector.vue`
- `frontend/src/components/payment/PaymentStatusPanel.vue`
- `frontend/src/views/user/PurchaseShopView.vue`
- `frontend/src/style.css`
- `frontend/src/components/payment/__tests__/PurchaseProductCard.spec.ts`

## 验证

- `pnpm exec vitest run src/components/payment/__tests__/PurchaseProductCard.spec.ts`：通过。
- `pnpm exec vue-tsc --noEmit`：通过。
- 内置浏览器已打开本地 `/model-plaza?embedded=1`，确认现有半透明组件形状；`/monitor` 与 `/purchase` 受前端登录守卫保护，本地浏览器当前无登录会话，因此未执行带真实用户数据的购买流程点击验证。
