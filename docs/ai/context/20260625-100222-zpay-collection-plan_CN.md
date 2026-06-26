# ZPay 收款功能实现计划

## 目标

在 `codex/zpay-payment-20260625` 分支上补齐 ZPay/EasyPay 收款体验，重点支持 `mapi.php` 返回的 `img` 二维码图片字段，并保留安全的后台配置方式。

## 步骤

1. 后端 RED：
   - 在 EasyPay provider 测试中添加 `mapi.php` 返回 `img` 的用例。
   - 在 service 响应测试中添加 `qr_image_url` 透传用例。
   - 先运行目标测试，确认失败来自字段缺失。

2. 后端 GREEN：
   - `backend/internal/payment/types.go` 增加 `QRImageURL`。
   - `backend/internal/payment/provider/easypay.go` 解析 `img`。
   - `backend/internal/service/payment_service.go` 增加 `qr_image_url`。
   - `backend/internal/service/payment_order.go` 透传字段。
   - 运行目标 Go 测试确认通过。

3. 前端 RED：
   - 在 `frontend/src/components/payment/__tests__/paymentFlow.spec.ts` 增加只有 `qr_image_url` 的启动决策测试。
   - 在 `frontend/src/views/user/__tests__/PaymentView.spec.ts` 或二维码页相关测试中补充路由参数/展示测试。
   - 先运行目标测试，确认失败来自未支持 `qr_image_url`。

4. 前端 GREEN：
   - `frontend/src/types/payment.ts` 增加 `qr_image_url`。
   - `frontend/src/components/payment/paymentFlow.ts` 增加 `qrImageUrl` 快照字段与决策。
   - `frontend/src/views/user/PaymentView.vue` 传递 `qr_image_url` 到二维码页和恢复态。
   - `frontend/src/views/user/PaymentQRCodeView.vue` 支持直接展示二维码图片。
   - 运行目标前端测试确认通过。

5. 配置说明：
   - 新增或更新 `docs/ai/context` 结果记录，说明实际配置方式。
   - 不记录完整 KEY。

6. 验证：
   - `go test ./internal/payment/provider ./internal/service`
   - 前端目标 Vitest 测试
   - `git diff` 复查无密钥落盘。

## 风险控制

- 不提交用户提供的商户密钥。
- 不更改订单履约状态机。
- 不新增支付事实源。
- 发现 ZPay 真实接口还有额外差异时，先用测试复现再补实现。
