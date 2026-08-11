# 管理员取消余额套餐权益入口

## 背景

管理员此前只能取消未支付订单，或执行会调用支付服务商的退款。对于已经生效但需要让用户重新购买的余额套餐，这两种操作都不符合需求。

## 实现

- 管理端 `GET /api/v1/admin/payment/orders` 和订单详情为符合条件的订单返回 `can_cancel_balance_package=true`。
- 条件为余额套餐订单，订单状态为 `COMPLETED` 或 `REFUND_FAILED`，并且仍关联未过期的 `active`、`completed` 或 `debt_paused` 套餐。
- 管理员订单页仅在该字段为真时显示“取消套餐”按钮，并在确认框中声明不会退款或修改订单状态。
- 新增 `POST /api/v1/admin/payment/orders/:id/cancel-balance-package`，使用幂等键保护重复提交。
- 服务端在事务内锁定用户、套餐和订单，将套餐改为 `cancelled`，把 `remaining_usd` 清零并清除 `next_credit_at`；用户普通余额、订单状态、退款金额和支付网关均不变。
- 操作写入 `BALANCE_PACKAGE_MANUAL_CANCELLATION` 审计，记录操作者、取消前状态和额度快照，并在提交后失效余额缓存。

## 验证

- `go test ./internal/service -run 'TestCancelPackageByOrderStopsFutureCreditsWithoutRefundOrBalanceAdjustment|TestResumeDebtPausedPackageRequiresNonNegativeBalance' -count=1` 通过。
- `go test ./internal/handler/admin ./internal/server -run 'TestSanitizeAdminPaymentOrderForResponseAddsCurrency|TestAPIContract' -count=1` 通过。
- `go test ./internal/server -count=1` 通过。
- `pnpm exec vue-tsc --noEmit` 通过。
- `pnpm run build` 通过，仅保留既有 Vite 分包体积与动态导入提示。
- `git diff --check` 通过。

## 发布边界

本次仅完成工作区实现和本地验证，未构建镜像、未替换生产容器，也未改变已有的用户套餐或支付订单。
