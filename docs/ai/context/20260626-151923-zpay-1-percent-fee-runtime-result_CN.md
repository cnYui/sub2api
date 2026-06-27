# ZPay 1% 手续费运行态配置结果

## 结论

- ZPay 每笔收取 1% 手续费，已通过运行态 `settings.RECHARGE_FEE_RATE=1` 统一处理。
- 不修改套餐、流量包和充值的基础价格字段；基础价仍作为履约、套餐定义和用户权益口径。
- 用户实际支付金额统一由后端按 `pay_amount = amount + 手续费` 生成，手续费按分向上取整。
- `/purchase` 前端列表卡片、确认区和支付按钮都会展示含 1% 后的实付价。

## 已验证运行态

- `settings.RECHARGE_FEE_RATE = 1`。
- 最新手续费后订单示例：
  - `payment_orders.id=52`
  - `order_type=balance`
  - `amount=2000.00`
  - `fee_rate=1.0000`
  - `pay_amount=2020.00`
  - `provider_key=easypay`
  - `payment_type=alipay`
- ZPay provider 仍为：
  - `name=ZPay Alipay`
  - `provider_key=easypay`
  - `supported_types=alipay`
  - `payment_mode=popup`
  - `refund_enabled=false`
  - `allow_user_refund=false`

## 前端改动

- 新增 `frontend/src/components/payment/payableAmount.ts`，复用与后端一致的手续费向上取整规则。
- `SubscriptionPlanCard` 新增 `feeRate` 参数：
  - 手续费为 0 时仍显示基础价。
  - 手续费大于 0 时主价格显示含手续费实付价，并展示 `基础价 + 费率 手续费`。
- `TrafficPackCard` 同样新增 `feeRate` 参数。
- `PaymentView` 将 `checkout.recharge_fee_rate` 传给订阅卡片和流量包卡片。
- 确认页原有手续费拆分展示和按钮实付价逻辑保留。

## 验证

- `./node_modules/.bin/vitest run src/components/payment/__tests__/SubscriptionPlanCard.spec.ts src/components/payment/__tests__/TrafficPackCard.spec.ts src/views/user/__tests__/PaymentView.spec.ts`
  - 3 个测试文件通过，25 个测试通过。
- `go test ./internal/payment -run TestCalculatePayAmount`
  - 通过。
- `node --test scripts/__tests__/configure-zpay-alipay-runtime.test.mjs`
  - 3 个测试通过。

## 注意

- 不要把 `RECHARGE_FEE_RATE=1` 理解为套餐涨价；它是支付手续费，履约金额仍以基础 `amount` 识别套餐/流量包。
- ZPay 支付金额校验必须继续使用本地 `pay_amount`，不能只校验基础 `amount`。
