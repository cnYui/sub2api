# 购买页商品卡自适应验证

## 验证结果

- `pnpm exec vue-tsc --noEmit`：通过。
- `pnpm exec vitest run src/components/payment/__tests__/PurchaseProductCard.spec.ts`：1 个测试文件、1 个测试通过。
- 定向 ESLint：购买卡、支付方式、支付状态、购买页和对应测试通过。
- `pnpm run build`：通过；仅保留项目既有的 Browserslist、动态导入和 chunk 体积提示。
- `git diff --check`：通过。

## 浏览器核对

- 内置浏览器已打开正式 `http://127.0.0.1:3002/purchase`，页面加载真实余额套餐和流量卡数据。
- 默认视口与窄桌面视口下，网格按可用空间自动换列，卡片标题、价格、明细和按钮保持可读。
- 等效手机窄视口下，网格自动降为单列；卡片没有横向溢出或中文逐字换行。
- 视口临时覆盖已调用 `reset()` 恢复默认状态。

## 清理

- 已移除仅用于视觉预览的 `PurchasePreviewView.vue` 和 `/purchase-preview` 路由，不改变正式购买流程。
