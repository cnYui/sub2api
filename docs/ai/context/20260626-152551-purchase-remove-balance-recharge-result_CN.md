# /purchase 下架余额充值页面结果

## 结论

- `/purchase` 用户页已下架余额充值 UI。
- 页面只保留订阅套餐和 GPT 流量包购买。
- `?tab=recharge` 不再进入余额充值页面，会停留在订阅购买视图。
- 后端余额充值订单能力未删除，用于兼容历史订单、支付恢复和后台运维。

## 前端改动

- `frontend/src/views/user/PaymentView.vue`
  - 移除 tab 切换。
  - 移除充值账户卡片、快捷金额、自定义金额、余额充值支付方式和余额充值按钮。
  - 保留订阅套餐、GPT 流量包、支付中状态、支付结果恢复和订阅续费 group 参数处理。
- `frontend/src/i18n/locales/zh.ts`
  - 用户入口从“充值/订阅”改为“购买订阅”。
  - 支付结果返回按钮从“返回充值”改为“返回购买”。
- `frontend/src/i18n/locales/en.ts`
  - 同步英文文案为 `Buy Subscription` 和 `Back to Purchase`。

## 测试

- `PaymentView.spec.ts` 新增覆盖：
  - 默认 `/purchase` 不再展示充值 tab、充值账户和金额输入。
  - `?tab=recharge` 被忽略，仍展示订阅购买视图。

## 验证

- `./node_modules/.bin/vitest run src/views/user/__tests__/PaymentView.spec.ts`
  - 19 个测试通过。
- `./node_modules/.bin/vitest run src/components/payment/__tests__/SubscriptionPlanCard.spec.ts src/components/payment/__tests__/TrafficPackCard.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/router/__tests__/title.spec.ts`
  - 4 个测试文件通过，31 个测试通过。
- `./node_modules/.bin/vue-tsc --noEmit`
  - 通过。

## 注意

- 后台设置中的旧“充值/订阅入口”配置文案本次未改，避免扩大到后台配置模型。
- 用户侧余额充值只是前端下架，不代表数据库、历史订单或后端 API 被删除。
