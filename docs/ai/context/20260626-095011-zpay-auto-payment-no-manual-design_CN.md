# ZPay 动态订单自动支付与人工支付下线设计

## 背景

- 用户确认：人工支付全部下线；当前 ZPay 已经可以通过 API 实时生成固定金额动态二维码。
- 参考文档：`docs/ai/context/20260626-094632-zpay-fixed-amount-dynamic-qr-research_CN.md`。
- 当前工作区分支：`codex/zpay-payment-20260625`。
- 本地 `main` 已合并：
  - `codex/add-99-plan-20260625`：四档套餐和 99 元 seed。
  - `codex/zpay-payment-20260625`：EasyPay/ZPay `img -> qr_image_url`。
- 实现应基于本地 `main` 新开分支，避免在当前 ZPay 分支漏掉 99 元套餐 migration。

## 目标

- 用户点击 29 / 39 / 59 / 99 元套餐后，系统创建 Sub2API 本地订单，并调用 ZPay/EasyPay API 生成该订单固定金额的动态二维码。
- 前端只展示 ZPay/EasyPay 返回的 `qr_code`、`qr_image_url` 或跳转链接。
- 用户付款后，系统通过 ZPay 回调或主动查单确认支付。
- 后端必须比对 ZPay 返回金额与本地订单 `pay_amount`，金额一致才发放对应套餐。
- 余额充值、订阅、GPT 流量包都不再进入静态二维码或人工兑换码链路。

## 非目标

- 不新增独立 `zpay` provider。ZPay 继续作为 EasyPay 协议实现接入。
- 不把商户 PID、密钥、完整 API Key 写入代码、文档、日志或提交。
- 不支持根据用户实际支付金额自动改套餐。订单选择的是哪个套餐，就只能履约该套餐；金额不匹配进入异常处理。
- 不保留购买页人工收款兜底。ZPay 配置不可用时应明确失败，而不是展示静态码。

## 当前事实

### 已具备的自动支付能力

- `backend/internal/payment/provider/easypay.go`
  - `createAPIPayment` 调用 `mapi.php`。
  - 请求参数包含 `money: req.Amount`。
  - 响应已解析 `payurl`、`qrcode`、`img`，其中 `img` 透传为 `qr_image_url`。
- `backend/internal/service/payment_order.go`
  - 订阅订单根据 `plan_id` 读取后端套餐价格。
  - `plan != nil` 时使用 `plan.Price` 作为订单金额事实源。
  - 本地订单保存 `pay_amount`，再调用 provider 创建支付请求。
- `backend/internal/service/payment_order_lifecycle.go`
  - `VerifyOrderByOutTradeNo` 可主动查上游订单状态，用于补偿 missed webhook。
- `backend/internal/service/payment_fulfillment.go`
  - `confirmPayment` 会校验 provider、metadata 和金额。
  - 金额不匹配会写 `PAYMENT_AMOUNT_MISMATCH`，并拒绝履约。
  - 订阅订单金额匹配后进入 `ExecuteSubscriptionFulfillment`，调用 `AssignOrExtendSubscription`。

结论：后端主链路已经符合“ZPay 固定金额动态二维码 + 金额校验 + 自动发放套餐”的架构，只需要补回归测试并下线人工入口。

### 仍需移除的人工链路

- `frontend/src/views/user/PaymentView.vue`
  - 引入 `ManualPaymentDialog`。
  - `confirmSubscribe()` 在没有支付方式时弹出人工二维码。
  - `confirmTrafficPack()` 在没有支付方式时弹出人工二维码。
  - `goRedeem()` 会跳转 `/redeem`。
- `frontend/src/components/payment/ManualPaymentDialog.vue`
  - 展示静态支付宝收款码。
  - 用户点击“已付款”后引导“前往兑换”。
- `frontend/src/components/payment/__tests__/ManualPaymentDialog.spec.ts`
  - 测试人工支付弹窗。
- `frontend/src/assets/payment/manual-alipay.*`、`frontend/src/assets/payment/manual-wxpay.jpg`
  - 静态人工收款码资源。
- `frontend/src/views/user/__tests__/PaymentView.spec.ts`
  - 当前测试断言无支付方式时打开人工支付弹窗。

这些行为与“人工支付全部下线”冲突。

## 方案对比

### 方案 A：彻底下线购买页人工支付，所有付费商品强制走订单支付

推荐。

- 删除购买页里的 `ManualPaymentDialog` 引用和所有手动支付状态。
- 订阅、流量包、余额充值都必须依赖 `checkout.methods` 中的可用支付方式。
- 无可用支付方式时，确认按钮禁用并展示不可用提示。
- 删除静态人工收款组件、测试和图片资源。
- 保留 `/redeem` 页面本身，不在本次删除兑换码系统，避免影响其它历史兑换码用途。

优点：

- 购买流程只有一个事实源：Sub2API payment order。
- ZPay 订单号、金额、回调和查单可以闭环。
- 用户付款后自动履约，不需要人工发码。

风险：

- ZPay 配置缺失时用户不能购买。这个失败是正确的，因为静态码无法自动履约。

### 方案 B：只对订阅下线人工支付，流量包仍保留人工支付

不推荐。

- 会保留两套付款事实源。
- GPT 流量包仍可能出现“已付款但系统不知道”的问题。
- 与用户确认的“人工支付全部下线”不一致。

### 方案 C：保留人工弹窗，但把静态码替换成 ZPay 动态二维码

