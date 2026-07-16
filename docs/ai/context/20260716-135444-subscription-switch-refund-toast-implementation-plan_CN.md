# 跨套餐购买自助退款提示实施计划

## 目标

替换跨套餐购买 toast 的过时管理员退款指引，同时不改变订阅购买或退款行为。

## 实施步骤

1. 在 `frontend/src/views/user/__tests__/paymentUx.spec.ts` 先修改现有语言资源断言：导入英文语言包，并要求 `ACTIVE_SUBSCRIPTION_EXISTS` 与 `ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND` 都返回新的中英文提示。运行该单测，预期因语言资源仍是旧文案而失败。
2. 在 `frontend/src/i18n/locales/zh.ts` 与 `frontend/src/i18n/locales/en.ts` 更新上述两个错误码，两个同语言键保持完全一致，避免兼容路径产生不同指引。
3. 运行语言资源测试和 `PaymentView` 购买页测试，确认新文案断言通过且跨套餐仍被拦截、同套餐续费仍允许。
4. 运行 `pnpm typecheck` 与 `git diff --check`，复核改动仅限语言资源、回归测试和本次上下文记录。
