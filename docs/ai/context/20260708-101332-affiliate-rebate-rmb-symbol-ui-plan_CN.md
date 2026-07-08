# Affiliate 返利金额人民币符号 UI 调整计划

## 背景

用户截图指出 `/affiliate` 页面顶部统计卡片中“可转返利额度”和“历史返利额度”显示为 `US$0.00`，但当前返利金额口径已是人民币，应展示人民币符号。

## 设计

- 不修改后端返利金额、转入余额或计算逻辑。
- 不修改通用 `formatCurrency()` 默认 USD 行为，避免影响 API 用量、账号成本等美元金额展示。
- 在 `AffiliateView.vue` 内增加页面局部 helper：`formatRebateCurrency(value)`，内部调用 `formatCurrency(value, 'CNY')`。
- 将 affiliate 页面所有返利金额展示统一改为人民币：可转返利额度、历史返利额度、冻结额度、邀请明细返利、转入成功提示。

## 实施计划

1. 新增 `AffiliateView.spec.ts`，mock `/user/aff` 返回非零返利金额，先断言页面应包含人民币符号且不包含 `US$`。
2. 运行目标测试确认当前失败，失败原因应为仍显示 `US$`。
3. 修改 `AffiliateView.vue` 的返利金额格式化调用。
4. 运行目标测试和格式检查验证。

## 验证

- `pnpm --dir frontend test:run src/views/user/__tests__/AffiliateView.spec.ts`
- `git diff --check -- frontend/src/views/user/AffiliateView.vue frontend/src/views/user/__tests__/AffiliateView.spec.ts docs/ai/context/20260708-101332-affiliate-rebate-rmb-symbol-ui-plan_CN.md`
