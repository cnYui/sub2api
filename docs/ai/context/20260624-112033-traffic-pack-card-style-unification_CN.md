# GPT 流量包卡片样式统一

## 背景

用户反馈 `/purchase` 页面新增的 GPT 流量包卡片与上方订阅卡片视觉不一致，要求统一卡片格式并复用相同圆角按钮风格。

## 处理方案

- 新增 `TrafficPackCard.vue`，按 `SubscriptionPlanCard.vue` 的结构实现：
  - `rounded-lg border overflow-hidden` 卡片外壳。
  - 平台色顶部 `h-1.5` 强调条。
  - 标题 + OpenAI 圆角徽标 + 右侧价格。
  - `rounded-xl py-2.5` 满宽购买按钮。
- `PaymentView.vue` 不再内联维护流量包卡片 HTML，改为渲染 `TrafficPackCard`，避免订阅卡和流量包卡出现两套视觉规则。
- 补充 `TrafficPackCard.spec.ts`，锁定卡片外壳、顶部条、按钮圆角和价格/摘要格式。
- 更新 `PaymentView.spec.ts`，通过组件事件继续覆盖流量包下单流程。

## 验证记录

- 已先运行 `TrafficPackCard.spec.ts`，确认组件不存在时测试失败。
- 实现后运行 `pnpm test:run src/components/payment/__tests__/TrafficPackCard.spec.ts src/views/user/__tests__/PaymentView.spec.ts`，12 个测试通过。
