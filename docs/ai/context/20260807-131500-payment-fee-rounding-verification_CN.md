# 购买页手续费向上取整补充验证

时间：2026-08-07 13:15:00（Asia/Tokyo）

新增小数标价场景：标价 ¥29.99、手续费 1% 时，前端手续费为 ¥0.30，实付 ¥30.29，与服务端按分向上取整规则一致。

验证命令：`pnpm vitest run src/components/payment/__tests__/paymentAmount.spec.ts`，4/4 通过；`git diff --check` 通过。
