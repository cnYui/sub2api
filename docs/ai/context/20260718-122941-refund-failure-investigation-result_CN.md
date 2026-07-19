# 当前退款失败排查结果

## 结论

2026-07-18 运行态最近两笔用户自助退款失败不是本地权限、订单状态或供应商配置关闭导致，而是 EasyPay/ZPay 网关明确返回：`卖家余额不足`。

## 证据

- 应用容器：`sub2api-candidate`，公网候选链路仍由 `127.0.0.1:18084` 承接。
- 支付实例 `payment_provider_instances.id=1`：
  - `name=ZPay Alipay`
  - `provider_key=easypay`
  - `enabled=true`
  - `refund_enabled=true`
  - `allow_user_refund=true`
  - `supported_types=alipay`
- 订单 `176`：
  - 用户 `105`
  - 订阅订单，支付宝网关支付，原订单金额 `59.00`，实付 `59.59`
  - 本次应退 `47.20`
  - 当前 `status=REFUND_FAILED`
  - `refund_gateway_status=FAILED`
  - `failed_reason=easypay refund failed (HTTP 200): 卖家余额不足`
  - 2026-07-18 11:11:46 日志：`POST /api/v1/payment/orders/176/refund-request` 返回 500，错误同上
- 订单 `119`：
  - 用户 `57`
  - 订阅订单，支付宝网关支付，原订单金额 `29.00`，实付 `29.29`
  - 本次应退 `14.50`
  - 当前 `status=REFUND_FAILED`
  - `refund_gateway_status=FAILED`
  - `failed_reason=easypay refund failed (HTTP 200): 卖家余额不足`
  - 2026-07-18 11:23:25、11:23:33、11:24:54、11:26:23 多次重试均失败，错误同上

## 代码链路

- 用户退款入口 `PaymentHandler.RequestRefund` 调用 `PaymentService.RequestRefund`。
- `validateUserAutoRefundRequest` 已通过：订单属于当前用户、订阅信息存在、支付类型为支付宝、支付实例允许用户退款。
- `executeUserGatewaySubscriptionRefund` 调用 `gwRefund`，最终进入 `EasyPay.Refund`。
- `EasyPay.Refund` 将 `卖家余额不足` 判定为网关明确未执行退款，服务层把订单置为 `REFUND_FAILED` 和 `refund_gateway_status=FAILED`。

## 次要发现

订单 `119` 连续重试时，`payment_audit_logs` 的唯一索引 `idx_payment_audit_logs_order_action_uniq(order_id, action)` 阻止了重复写入 `REFUND_FAILED` / `REFUND_RETRY_REQUESTED` 审计记录。该问题只导致重复重试的审计日志缺失，不是退款失败根因。

## 处置边界

- 若继续走自动退款，需要先给 ZPay/EasyPay 商户余额补足，再让用户或管理员重试原订单退款。
- 若网关无法补足或不支持原路退款，需要走人工退款，并同步做本地订单、订阅权益和审计的一致性收尾；执行前必须先写运行态修改计划、备份数据库并明确回滚边界。
