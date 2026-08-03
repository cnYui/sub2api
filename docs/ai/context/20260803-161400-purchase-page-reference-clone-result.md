# /purchase 页面参考实现迁移结果

时间：2026-08-03 16:14:00（Asia/Tokyo）

## 已完成

- 新增 `frontend/src/views/user/PurchaseShopView.vue`，并将 `/purchase` 路由从旧 `PaymentView.vue` 切换到新页面。
- 保留旧 `PaymentView.vue`，不删除、不修改，旧商店页面不会再由 `/purchase` 渲染。
- 从 `D:\CodeWorkSpace\sub2api` 的 18080 参考实现复制商品卡片组件及其类型定义：
  - `PurchaseProductCard.vue`
  - `purchaseProductCard.ts`
- 新页面复刻了参考实现的核心前端结构：深色高对比商品卡、380px 最小卡片高度、响应式 1/2/3/4 列网格、圆角全宽操作按钮、内联充值和订阅确认页、支付状态切换动效。
- 复用当前项目的结算信息、支付方式、下单、支付状态和订阅刷新逻辑，没有修改后端接口或计费规则。

## 差异与原因

参考实现支持流量包商品；当前项目的 `CheckoutInfoResponse` 和后端未提供流量包字段。为避免仅为了视觉效果引入无效数据，本次不渲染该类别，余额充值和订阅商品使用与参考实现相同的卡片和网格设计。

## 验证

- `pnpm exec vue-tsc --noEmit` 通过。
- `pnpm vitest run src/components/payment/__tests__/PurchaseProductCard.spec.ts` 通过（1 个测试）。
- 已启动 `http://localhost:5174/purchase` Vite 预览，并通过浏览器确认页面代码可加载。该预览与 `18082` 的登录状态属于不同来源，因此未读取或迁移浏览器存储中的登录令牌；受保护页面将显示登录页，登录后即可检查页面。
