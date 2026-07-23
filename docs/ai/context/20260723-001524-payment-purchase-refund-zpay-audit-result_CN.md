# 支付购买与退款逻辑只读检查结果

检查时间：2026-07-23 00:15（本机时间）

## 结论

- 当前代码层购买链路正常：用户创建订单后会选择启用的支付实例，保存 provider 快照与实例 ID；支付回调或登录态主动验单会确认金额、商户元数据与 provider 后进入自动履约。
- 当前代码层退款链路正常但受网关真实状态约束：订阅退款会重新计算额度退款报价，锁定订单和权益事实，调用原订单绑定的支付实例退款；网关成功后再撤销对应订单权益段。网关返回失败时订单进入 `REFUND_FAILED`，符合可重试/待人工处理状态机。
- 当前运行态仍使用易支付/zpay：`payment_visible_method_alipay_source=easypay_alipay`，唯一启用实例为 `provider_key=easypay`、`apiBase=https://zpayz.cn`、`payment_mode=popup`，前台展示为“支付宝”，实际跳转到 zpay 托管收银台。
- 当前运行态微信支付未启用：`payment_visible_method_wxpay_enabled=false`。
- 当前运行态退款不是全部成功：已有 `REFUND_FAILED` 主要原因是 zpay 返回“卖家余额不足”，另有历史 DNS 解析失败。这不是代码路径断链，但说明 zpay 商户余额不足时自动退款会失败，需要补足商户余额后重试或人工处理。

## 关键代码事实

- 用户支付路由：`backend/internal/server/routes/payment.go` 注册 `/api/v1/payment/orders`、`/api/v1/payment/orders/verify`、`/api/v1/payment/webhook/easypay`、退款申请与管理端退款接口。
- 购买入口：`backend/internal/service/payment_order.go` 中 `CreateOrder` 只允许用户侧外部支付方式 `alipay`，随后通过 `selectCreateOrderInstance` 选择实例，并在 `invokeProvider` 调用 provider。
- 可见支付方式路由：`backend/internal/service/payment_visible_method_instances.go` 将前台 `alipay` 映射到后台配置的官方支付宝或 EasyPay 实例。
- EasyPay/zpay 实现：`backend/internal/payment/provider/easypay.go` 支持 `CreatePayment`、`QueryOrder`、`VerifyNotification`、`Refund`；`paymentMode=popup` 时生成 `submit.php` 托管收银台 URL。
- 履约入口：`backend/internal/service/payment_fulfillment.go` 中 `HandlePaymentNotification` 校验订单号、金额、provider 和商户元数据；成功后调用订阅/流量卡/余额履约。
- 退款入口：`backend/internal/service/payment_refund.go` 和 `payment_refund_state.go` 中用户退款、管理员退款、网关退款、权益撤销分阶段推进。

## 运行态只读查询摘要

查询对象：`sub2api-postgres-dev`。

- `payment_enabled=true`
- `payment_visible_method_alipay_enabled=true`
- `payment_visible_method_alipay_source=easypay_alipay`
- `payment_visible_method_wxpay_enabled=false`
- `RECHARGE_FEE_RATE=1`
- 启用支付实例：
  - `id=1`
  - `provider_key=easypay`
  - `name=ZPay Alipay`
  - `supported_types=alipay`
  - `payment_mode=popup`
  - `refund_enabled=true`
  - `allow_user_refund=true`
  - `apiBase=https://zpayz.cn`
  - `notifyUrl=https://api.aaccx.pw/api/v1/payment/webhook/easypay`
  - `returnUrl=https://aaccx.pw/payment/result`
- 最近订单显示购买链路有成功完成的 `alipay/easypay` 订阅、余额充值和流量卡订单。
- 退款历史统计：`PARTIALLY_REFUNDED=4`，`REFUND_FAILED=6`。失败原因主要是 `easypay refund failed (HTTP 200): 卖家余额不足`。

## 验证命令

- `go test ./internal/payment/... ./internal/service ./internal/handler/... -run "Payment|Refund|EasyPay|Order|Fulfillment|BalancePay|Webhook"`：通过。
- `pnpm test:run src/api/__tests__/payment.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/PaymentResultView.spec.ts src/views/user/__tests__/paymentRefund.spec.ts src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/PaymentMethodSelector.spec.ts src/components/payment/__tests__/PaymentStatusPanel.spec.ts`：7 个文件、93 个测试通过。
- `node --test scripts/__tests__/configure-zpay-alipay-runtime.test.mjs`：3 个测试通过。

## 风险与建议

- 自动退款依赖 zpay 商户侧余额；若 zpay 余额不足，当前代码会正确失败并保留重试状态，但用户侧会看到退款失败。
- 现有 `PARTIALLY_REFUNDED` 历史订单的 `refund_gateway_status` 仍为 `NOT_STARTED`，应视为旧历史状态或手工处理遗留；不要把它当作当前新退款状态机的标准样例。
- 若要确保“线上可退款”，需要在 zpay 后台补足商户余额后，用失败订单重试退款验证一笔真实成功。
