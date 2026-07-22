# 20260722 前端购买 / 订阅页周额度文案修复结果

## 范围

- `/purchase`：订阅卡片文案改为 `周限额`，刷新值改为 `每周刷新`
- `/subscriptions`：公共 Codex 订阅描述不再直出 `group.description`，改为周额度 + 28 天有效期文案
- 同步更新中英文 locale、相关单测

## 实现

- `frontend/src/views/user/PaymentView.vue`
  - 公共 Codex 订阅确认文案改为参数化周描述
  - 保持流量卡逻辑不动
- `frontend/src/views/user/SubscriptionsView.vue`
  - 对公共 Codex 订阅使用统一描述格式
  - 不再透出后端旧 `group.description`
- `frontend/src/i18n/locales/zh.ts`
  - `weeklyLimit -> 周限额`
  - `weeklyRefresh -> 每周刷新`
  - `weeklyDescription -> 每周 {quota}，{days} 天有效期。`
- `frontend/src/i18n/locales/en.ts`
  - 同步英文文案

## 测试

- `pnpm test:run -- frontend/src/views/user/__tests__/PaymentView.spec.ts frontend/src/views/user/__tests__/SubscriptionsView.spec.ts frontend/src/components/payment/__tests__/PurchaseProductCard.spec.ts`
- `pnpm typecheck`
- `pnpm lint:check`
- `pnpm build`

## 运行态

- 已重启本地 `sub2api-dev`
- `http://127.0.0.1:8080/health` 返回 `200`

## 备注

- 当前工作树仍保留上一轮未提交的后端改动和既有上下文文件，未覆盖、未删除。
