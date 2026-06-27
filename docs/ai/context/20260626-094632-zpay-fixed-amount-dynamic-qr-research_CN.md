# ZPay 固定金额动态二维码与金额校验结论

## 核心结论

- ZPay 的 EasyPay 兼容 API 可以为每一笔订单创建指定金额的支付请求。
- 用户选择 29 元套餐时，后端应创建本地 29 元订单，并调用 ZPay 时传 `money=29.00`。
- ZPay 返回的 `payurl`、`qrcode` 或 `img` 属于这笔订单的支付入口，正常扫码支付时应按该订单金额收款。
- 这不是“修改同一张二维码金额”，而是“每次购买都创建一笔新的固定金额订单二维码”。
- 即使 ZPay 动态订单已经指定金额，后端仍必须在异步通知或查单结果中校验金额，只有回调金额等于本地订单应付金额时才发放套餐。
- 静态支付宝收款码不适合作为自动发放套餐的依据，因为它通常不能可靠限制用户必须支付指定金额。

## ZPay API 依据

用户提供的 ZPay 文档说明：

- 页面跳转支付接口：`https://zpayz.cn/submit.php`
- API 支付接口：`https://zpayz.cn/mapi.php`
- 两个创建支付请求的接口都包含必填参数：
  - `money`：订单金额，最多保留两位小数
  - `out_trade_no`：商户订单号，每个商品不可重复
  - `name`：商品名称
  - `notify_url`：异步通知地址
  - `return_url`：前端跳转地址
  - `sign` / `sign_type`：签名

因此 29、39、59、99 四种套餐应该分别创建不同金额的 ZPay 订单，而不是复用同一张静态收款码。

示例流程：

```text
用户选择 29 元套餐
-> Sub2API 创建本地订单 out_trade_no=A，pay_amount=29.00，plan_id=29套餐
-> Sub2API 调用 ZPay mapi.php，传 money=29.00
-> ZPay 返回 payurl/qrcode/img
-> 前端展示这笔订单对应的二维码或跳转链接
-> 用户支付
-> ZPay 回调 notify_url，携带 out_trade_no=A、money=29.00、trade_status=TRADE_SUCCESS、sign
-> Sub2API 验签、查本地订单、比对金额
-> 金额一致才发放 29 元套餐
```

## 当前项目代码依据

当前 Sub2API 已经按这个思路实现关键防护：

- `backend/internal/service/payment_order.go`
  - 用户创建订阅订单时，会按套餐价格设置订单金额。
  - `plan != nil` 时，`orderAmount = plan.Price`，`limitAmount = plan.Price`。
  - 创建本地订单时保存 `SetPayAmount(payAmount)`。
  - 调用支付服务商时把计算后的金额传给 provider。

- `backend/internal/payment/provider/easypay.go`
  - EasyPay / ZPay API 支付请求会传 `money: req.Amount`。
  - ZPay 返回的 `qrcode` 和 `img` 会透传给前端展示。

- `backend/internal/service/payment_fulfillment.go`
  - `confirmPayment` 收到回调后，会读取本地订单。
  - 会校验 provider、metadata、回调金额。
  - 如果 `paid` 与本地订单 `o.PayAmount` 差额超过币种容差，会记录 `PAYMENT_AMOUNT_MISMATCH` 并拒绝履约。
  - 只有金额匹配后才进入 `toPaid` 和后续套餐发放。

关键逻辑：

```text
if abs(回调金额 - 本地订单应付金额) > 容差:
    记录 PAYMENT_AMOUNT_MISMATCH
    不标记订单为已支付
    不发放套餐
```

## 金额不匹配时的处理结论

如果用户选择 39 元套餐：

- 实付 29 元：不发放 39 元套餐，订单进入异常处理。
- 实付 59 元：不自动升级到 59 元套餐，也不发放 39 元套餐，订单进入异常处理。
- 实付 39 元：金额匹配，验签和订单状态都通过后，才自动发放 39 元套餐。

不建议根据用户实际付款金额自动改套餐，因为订单号绑定的是用户当时选择的套餐。自动改套餐会让补差、退款、优惠价、重复通知、人工介入等场景变复杂，并增加错发风险。

## 推荐落地方式

正式自动收款发套餐时：

1. 前端套餐按钮只传 `plan_id`，不要让前端自由传套餐金额作为事实源。
2. 后端根据 `plan_id` 读取数据库套餐价格。
3. 后端创建本地订单，保存 `pay_amount` 和 `plan_id`。
4. 后端调用 ZPay，`money` 使用本地订单的 `pay_amount`。
5. 前端展示 ZPay 返回的动态二维码或跳转链接。
6. ZPay 回调后，后端验签并校验 `money == pay_amount`。
7. 金额匹配才发放套餐。
8. 金额不匹配进入异常订单，人工处理退款、补差或重下单。

## 静态收款码的定位

静态支付宝收款码只适合作为临时兜底或人工核对：

- 可以让用户扫码付款。
- 不能作为自动发放套餐的可靠依据。
- 用户可能手动输入 29、39、59、99 或其它金额。
- 如果要自动发放，必须依赖 ZPay 动态订单二维码和回调金额校验。

## 最终判断

ZPay API 可以创建固定金额订单二维码；四种套餐应分别通过 API 动态生成对应金额的支付入口。

但“固定金额二维码”不能替代服务端校验。自动发放套餐的最终依据必须是：

```text
签名正确
+ 支付成功
+ out_trade_no 匹配本地订单
+ 回调金额等于本地订单 pay_amount
+ 订单未重复处理
```

全部满足后，才能把对应套餐加到用户账户上。
