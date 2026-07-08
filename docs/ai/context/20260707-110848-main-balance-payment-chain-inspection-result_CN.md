# 本地 main 站内余额、ZPay 回调与余额购买能力排查

## 结论

- 本地 `main` 的站内用户余额是真实现，不是只有展示字段。
- 后端支持创建 `order_type=balance` 的余额充值订单，并支持通过支付宝/微信等支付 provider 完成支付。
- ZPay 在代码里对应 EasyPay 聚合支付实现：`provider/easypay.go` 支持支付宝、微信，`/api/v1/payment/webhook/easypay` 支持 GET/POST 回调。
- ZPay/EasyPay 回调验签、按 `out_trade_no` 找订单、校验 provider/商户元数据和金额后，会把订单置为 `PAID` 并自动履约。
- 余额充值履约不是直接在 webhook handler 里改 `users.balance`，而是创建内部余额兑换码并自动兑换，最终调用 `userRepo.UpdateBalance()` 增加用户余额。
- 用户余额也会在普通 API 计费里被扣减；当没有套餐/流量包兜底时，网关账单路径会记录 `BalanceCost` 并扣 `users.balance`。
- 当前没有实现“使用站内余额购买套餐或流量卡”的用户支付分支。

## 已实现链路

### 余额充值

1. 用户接口 `POST /api/v1/payment/orders` 创建订单。
2. 请求可传 `order_type=balance`，未传 `order_type` 时后端默认 `balance`。
3. `PaymentService.CreateOrder()` 校验支付系统开关、金额范围、每日限额、可用支付渠道。
4. 支付 provider 创建支付单；EasyPay/ZPay 支持 `submit.php` 跳转模式和 `mapi.php` 二维码/API 模式。
5. ZPay/EasyPay 回调到 `/api/v1/payment/webhook/easypay`。
6. `EasyPay.VerifyNotification()` 验签，成功时返回 `PaymentNotification{Status: success, OrderID: out_trade_no, Amount: money}`。
7. `PaymentService.HandlePaymentNotification()` 按 `out_trade_no` 找订单，`confirmPayment()` 校验金额和 provider 后 `toPaid()`。
8. `executeFulfillment()` 识别 `order_type=balance`，执行 `ExecuteBalanceFulfillment()`。
9. `doBalance()` 创建 `RedeemTypeBalance` 内部兑换码并调用 `RedeemService.Redeem()`。
10. `RedeemService.Redeem()` 调 `userRepo.UpdateBalance()`，最终 `users.balance += amount`，正数也累计 `users.total_recharged`。

### 套餐购买

- 后端支持 `order_type=subscription`。
- 支付成功后 `ExecuteSubscriptionFulfillment()` 调 `AssignOrExtendSubscription()` 给用户分配或延长套餐。
- 这是外部支付购买套餐，不是余额购买套餐。

### 流量卡购买

- 后端支持 `order_type=traffic_pack`。
- 支付成功后 `ExecuteTrafficPackFulfillment()` 调 `trafficPackService.CreditPurchase()`。
- `trafficPackRepository.CreditPurchase()` 写 `user_traffic_credits` 和 `traffic_credit_ledger(entry_type='purchase')`。
- 这是外部支付购买流量卡，不是余额购买流量卡。

### API 用量扣余额

- `gateway_service.go` 的账单路径在非套餐、非流量包时使用余额。
- 统一账单命令会写 `BalanceCost`；降级路径会直接调用 `userRepo.DeductBalance()`。
- `userRepo.DeductBalance()` 会扣 `users.balance`；当前注释说明允许本次请求扣到负数，后续请求由认证/计费准入拦截。

## 未实现或当前未暴露的链路

- 当前前端购买页 `PaymentView.vue` 只展示套餐和流量卡商品；当前代码没有普通用户自定义余额充值输入框。
- 前端 `PaymentMethodSelector` 只选择外部支付渠道，如支付宝、微信、Stripe、Airwallex；没有“余额支付”选项。
- 前端创建套餐/流量卡订单时只传：
  - `payment_type=alipay/wxpay/stripe/easypay/airwallex`
  - `order_type=subscription` 或 `traffic_pack`
  - `plan_id` 或 `traffic_pack_id`
- 后端 `CreateOrderRequest.PaymentType` 是必填外部支付类型；没有 `payment_type=balance` 或 `pay_from_balance` 字段。
- 没有发现从 `users.balance` 扣款后直接触发 `AssignOrExtendSubscription()` 或 `CreditPurchase()` 的购买入口。

## 关键代码位置

- 支付订单类型：`backend/internal/payment/types.go`
- 创建订单：`backend/internal/service/payment_order.go`
- 支付回调和履约：`backend/internal/service/payment_fulfillment.go`
- ZPay/EasyPay provider：`backend/internal/payment/provider/easypay.go`
- Webhook 路由与 handler：`backend/internal/server/routes/payment.go`、`backend/internal/handler/payment_webhook_handler.go`
- 余额写入/扣减：`backend/internal/repository/user_repo.go`
- 流量卡入账：`backend/internal/repository/traffic_pack_repo.go`
- 前端购买页：`frontend/src/views/user/PaymentView.vue`
- 前端支付 payload：`frontend/src/components/payment/paymentFlow.ts`

## 判断

- “用户通过支付宝/ZPay 给站内余额充值，回调自动加余额”：后端真实实现存在；当前普通购买页是否暴露自定义余额充值入口需要额外确认运行态 UI，因为源码当前主购买页没有金额输入框。
- “用户用余额支付套餐和流量卡”：当前未实现。若需要，应新增余额购买入口和后端事务：校验余额、扣余额、创建/完成内部订单或审计流水、执行套餐/流量卡履约，并处理幂等与退款策略。
