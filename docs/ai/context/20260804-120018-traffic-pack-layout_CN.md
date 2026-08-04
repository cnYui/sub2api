# 2026-08-04 流量卡独立行布局调整记录

## 需求

购买页底部三个流量卡需要与上方余额套餐复用完全相同的卡片网格尺寸，不能因单独使用三列网格而被拉宽；流量卡仍作为独立区块显示在余额套餐之后。

## 实现

- 将购买目录的余额套餐网格类重命名为 `catalogGridClass`。
- 余额套餐和流量卡容器统一绑定 `catalogGridClass`，共享列数、间距和最小行高；列数按两组商品数量的较大值决定，确保三张流量卡在桌面端始终保持独立一行。
- 保留流量卡独立的 `section` 区块，因此三张卡在桌面端位于余额套餐之后的独立一行；移动端继续按响应式规则换行。
- 未改动商品模型、选择流程或支付订单参数。

## 分支

- 从本地 `main` 创建并切换到 `codex/fix-traffic-pack-card-layout`。

## 验证

- `pnpm typecheck`：通过。
- `pnpm exec eslint src/views/user/PurchaseShopView.vue`：通过。
- `pnpm exec vitest run src/components/payment/__tests__/PurchaseProductCard.spec.ts`：通过，1 个测试。
- `git diff --check`：通过；仅有已有工作树文件的换行提示。
- 全量 `pnpm test:run` 仍有工作树既有失败：`HomeView.compact.spec.ts` 1 项、`admin.system.rollback.spec.ts` 2 项，并伴随 GroupsView mock 的未处理错误，与本次购买页布局无关。
