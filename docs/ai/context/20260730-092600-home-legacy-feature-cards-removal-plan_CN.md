# 首页旧套餐展示移除计划

## 背景

默认首页 Hero 下方仍展示三枚功能标签和四张旧套餐卡。该区域与新的完整价格清单重复，且不再符合当前首页内容。

## 范围

- 删除 `HomeView.vue` 中的三枚功能标签与四张旧套餐卡。
- 删除仅被该区域使用的中英文翻译项。
- 更新首页回归测试，防止旧区域恢复。

## 验证

- 运行 `pnpm vitest run src/views/__tests__/HomeView.spec.ts`。
- 运行 `pnpm typecheck`。
- 运行 `pnpm build`。
