# 同套餐续费 Review 修复结果

## 背景

完成同套餐续费实现后，代码审查发现两个入口边界需要补齐：

- 外部支付订单在创建后、付款回调前，如果用户通过其他方式获得不同 `group_id` 的 active 订阅，履约阶段仍可能发放原订单套餐。
- `/purchase?group=...` 路由预选会直接设置 `selectedPlan`，绕过前端套餐选择时的跨组阻断。

这两个问题都属于续费/换套餐边界，不涉及退款金额、退款状态机或撤权流程。

## 修复

- 后端在 `fulfillSubscriptionOrderInTx()` 首次发放订阅前复用 `ensureSubscriptionPurchaseAllowed()` 做付款时复验。
- 如果订单已经通过 `SUBSCRIPTION_ASSIGNED` / `SUBSCRIPTION_SUCCESS` 审计或订单 note 证明发放过，则继续按原幂等路径恢复，不因后续状态变化重复拦截。
- 前端 route group 预选改为复用 `refreshAndBlockDifferentActiveSubscription(plan)`：
  - 同组 route 预选仍可进入支付确认或续费弹窗。
  - 跨组 route 预选直接提示先退款后购买，不打开支付方式。

## 新增测试

- `TestExecuteSubscriptionFulfillmentRejectsDifferentActiveSubscriptionAtPaymentTime`
  - 复现用户创建 group 7 外部支付订单后，在付款前获得 group 2 active 订阅。
  - 断言履约返回 `ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND`，订单转 `FAILED`，不创建订阅、不写 `subscription_id`。
- `blocks subscription group route preselect when it would switch plans`
  - 复现用户已有 group 2 active 订阅时访问 `/purchase?tab=subscription&group=3`。
  - 断言前端强制刷新 active 订阅、提示新错误码、不展示支付方式。

## 重新验证

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server

cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/paymentUx.spec.ts
pnpm typecheck
pnpm build

git diff --check
git status --short
```

结果：

- `./internal/service`：通过，`87.915s`。
- `./cmd/server`：通过，`0.543s`。
- 前端目标 Vitest：`45/45` 通过。
- `pnpm typecheck`：退出码 0。
- `pnpm build`：退出码 0，耗时 `44.50s`；仅有既有 Vite chunk / dynamic import / Browserslist 警告。
- `git diff --check`：通过。
- `git status --short`：干净。

## 提交

- `e9410568c fix: recheck subscription switch before fulfillment`
