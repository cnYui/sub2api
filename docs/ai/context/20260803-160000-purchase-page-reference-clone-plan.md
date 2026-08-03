# /purchase 页面参考实现迁移

时间：2026-08-03 16:00:00（Asia/Tokyo）

## 背景

用户要求将当前项目的 `/purchase` 页面替换为本机 `18080` 服务对应源码 `D:\CodeWorkSpace\sub2api` 中的商店页面设计，并暂时只实现前端一致性。

## 参考依据

- 运行中的 `18080` 服务由 `sub2api-dev` 容器提供，其源码目录为 `D:\CodeWorkSpace\sub2api`。
- 参考页面的核心结构是 `PaymentView.vue`、`PurchaseProductCard.vue` 与 `purchaseProductCard.ts`：余额充值和订阅使用统一的深色商品卡片，选择后进入内联支付确认页。

## 实施边界

- 新增独立的 `PurchaseShopView.vue` 并让 `/purchase` 路由指向它，旧 `PaymentView.vue` 不删除，以便保留和回滚既有实现。
- 复用当前项目现有的结算查询、下单、支付状态和支付方式组件，不修改后端 API 或计费逻辑。
- 当前后端未提供参考实现中的流量包字段，因此不虚构流量包商品；余额充值与订阅商品在样式和布局上与参考实现对齐。

## 验证

通过 TypeScript 检查、组件测试和本地 Vite 页面截图对照 `18080` 源码中的商品卡片尺寸、网格、排版、按钮与深色样式。
