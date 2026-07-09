# 当前退款设置只读核查

时间：2026-07-09 07:33 JST

## 结论

- 当前公网候选环境 `sub2api-candidate` 的支付宝收款不是官方支付宝直连实例，而是 `easypay` provider instance：`ZPay Alipay`。
- 当前唯一支付实例配置为：
  - `provider_key=easypay`
  - `supported_types=alipay`
  - `enabled=true`
  - `payment_mode=popup`
  - `refund_enabled=false`
  - `allow_user_refund=false`
- 因此当前运行态不能通过 Sub2API 后台发起支付宝/ZPay 退款；管理员点击退款会先被 `PrepareRefund()` 拦截为 `REFUND_DISABLED`，用户侧也不会出现可自助退款的 provider。
- 当前库中没有任何退款状态订单，也没有 `REFUND_*` 审计流水。

## 代码路径

- 用户自助退款接口：`POST /api/v1/payment/orders/:id/refund-request`。
  - 只允许 `order_type=balance` 的已完成订单。
  - 必须订单原支付实例存在，且 `allow_user_refund=true`。
  - 当前 `ZPay Alipay.allow_user_refund=false`，所以用户侧不会开放。
- 管理员退款接口：`POST /api/v1/admin/payment/orders/:id/refund`。
  - 允许 `COMPLETED`、`REFUND_REQUESTED`、`REFUND_FAILED` 状态。
  - 必须订单绑定明确的历史 `provider_instance_id` 或 provider snapshot。
  - 必须 `refund_enabled=true`。
  - 当前 `ZPay Alipay.refund_enabled=false`，所以管理员退款被拦截。
- 真正退款执行在 `PaymentService.gwRefund()`：
  - 有 `payment_trade_no` 时会创建订单原 provider 实例，并调用 `prov.Refund()`。
  - 无 `payment_trade_no` 时会记录 `REFUND_NO_TRADE_NO` 并跳过网关退款，这对“原路退回”不成立。

## Provider 能力

- 官方支付宝 provider 已实现 `Alipay.Refund()`，调用支付宝 SDK 的 `TradeRefund`，使用 `OutTradeNo`、`RefundAmount`、`RefundReason`、`OutRequestNo`。若运行态改成官方支付宝直连，并开启退款权限，理论上可走支付宝官方退款链路。
- EasyPay/ZPay provider 已实现 `EasyPay.Refund()`，调用 `${apiBase}/api.php?act=refund`，优先传 `out_trade_no`，如果订单不存在再尝试 `trade_no`。这只是委托 ZPay/EasyPay 退款接口；是否属于“支付宝原路退回”取决于 ZPay 平台实际能力和商户通道，不是 Sub2API 官方支付宝直连。

## 外部文档校验

- 支付宝开放平台 `alipay.trade.refund` 文档说明退款规则会把支付款按原路退到买家帐号上；但这是官方支付宝接口语义，不能自动外推到当前 ZPay/EasyPay 聚合通道。
- 官方文档链接：https://opendocs.alipay.com/open/6c0cdd7d_alipay.trade.refund

## 金额口径

- 管理员输入的退款金额上限按订单 `amount` 校验。
- 网关退款金额会用 `calculateGatewayRefundAmount(order.amount, order.pay_amount, refund_amount)` 转成支付渠道金额：
  - 全额退款时退 `pay_amount`，即包含当前 1% 手续费实付金额。
  - 部分退款时按 `pay_amount * refund_amount / amount` 比例计算。

## 运行态只读查询

```sql
SELECT id, provider_key, name, supported_types, enabled, payment_mode,
       refund_enabled, allow_user_refund, sort_order, updated_at
FROM payment_provider_instances
ORDER BY sort_order, id;
```

结果：只有 `id=1 / easypay / ZPay Alipay / alipay / enabled=true / popup / refund_enabled=false / allow_user_refund=false`。

```sql
SELECT status, COUNT(*)
FROM payment_orders
WHERE status LIKE 'REFUND%' OR status='PARTIALLY_REFUNDED'
GROUP BY status;
```

结果：0 行。

```sql
SELECT id, order_id, action, operator, created_at
FROM payment_audit_logs
WHERE action LIKE 'REFUND%'
ORDER BY created_at DESC
LIMIT 20;
```

结果：0 行。

## 判断

当前不能直接说“支持支付宝原路退回”。

- 如果继续使用当前 ZPay/EasyPay 聚合通道：需要先确认 ZPay 商户后台/API 是否支持并允许 `api.php?act=refund`，再开启 `refund_enabled`；即便开启，也应表述为“通过 ZPay/EasyPay 原支付通道退款”，不要承诺“支付宝官方原路退”。
- 如果要明确支付宝官方原路退回：需要新增/切换官方支付宝 `provider_key=alipay` 实例，配置 AppID、私钥、支付宝公钥，并确认支付宝开放平台退款权限、资金余额、风控和证书/密钥模式；然后开启 `refund_enabled`，用小额真实订单验收。

本次未修改代码、未修改运行态 DB、未重启容器、未发起真实退款。
