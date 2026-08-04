# 2026-08-04 流量卡复用统一购买卡片记录

## 变更原因

购买页上方余额套餐使用 `PurchaseProductCard.vue`，此前流量卡另有一套 `TrafficPackCard.vue`，造成卡片圆角、边框、间距、按钮和暗色主题不一致。用户要求流量卡与上方组件完全一致，并复用同一组件。

## 实现

- `PurchaseShopView.vue` 继续使用 `PurchaseProductCard` 渲染余额套餐。
- 流量卡数据转换为相同的 `PurchaseProductCardModel`：
  - 顶部标签：`GPT 流量卡`；
  - 标题：服务端套餐名称；
  - 价格：服务端人民币价格；
  - 明细：可用额度、有效期、余额不足时使用；
  - 操作：立即购买。
- 余额套餐和流量卡统一通过 `selectCatalogProduct` 选择，保留原有 `balance_subscription` 与 `traffic_pack` 订单类型分流，不改变支付后端协议。
- 删除不再使用的 `TrafficPackCard.vue`，避免后续维护两套购买卡片样式。

## 验证

- `frontend/pnpm typecheck` 通过，退出码为 0。
- `src/components/payment/__tests__/PurchaseProductCard.spec.ts` 通过，1 个测试、1 个断言场景通过。
- 已确认前端源码不再引用 `TrafficPackCard`。
