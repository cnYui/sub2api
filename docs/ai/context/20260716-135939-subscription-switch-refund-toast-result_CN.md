# 跨套餐购买自助退款提示实施结果

## 分支

`codex/subscription-switch-refund-toast`，从本地 `main` 的 `9e81aae8e` 创建于独立 worktree。

## 已完成

- 将中文 `ACTIVE_SUBSCRIPTION_EXISTS` 和 `ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND` 统一改为：`仅可续费当前套餐；购买新套餐前，请在“我的订单”按比例退款。`
- 将两个英文兼容键同步为明确的当前套餐续费和 My Orders 按比例退款指引。
- 扩展现有语言资源回归测试，同时锁定两个兼容错误码的中英文文案；购买页既有测试继续验证跨套餐不会打开支付方式或创建订单。

## TDD 与验证

- RED：先将语言资源测试期望改为新文案，`pnpm vitest run src/views/user/__tests__/paymentUx.spec.ts` 因旧中文文案失败，符合预期。
- GREEN：更新语言资源后，同一测试 `10/10` 通过。
- 回归：`pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/paymentUx.spec.ts`，`46/46` 通过。
- 类型检查：`pnpm typecheck` 通过。
- 差异检查：`git diff --check` 通过。

## 非范围

未修改同套餐续费、跨套餐购买拦截、自动退款、比例计算、订单状态机、后端、数据库、Redis、Nginx、容器或运行态；未部署。
