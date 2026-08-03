# 余额套餐购买与退款实现记录

## 已实现

- 购买页沿用现有 `balance_subscription` 订单流程：创建订单时保存套餐快照，ZPay 回调确认成功后由余额套餐履约服务首次增加用户余额，并创建 `user_balance_packages` 记录；定时任务按套餐周期发放后续额度。
- 用户订单页对余额套餐显示退款入口。打开入口会调用 `GET /api/v1/payment/orders/:id/refund-quote`，展示购买金额、手续费、周期用量、时间比例、额度比例、最终消费比例和预计退款。
- 退款提交时服务端重新查询套餐和 `usage_logs.actual_cost`，不信任前端报价。服务端把订单状态推进到 `REFUND_REQUESTED`，再进入通用 `REFUNDING` 流程。
- 退款服务始终从订单保存的 `provider_instance_id` 恢复原始支付实例。ZPay 易支付优先使用 `out_trade_no`，只有订单没有任何交易标识时才跳过网关调用。
- ZPay 成功后订单为 `REFUNDED` 或 `PARTIALLY_REFUNDED`，套餐变为 `refunded` 并清空 `next_credit_at`，不会继续自动到账；不跨订单扣除用户已有余额。
- 网关失败后余额套餐订单标记 `REFUND_FAILED`，用户可以重新报价并重试。网关待确认保持 `REFUND_PENDING`，由已有查询确认流程最终处理。
- 退款成功和待确认审计日志会保留 `user` 或 `admin` 操作者来源。

## 退款公式

```text
套餐周期总额度 = weekly_credit_usd × refresh_count
时间比例 = clamp((now - starts_at) / (expires_at - starts_at), 0, 1)
额度比例 = clamp(sum(usage_logs.actual_cost) / 套餐周期总额度, 0, 1)
消费比例 = max(时间比例, 额度比例)
退款本金 = max(order.amount × (1 - 消费比例), 0)
```

网关退款金额沿用订单支付金额与购买本金的比例换算，因此包含实际支付币种和支付手续费的既有金额处理。

## 验证

- `go test ./internal/service -run 'Test(RefundTimeRatio|ClampRefundRatio|BalancePackageRefundFormula)$'` 通过。
- `go test ./internal/service ./internal/handler ./internal/server/routes -run '^$'` 编译通过。
- `npm run typecheck` 通过。
- `npm run build` 通过；构建仅产生既有 chunk 大小和动态导入警告。
- 完整 `go test ./internal/service ./internal/handler ./internal/server/...` 中，handler/server 通过，service 包因仓库既有环境/集成测试失败；失败日志与本次退款公式无关。

## 运行注意

- 没有使用或写入用户提供的测试账号密码，也没有提交真实支付或真实退款。
- 已运行的 18082 服务进程不会自动加载本地源码改动；部署前需按项目现有方式重新构建并重启后端/前端。
