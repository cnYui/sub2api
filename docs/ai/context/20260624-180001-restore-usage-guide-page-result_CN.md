# 恢复 UsageGuide 页面结果

## 结论

- `UsageGuide` 页面不是业务上应该删除，而是当前工作区分支的基线代码里缺少：
  - `frontend/src/views/user/UsageGuideView.vue`
  - `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
  - `/usage-guide` 路由
  - 普通用户侧边栏入口
  - 对应 i18n 文案键
- 同时，页面依赖的 `frontend/src/assets/usage-guide/*.png` 也不在当前分支里。

## 本次恢复内容

- 恢复页面文件 `frontend/src/views/user/UsageGuideView.vue`
- 恢复测试文件 `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
- 恢复 10 张教程截图资源 `frontend/src/assets/usage-guide/*`
- 在 `frontend/src/router/index.ts` 恢复 `/usage-guide` 路由
- 在 `frontend/src/components/layout/AppSidebar.vue` 恢复普通用户“使用方法”入口
- 在 `frontend/src/i18n/locales/zh.ts` / `frontend/src/i18n/locales/en.ts` 补回：
  - `nav.usageGuide`
  - `usageGuide.title`
  - `usageGuide.description`
- 页面中的生图说明同步为 `29/39/59/99 元套餐已支持生图和图生图`

## 验证

- 已执行：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/frontend && npx vitest run src/views/user/__tests__/UsageGuideView.spec.ts
```

- 结果：`1 passed` / `4 passed`

- 已执行：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/frontend && npx vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/UsageGuideView.spec.ts
```

- 结果：`2 test files passed` / `10 passed`

## 备注

- 本次只恢复 UsageGuide 相关页面能力，不处理当前工作区里其他未跟踪的 `.tmp-*`、`deploy/*` 文件。
