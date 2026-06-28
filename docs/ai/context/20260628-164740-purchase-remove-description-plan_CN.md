# /purchase 标题说明删除计划

## 背景

用户要求删除本地 `main` 分支登录后 `/purchase` 页面标题下的小字“通过内嵌页面完成订阅购买”，其他内容保持不变。

## 设计

- 只影响 `/purchase` 页面标题区域，不改支付流程、套餐卡、订单创建、支付方式和后端接口。
- 该小字来自 `/purchase` 路由的 `meta.descriptionKey: 'purchase.description'`，由 `AppHeader` 统一渲染。
- 最小方案是移除 `/purchase` 路由上的 `descriptionKey`，让标题下方说明为空；同时删除中英文未引用的旧 `purchase.description` 文案，避免死文本继续残留。

## 实施计划

1. 在路由测试中增加失败用例：`PurchaseSubscription` 路由不再声明 `descriptionKey`，但仍保留 `titleKey: 'nav.buySubscription'` 和 `requiresPayment: true`。
2. 运行该测试，确认当前代码因仍有 `descriptionKey` 失败。
3. 修改 `frontend/src/router/index.ts`，删除 `/purchase` 路由 `meta.descriptionKey`。
4. 删除中英文 locale 中未引用的 `purchase.description` 旧文案。
5. 运行相关前端测试，确认回归通过。

## 验证

- `pnpm --dir frontend test:run src/router/__tests__/title.spec.ts`
- 如单测命令不可用，使用项目当前可用的 Vitest 等价命令，并记录实际结果。
