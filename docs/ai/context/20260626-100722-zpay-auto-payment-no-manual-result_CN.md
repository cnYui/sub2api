# ZPay 动态订单自动支付与人工支付下线结果

## 完成内容

- 基于本地 `main` 创建实现分支：`codex/zpay-auto-payment-no-manual-20260626`。
- 购买页已移除 `ManualPaymentDialog` 人工支付入口。
- 订阅和 GPT 流量包不再展示静态收款码；余额充值原本只在无支付方式时展示不可用提示，本次保持该自动订单链路。
- 无可用支付方式时，订阅和 GPT 流量包确认按钮禁用，并展示 `payment.notAvailable`。
- 29 / 39 / 59 / 99 元套餐继续通过 `plan_id` 创建订阅订单，前端金额只用于展示和请求上下文，后端仍以套餐表价格作为订单金额事实源。
- ZPay/EasyPay 动态二维码继续使用 `qr_code` 或 `qr_image_url` 展示。
- 后端测试覆盖 ZPay/EasyPay 订阅回调金额匹配才履约、金额不匹配不履约。
- EasyPay provider 回归测试明确断言创建 API 支付时 `money=29.00` 会原样提交给 `mapi.php`。

## 关键行为

- 用户点击 29 元或 39 元套餐时，前端创建 `order_type=subscription` 的支付订单，并进入 `PaymentStatusPanel` 等待态展示 ZPay 返回的二维码图片或链接。
- 用户付款后，自动履约仍复用现有回调/查单链路：签名、订单号、provider/merchant metadata 和金额全部一致后，才会调用订阅发放。
- 如果 ZPay 回调金额与本地订单 `pay_amount` 不一致，订单保持 `PENDING`，不会发放订阅。
- 本次不删除 `/redeem` 页面和兑换码系统，只下线购买页人工支付入口，避免影响历史兑换码用途。

## 验证

- `./node_modules/.bin/vitest run src/views/user/__tests__/PaymentView.spec.ts`
- `go test -tags=unit ./internal/service -run 'TestConfirmPaymentRejectsSubscriptionAmountMismatch|TestConfirmPaymentCompletesSubscriptionWhenAmountMatches|TestPaymentAmountToleranceForThreeDecimalCurrency'`
- `go test ./internal/payment/provider`
- `./node_modules/.bin/vitest run src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/PaymentStatusPanel.spec.ts`
- `./node_modules/.bin/vue-tsc --noEmit`
- `gofmt -w backend/internal/service/payment_fulfillment_test.go backend/internal/payment/provider/easypay_create_test.go`
- `git diff --check`
- `rg -n "ManualPaymentDialog|payment\\.manual|manual-alipay|manual-wxpay|showManualPaymentDialog|goRedeem" frontend/src` 无输出

## 注意

- 本机 `frontend/node_modules` 由 pnpm 9 创建，当前 Codex runtime 的 pnpm 11 会尝试非交互重建依赖并被 `ERR_PNPM_ABORTED_REMOVE_MODULES_DIR_NO_TTY` 拦截；本次验证直接调用已有 `node_modules/.bin` 下的 Vitest 和 vue-tsc，没有改依赖目录。
- 未记录 ZPay PID、密钥或任何完整敏感凭据。
