# /purchase 标题说明删除结果

## 结果

- 已删除登录后 `/purchase` 页面标题下的小字“通过内嵌页面完成订阅购买”。
- 实现方式是移除 `/purchase` 路由 `meta.descriptionKey`，让 `AppHeader` 不再渲染页面说明。
- 已删除中英文 locale 中未引用的旧 `purchase.description` 文案。
- 未修改支付流程、套餐卡、订单创建、后端接口、运行态 DB 或容器。

## 修改文件

- `frontend/src/router/index.ts`
- `frontend/src/router/__tests__/title.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

## 验证

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/frontend
./node_modules/.bin/vitest run src/router/__tests__/title.spec.ts src/views/user/__tests__/PaymentView.spec.ts
```

结果：2 个测试文件通过，28 个测试通过。

## 注意

- `frontend/pnpm-lock.yaml` 曾因 `pnpm --dir frontend test:run ...` 被 pnpm 触发无关刷新，已恢复为无 diff。
- 当前工作区原有后端中间件修改未触碰。
