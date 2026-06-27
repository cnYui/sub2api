# /purchase 默认订阅页签调整计划

## 背景

用户要求 `/purchase` 页面中「订阅」和「充值」两个组件互换位置，并且打开页面时默认展示订阅内容。

## 修改边界

- 只修改前端页面展示状态与对应前端测试。
- 不修改支付接口、订阅接口、订单创建逻辑、数据库、路由或计费规则。
- 保留现有 `balance_disabled` 配置：余额充值被禁用时仍不展示充值页签。

## 方案

1. 将 `PaymentView.vue` 的默认 `activeTab` 从 `recharge` 改为 `subscription`。
2. 将页签生成顺序改为先展示订阅，再按配置追加充值。
3. 在 `PaymentView.spec.ts` 增加回归测试，覆盖：
   - `/purchase` 默认展示订阅计划卡片；
   - 页签顺序为「订阅」在左、「充值」在右；
   - 默认状态不展示充值账号区域。

## 验证计划

- 运行 `pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts`。
- 运行相关前端回归测试。
- 运行前端构建，确认类型检查和打包不被影响。
- 检查 `/purchase` 本地预览地址可访问。
