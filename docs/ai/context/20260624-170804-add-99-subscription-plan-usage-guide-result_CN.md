# 使用方法页同步 99 元套餐结果

## 本次修改

- 已将用户“使用方法”页生图说明从 `29/39/59 元套餐已支持生图和图生图` 更新为 `29/39/59/99 元套餐已支持生图和图生图`。
- 已同步更新对应前端测试断言。

## 修改文件

- [frontend/src/views/user/UsageGuideView.vue](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/user/UsageGuideView.vue)
- [frontend/src/views/user/__tests__/UsageGuideView.spec.ts](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/user/__tests__/UsageGuideView.spec.ts)
- [docs/ai/context/20260624-164537-add-99-subscription-plan-usage-guide-implementation-plan_CN.md](/Users/wujianxiang/CodeSpace/sub2api/docs/ai/context/20260624-164537-add-99-subscription-plan-usage-guide-implementation-plan_CN.md)

## 验证

- 已执行：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/frontend && npx vitest run src/views/user/__tests__/UsageGuideView.spec.ts
```

- 结果：`4 passed`

- 已执行：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/frontend && npx vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/UsageGuideView.spec.ts
```

- 结果：`2 test files passed`、`16 passed`

## 说明

- 本次只同步“使用方法”页文案，不涉及购买页套餐列表、后台套餐配置、首页文案或支付履约逻辑。
