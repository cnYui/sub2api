# Purchase 套餐标题“月度”文案修改结果

## 结果

`/purchase` 页面订阅套餐卡片标题已从“阅读订阅套餐A/B/...”改为“月度订阅套餐A/B/...”。套餐字母后缀、价格、额度、手续费、购买和续费逻辑均未改变。

## 修改范围

- `frontend/src/views/user/PaymentView.vue`：替换订阅卡片标题前缀。
- `frontend/src/views/user/__tests__/PaymentView.spec.ts`：更新套餐 A、D、F、G 的页面回归期望。
- `frontend/src/components/payment/__tests__/PurchaseProductCard.spec.ts`：更新共享卡片测试夹具和期望。

## TDD 记录

1. 修改测试期望后运行目标测试，`PaymentView.spec.ts` 有 5 项按预期失败，实际输出仍为“阅读订阅套餐A/F”等旧文案。
2. 仅修改生产标题前缀后重跑，2 个测试文件、35 个测试全部通过。

## 验证

- `pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/PurchaseProductCard.spec.ts`：35/35 通过。
- `pnpm typecheck`：通过。
- `git diff --check`：通过。
- 目标前端文件搜索确认仅存在“月度订阅套餐”，不再存在“阅读订阅套餐”。

## 边界

未修改后端、数据库、支付流程、套餐配置或协议中的“阅读”；未构建镜像，未部署运行态环境。
