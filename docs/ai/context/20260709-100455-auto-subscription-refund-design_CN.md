# 自动套餐退款设计

时间：2026-07-09 10:04 JST

## 目标

用户在 `/orders` 我的订单页点击退款后，系统直接自动执行退款并取消套餐，不再只创建 `REFUND_REQUESTED` 等待管理员处理。

## 最终口径

- 只退款月度订阅套餐：`order_type=subscription`。
- 流量卡不退款：`order_type=traffic_pack` 直接拒绝。
- 支付宝真实消费退回支付宝：
  - 仅支持 `payment_type=alipay` 且订单原 provider 为 `easypay`/ZPay 的订单。
  - 调用 EasyPay/ZPay `api.php?act=refund`，优先使用 `out_trade_no`，失败再尝试 `trade_no`。
  - 支付宝退款不退 1% 手续费，退款基数用订单 `amount`，不是 `pay_amount`。
- 余额消费退回余额：
  - 仅支持 `payment_type=balance` 的套餐订单。
  - 余额退款退手续费，退款基数用订单 `pay_amount`。
- 退款金额按剩余完整天数计算：
  - `remaining_days = floor((subscription.expires_at - now) / 24h)`。
  - 例如还剩 `24.2` 天按 `24` 天退款。
  - `refund_amount = round_to_0.1(refund_base * remaining_days / subscription_days)`。
  - 例如 29 元 30 天套餐，用 5 天剩 25 天，退 `round_to_0.1(29 * 25 / 30) = 24.2`。
- 退款成功后取消套餐：
  - 调用订阅撤销逻辑，把当前 active subscription 置为取消/失效。
  - 网关退款失败时不取消套餐，避免钱没退但权益已删。

## 页面与 API

- 用户入口仍是 `/orders`，组件 `frontend/src/views/user/UserOrdersView.vue`。
- 用户点击按钮仍调用 `POST /api/v1/payment/orders/:id/refund-request`。
- 该接口语义从“提交退款申请”改为“按规则自动退款”。
- 按钮展示条件仍依赖 `/payment/orders/refund-eligible-providers`，运行态需把 ZPay Alipay 的 `refund_enabled` 与 `allow_user_refund` 打开。

## 事务与失败

- 执行自动退款前先把订单从 `COMPLETED` 锁到 `REFUNDING`，防止重复点击。
- 支付宝退款路径：
  - 先计算退款金额并定位 active subscription。
  - 调用 ZPay 网关退款。
  - 网关成功后撤销套餐，再标记订单 `REFUNDED`/`PARTIALLY_REFUNDED`。
  - 网关失败则标记 `REFUND_FAILED`，不撤销套餐。
- 余额退款路径：
  - 先计算退款金额并定位 active subscription。
  - 给用户余额增加退款金额。
  - 撤销套餐。
  - 标记订单 `REFUNDED`/`PARTIALLY_REFUNDED`。
  - 如果撤销套餐失败，需要扣回已退余额；扣回失败要写 `REFUND_ROLLBACK_FAILED` 审计日志。

## ZPay 文档依据

- ZPay/EasyPay 退款接口使用 `POST {gateway}/api.php?act=refund`。
- 关键参数：`pid`、`key`、`money`、`out_trade_no` 或 `trade_no`。
- 网关地址：`https://zpayz.cn/`。
- 参考文档：https://z-pay.cn/doc.html

## 安全约束

- 商户密钥只写入运行态 `payment_provider_instances.config` 加密字段。
- 不把完整 PID/KEY 写入源码、文档、提交信息或日志。
- 本轮代码只实现规则，不在迁移中硬编码商户凭据。
