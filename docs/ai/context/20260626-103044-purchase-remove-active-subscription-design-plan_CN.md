# Purchase 页面移除当前订阅展示设计与计划

## 背景

- 用户反馈 `/purchase` 页面底部的“当前订阅”展示与“我的订阅”页面重复。
- `/purchase` 的核心职责应是选择订阅、购买 GPT 流量包、发起支付订单。
- “我的订阅”页面负责展示订阅状态、有效期、分组等存量订阅信息。

## 设计

采用最小改动：只移除 `frontend/src/views/user/PaymentView.vue` 订阅列表下方的 compact 当前订阅展示区。

保留 `activeSubscriptions` 数据读取和传递，原因：

- `SubscriptionPlanCard` 仍需要根据当前订阅判断按钮文案和续费态。
- 续费弹窗仍需要相同数据。
- 支付成功后刷新订阅状态仍属于购买流程必要反馈。

不修改后端 API、订阅页、订阅 store、支付订单创建逻辑。

## 方案取舍

- 方案 A：只删展示区，保留状态依赖。推荐，风险最低，页面功能仍独立。
- 方案 B：连同 active subscription store 获取一起删除。不可取，会影响套餐卡和续费态。
- 方案 C：把展示区改成跳转入口。仍然保留重复展示倾向，不符合“移除当前订阅展示”的目标。

## 实施计划

1. 新建工作分支 `codex/remove-purchase-active-subscription-20260626`。
2. 先在 `frontend/src/views/user/__tests__/PaymentView.spec.ts` 增加失败测试：当 store 有 active subscription 时，`PaymentView` 不渲染 `payment.activeSubscription` 和订阅分组名，但套餐卡仍接收 `activeSubscriptions`。
3. 运行定向测试，确认新测试因现有展示区失败。
4. 修改 `frontend/src/views/user/PaymentView.vue`：删除底部 active subscriptions 展示模板；移除只为该展示区服务的 `getDaysRemaining` 和平台浅色 badge/accent bar 导入。
5. 运行定向测试确认通过。
6. 写结果上下文，并在 `AGENTS.md` 补一条简短记忆。

## 验证

- `cd frontend && npm run test:run -- src/views/user/__tests__/PaymentView.spec.ts`
