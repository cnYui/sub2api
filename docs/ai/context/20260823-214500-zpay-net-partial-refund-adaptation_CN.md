# ZPay 页面净额部分退款适配

## 背景

用户 `itjiangzengwen@gmail.com`（ID `463`）的订单 `536` 在退款页面计算得到 `1.99 CNY`。该报价已排除订单支付手续费：订单标价 `49.00 CNY`、实际支付 `49.49 CNY`。

旧实现对所有支付服务商都使用 `pay_amount * refund_amount / amount` 计算网关退款额，因此将该笔 ZPay 请求从 `1.99` 放大为 `2.01 CNY`。这与页面展示的“手续费不退”语义不一致。

截图中的 `今日交易额 ¥351.48` 是当日交易额，不是 ZPay 商户可退款余额；商户后台会话未登录，不能据此验证可退款资金。生产日志已证明请求到达 ZPay 并收到 `卖家余额不足` 业务响应。

## 实现

- 新增支付能力接口 `NetPartialRefundAmountProvider`。
- 仅 `apiBase` 主机为 `zpayz.cn` 的 EasyPay 实例启用该能力：部分退款直接提交站内页面计算的 `refund_amount`；完整退款继续提交原实付 `pay_amount`。
- 订单 `536` 下次重试时，ZPay 参数 `money` 将为 `1.99`，不再为 `2.01`。
- 退款成功后，余额套餐会标记为 `refunded`、将 `remaining_usd` 清零、清除 `next_credit_at`，从而停止当前套餐后续权益；已计入普通余额的历史消费不在该步骤改写。
- `payment_audit_logs` 的重复动作改为更新最后一次详情与操作者，适配数据库对 `(order_id, action)` 的唯一约束，不再因重复 `REFUND_FAILED` 写入产生干扰日志。

## 验证

- `go test -v ./internal/payment/provider -run 'Test(EasyPayUsesNetPartialRefundAmountOnlyForZPay|EasyPayRefund)' -count=1` 通过。
- `go test -v ./internal/service -tags unit -run 'Test(ResolveGatewayRefundAmountUsesNetAmountForZPayPartialRefund|MarkRefundOKRevokesBalancePackageRemainingCredit|WriteAuditLogUpdatesDuplicateAction|FinishRefundSuccessStatusesFinalize|CalculateGatewayRefundAmountUsesCurrencyPrecision)' -count=1` 通过。
- `git diff --check` 通过。

## 生产重试边界

本次实现未构建、未部署、未重试生产订单。部署后可由用户再次发起订单 `536` 的自助退款；服务端会重新计算页面退款金额并以该金额调用绑定的原 ZPay 商户。若仍返回 `卖家余额不足`，则该错误不再能归因于本地将 `1.99` 放大为 `2.01`，应由 ZPay 商户后台核对可退款卖家余额或该原支付渠道的退款限制。
