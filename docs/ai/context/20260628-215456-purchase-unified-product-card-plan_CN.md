# /purchase 订阅与流量卡统一商品卡计划

## 用户要求

- 底部 3 个一次性流量包不能单独一组展示，要和上面的订阅卡放在一起。
- 订阅和一次性流量包必须复用同一个卡片组件。
- 一次性流量包卡片：
  - 标题不再写“有效期 365天”，改成 `5刀流量卡`、`10刀流量卡`、`20刀流量卡`。
  - 原 `可用范围` 字段改成 `刷新时间`。
  - 刷新时间值固定显示 `365天`。
- 订阅卡片：
  - 标题从 `月度订阅-时间 30天` 改成 `阅读订阅套餐A/B/C/D...`。
  - 其他价格、日限额、刷新时间、手续费详情保持不变。
- 修改完成后重启当前 5174 端口。

## 设计

采用一个通用前端组件 `PurchaseProductCard.vue`，接收已经归一化的展示字段：

- `testId`
- `title`
- `priceText`
- `detailRows`
- `buttonText`
- `active`

`PaymentView.vue` 负责把 API 返回的订阅套餐和流量包转换成统一商品列表：

- `subscription` 商品保留原 `selectPlan(plan)`。
- `traffic_pack` 商品保留原 `selectTrafficPack(pack)`。
- 选择页只渲染一个 `purchaseProductGridClass` 网格，所有商品一起排布。
- 如果只有流量包没有订阅，也显示同一个网格；如果两类都没有，才显示空状态。
- 续费弹窗也用同一个 `PurchaseProductCard` 渲染订阅卡，避免再依赖旧订阅卡组件。

保留 `SubscriptionPlanCard.vue` 与 `TrafficPackCard.vue` 的兼容测试成本较高，且会继续表达“两套卡片”。本轮会把测试迁移到通用组件与购买页合并网格上。

## 测试计划

- 新增/更新通用卡片组件测试，覆盖黑色卡片结构、按钮事件、订阅标题、流量卡标题与刷新时间字段。
- 更新 `/purchase` 测试：
  - 断言订阅和流量包都通过同一个 `PurchaseProductCard` 渲染。
  - 断言合并后商品数量包含 5 个订阅 + 3 个流量包。
  - 断言流量包下单仍走 `traffic_pack`，订阅下单仍走 `subscription`。
- 运行 `typecheck`、相关 Vitest、`git diff --check`。

