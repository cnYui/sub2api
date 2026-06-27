# 订阅卡片价格与描述精简结果

## 结果

- 购买页订阅卡片的右侧价格改为人民币格式，例如 `¥29元`，不再展示 `$` 和 `/30`。
- 带日限额的月度套餐固定展示精简描述：`月度订阅-时间 30天，日限额 19刀，24点刷新`；39 元、59 元套餐按各自日限额展示。
- 带日限额的月度套餐不再展示倍率、模型 scope、features 等重复信息。
- 首页三张套餐卡片文案同步精简为同样格式。
- 未修改后端接口、套餐真实价格、日限额、订阅有效期、订单创建、支付或计费逻辑。

## 变更文件

- `frontend/src/components/payment/SubscriptionPlanCard.vue`
- `frontend/src/components/payment/__tests__/SubscriptionPlanCard.spec.ts`
- `frontend/src/views/__tests__/HomeView.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

## 验证

- 先运行 `pnpm vitest run src/components/payment/__tests__/SubscriptionPlanCard.spec.ts`，新增用例在旧实现下红灯，失败原因为旧卡片仍输出 `$29 / 30...`。
- 修改后运行 `pnpm vitest run src/components/payment/__tests__/SubscriptionPlanCard.spec.ts`：1 个测试文件通过，3 个测试通过。
- 运行 `pnpm vitest run src/components/payment/__tests__/SubscriptionPlanCard.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/__tests__/HomeView.spec.ts src/__tests__/visualThemeSource.spec.ts`：4 个测试文件通过，15 个测试通过。
- `pnpm build`：构建通过，只有既有 Vite 动态导入、大 chunk 和 Browserslist 数据过期警告。
- `git diff --check`：通过。
- `curl -I http://127.0.0.1:5174/purchase`：返回 `HTTP/1.1 200 OK`。

## 发布状态

- 本地前端预览服务仍可通过 `http://localhost:5174/purchase` 查看。
- 尚未推送代码，尚未重新部署公网版本。