不推荐。

- 现有 `PaymentStatusPanel` 已经承担二维码展示、倒计时、轮询、取消和成功态。
- 再保留 `ManualPaymentDialog` 会制造重复 UI 和状态机。
- 容易把“人工已付款按钮”误保留下来。

## 推荐设计

采用方案 A。

### 前端设计

1. `PaymentView.vue`
   - 删除 `<ManualPaymentDialog />` 模板块。
   - 删除 `ManualPaymentDialog` import。
   - 删除 `showManualPaymentDialog`、`manualPaymentItem`、`goRedeem`。
   - `selectPlan`、`selectTrafficPack`、`backToSubscriptionList`、`selectPlanFromModal` 不再重置人工支付状态。
   - 新增 `hasPaymentMethods = computed(() => enabledMethods.value.length > 0)`。
   - `canSubmitSubscription` 和 `canSubmitTrafficPack` 必须要求 `hasPaymentMethods`。
   - `confirmSubscribe` / `confirmTrafficPack` 如果没有支付方式，显示错误提示并返回，不创建订单。
   - 订阅确认态、流量包确认态在无支付方式时展示 `payment.notAvailable`，按钮禁用。

2. 前端测试
   - 新增或改写 `PaymentView.spec.ts`：
     - 有 `alipay` 支付方式时，29 元套餐创建 `order_type=subscription`、`plan_id=1`、`amount=29` 的订单。
     - 有 `alipay` 支付方式时，39 元套餐创建 `order_type=subscription`、`plan_id=2`、`amount=39` 的订单。
     - ZPay 只返回 `qr_image_url` 时进入 `PaymentStatusPanel` 等待态。
     - 无支付方式时订阅和流量包确认按钮禁用，不创建订单，不渲染人工支付弹窗。
     - 四档套餐布局仍保留。
   - 删除 `ManualPaymentDialog.spec.ts`。

3. 前端资源和文案
   - 删除 `ManualPaymentDialog.vue`。
   - 删除静态人工收款图。
   - 删除 `payment.manual` i18n 子树。
   - 保留其它位置的普通 `manual` 文案，例如后台枚举文案，不做无关清理。

### 后端设计

后端业务逻辑原则上不需要改动，只补测试确认关键不变量：

1. EasyPay/ZPay 创建支付请求时传入固定订单金额。
   - 已有 `easypay_create_test.go` 覆盖 `money` 表单字段。
   - 保留并扩展必要断言。
2. ZPay/EasyPay 回调金额小于或大于本地订单 `pay_amount` 时：
   - `HandlePaymentNotification` 返回错误；
   - 订单仍为 `PENDING`；
   - 不调用订阅发放。
3. 回调金额等于本地订单 `pay_amount` 时：
   - 订单完成；
   - 订阅发放一次。

### 运行态配置要求

- ZPay 服务商应在后台配置为 EasyPay provider。
- 可见支付方式应至少启用 `alipay`，可选 `wxpay`。
- EasyPay/ZPay 实例需要 enabled，且 `supported_types` 包含对应可见方法。
- 如果同时启用官方支付宝/微信和 EasyPay/ZPay，需设置前台可见支付来源，保证 `/payment/checkout-info` 返回正确 `methods`。

## 数据流

```text
用户点击 29 元套餐
-> PaymentView 选择 plan_id=1
-> POST /api/v1/payment/orders { order_type=subscription, plan_id=1, payment_type=alipay }
-> PaymentService 根据 plan_id 读取 plan.Price=29
-> 本地订单保存 pay_amount=29.00、out_trade_no、plan_id、subscription_group_id
-> EasyPay provider 调 ZPay mapi.php，传 money=29.00
-> ZPay 返回 payurl/qrcode/img
-> 前端 PaymentStatusPanel 展示动态二维码
-> ZPay notify 或前端 verify 查单返回 paid amount=29.00
-> confirmPayment 验签/校验 provider/校验金额
-> ExecuteSubscriptionFulfillment 发放 plan 绑定的套餐
```

## 错误处理

- 没有支付方式：前端禁用确认按钮并显示不可用提示；不展示人工码。
- ZPay 创建订单失败：沿用现有 `PAYMENT_GATEWAY_ERROR` / provider 配置错误提示。
- ZPay 返回只有 `img`：前端使用 `qr_image_url` 直接展示图片。
- 支付金额不匹配：后端记录 `PAYMENT_AMOUNT_MISMATCH`，订单不履约。
- 重复回调：沿用现有幂等逻辑，已完成订单不重复发放。

## 测试策略

- 前端：
  - `pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/PaymentStatusPanel.spec.ts`
  - `pnpm typecheck`
- 后端：
  - `go test ./internal/payment/provider`
  - `go test -tags=unit ./internal/service -run 'TestConfirmPayment.*Subscription|TestPaymentAmountToleranceForThreeDecimalCurrency'`
- 静态检查：
  - `rg "ManualPaymentDialog|payment\\.manual|manual-alipay|manual-wxpay|showManualPaymentDialog|goRedeem" frontend/src`
  - 预期除历史文档外，前端源码无人工支付购买链路残留。

## 需要同步的长期上下文

实现完成后新建结果文档，并更新 `AGENTS.md`：

- 订阅、余额充值、GPT 流量包购买全部走 ZPay/EasyPay 动态订单。
- 人工静态收款码购买链路已下线。
- 自动履约依据是订单号、签名、provider/merchant metadata 和金额一致。
