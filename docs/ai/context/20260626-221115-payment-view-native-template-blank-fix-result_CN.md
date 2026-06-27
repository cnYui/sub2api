# 购买页主内容空白修复结果

## 结论

`http://127.0.0.1:18084/purchase` 主内容区空白的直接原因是 `PaymentView.vue` 在购买选择态中多包了一层无指令裸 `<template>`。

构建后的页面把订阅套餐和 GPT 流量包放进原生 `<template>` 节点，导致：

- DOM `textContent` 有 29/39/59/99 元订阅和 GPT 流量包；
- 浏览器视觉上不渲染这些内容；
- grid/card 计算宽高为 `0x0`；
- 页面看起来只有侧栏和 header，主区域空白。

## 已修改

- `frontend/src/views/user/PaymentView.vue`
  - 删除购买选择态中的冗余裸 `<template>` wrapper。
  - 保留支付中、流量包确认、订阅确认、套餐列表的既有 `v-if`/`v-else-if`/`v-else` 结构。
- `frontend/src/views/user/__tests__/PaymentView.spec.ts`
  - 新增回归测试：购买页选择态不能出现 `main > div > template`。

## 验证

测试：

```bash
pnpm --dir frontend test:run src/views/user/__tests__/PaymentView.spec.ts -t "does not hide purchasable items inside a native template element"
pnpm --dir frontend test:run src/views/user/__tests__/PaymentView.spec.ts
```

结果：

- 目标测试先失败，失败原因为存在 `main > div > template`。
- 修复后目标测试通过。
- `PaymentView.spec.ts` 全量 20 个测试通过。

候选镜像：

- 新镜像：`sub2api-candidate:20260626-220602-payment-template-30e66c82580f`
- 已只重建 `sub2api-candidate`。
- `sub2api-candidate-postgres`、`sub2api-candidate-redis` 未重建。
- 公网 `sub2api` 未重建。

浏览器验证：

- `http://127.0.0.1:18084/purchase` 中 `main > div > template` 不再存在。
- 页面可见 29/39/59/99 元订阅套餐。
- 页面可见 2/3/5 元 GPT 流量包。
- `127.0.0.1:18084/health` 为 200。
- `127.0.0.1:18080/health` 为 200。

## 注意

候选环境仍关闭真实支付 provider 和可见支付方法，用于避免预演环境触发外部支付副作用。
