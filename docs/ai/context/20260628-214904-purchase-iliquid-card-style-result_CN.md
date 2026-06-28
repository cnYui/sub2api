# /purchase 购买卡片 iLiquid 风格替换结果

## 改动范围

- `frontend/src/components/payment/SubscriptionPlanCard.vue`
  - 订阅套餐卡片改为参考图黑色商品卡风格：黑底、半透明细边框、顶部亮边、轻微 hover 上浮、白色圆角按钮。
  - 卡片内容改为 `PLAN`、`月度订阅-时间 {天数}`、`Price`、应付价、日/周/月额度、刷新时间和手续费详情。
  - 保留原有 `select` 事件、续费态按钮文案、手续费应付价计算。
- `frontend/src/components/payment/TrafficPackCard.vue`
  - GPT 流量包卡片使用同一套黑色商品卡风格。
  - 内容映射为有效期、应付价、可用额度、可用范围和手续费详情。
- `frontend/src/views/user/PaymentView.vue`
  - 仅在选择态把外层宽度放到 `max-w-7xl`，让桌面端可排 4 列。
  - 选择后的确认/支付态仍保持 `max-w-4xl`。
  - 订阅/流量包网格改为 380px 最小行高、桌面大间距。
- 前端测试同步更新卡片风格断言。

## 未改内容

- 未修改后端、支付 API、订单创建、支付回调、运行态数据库或 nginx。
- 未搬参考页面的 iLiquid 导航、hero、footer，只复刻卡片视觉与卡片内容结构。
- 未改已有脏工作区中的后端/部署文件。

## 验证

- 已先让新卡片测试失败，确认旧卡片样式会被测试抓到。
- `pnpm --dir frontend typecheck` 通过。
- `pnpm --dir frontend test:run src/components/payment/__tests__/SubscriptionPlanCard.spec.ts src/components/payment/__tests__/TrafficPackCard.spec.ts src/views/user/__tests__/PaymentView.spec.ts` 通过：3 个测试文件，29 个测试。
- 本地 dev server 已启动：`http://127.0.0.1:5174/`，代理到 `http://127.0.0.1:18080`。

