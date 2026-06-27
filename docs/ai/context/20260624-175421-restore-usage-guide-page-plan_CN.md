# 恢复 UsageGuide 页面计划

## 目标

- 在当前工作区恢复普通用户 `UsageGuide` 页面。
- 页面内容保留原有教程结构，并同步包含 `29/39/59/99` 套餐的生图说明。

## 范围

- 恢复页面文件 `frontend/src/views/user/UsageGuideView.vue`
- 恢复测试文件 `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
- 恢复路由 `/usage-guide`
- 恢复普通用户侧边栏入口
- 补回 `zh/en` i18n 中 `nav.usageGuide` 与 `usageGuide.*` 文案键

## 验证

- `npx vitest run src/views/user/__tests__/UsageGuideView.spec.ts`
- `npx vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/UsageGuideView.spec.ts`
