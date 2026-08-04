# `/subscriptions` 展示余额套餐

## 问题

购买页当前使用 `balance_package_plans`，支付成功后写入 `user_balance_packages`；原 `/subscriptions` 页面只读取 `user_subscriptions`，因此购买 ¥29、¥39、¥49、¥299 等余额套餐的用户会看到空页面。

## 实现决策

- 新增认证接口 `GET /api/v1/payment/balance-packages`，只返回当前用户自己的已购/获发余额套餐。
- 服务端按 `user_balance_packages` 的到账进度和生命周期返回数据，并关联 `balance_package_plans` 补充套餐名称、价格和购买页同源的价格档位。
- 服务端根据到期时间将未更新的套餐状态归一为 `expired`；保留 `active`、`completed`、`refunded` 状态供前端展示。
- `/subscriptions` 同时展示余额套餐和历史模型订阅，避免旧订阅数据在迁移后丢失。
- 前端套餐卡显示价格、每周到账额度、到账次数、周期进度、下次到账时间和到期时间；再次购买跳转 `/purchase`。

## 验证

- `frontend/pnpm typecheck`：通过。
- `frontend/pnpm test:run -- --runInBand`：存在既有失败 `src/views/__tests__/HomeView.compact.spec.ts`（期望 `Test site`，实际为 `Genius Programmer Hub`），其余已运行测试未见本次页面相关失败。
- 后端定向测试结果待命令完成后补充。
