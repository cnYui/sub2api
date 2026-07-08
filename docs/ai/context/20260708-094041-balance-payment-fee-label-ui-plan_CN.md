# 余额支付按钮手续费文案 UI 调整计划

## 背景

用户要求在支付页面的余额付款说明栏中移除“手续费 1%”副文案，其他内容保持不变，仅修改 UI。

## 设计

- 只调整前端支付方式按钮展示逻辑。
- `PaymentMethodSelector` 对 `balance` 类型隐藏费率副文案。
- 外部支付方式如果配置了 `fee_rate > 0`，仍按原逻辑显示手续费副文案。
- 订单金额明细中的手续费行和后端支付金额计算不变。

## 实施计划

1. 在支付页测试中增加断言：商品结算时余额按钮不显示 `payment.fee`，但余额支付方式仍存在。
2. 修改 `frontend/src/components/payment/PaymentMethodSelector.vue`，将手续费副文案显示条件限定为非余额支付。
3. 运行目标前端测试验证 UI 回归。

## 验证

- `pnpm --dir frontend test:run src/components/payment/__tests__/PaymentMethodSelector.spec.ts`
- `pnpm --dir frontend test:run src/views/user/__tests__/PaymentView.spec.ts`
