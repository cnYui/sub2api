# ZPay 易支付迁移结果

## 已完成

- 参考 `18080` 实例确认其支付提供商为 `easypay / ZPay Alipay`，支付宝使用 `popup` 托管收银台。
- 当前仓库已支持解析 ZPay `mapi.php` 的 `img` 字段，并将其通过 `qr_image_url` 传到前端。
- 支付页、恢复快照和独立二维码页均支持在缺少二维码文本时展示服务商二维码图片。
- 新增 `scripts/configure-zpay-alipay-runtime.mjs` 及单元测试；脚本仅接受环境变量中的商户凭证，默认 dry-run，并会在输出中脱敏。
- 已按用户明确授权，将 `18080` 的 ZPay 商户凭证复制至隔离 `18082` PostgreSQL 实例。
- 目标实例启用支付宝可见支付方式并路由至 EasyPay/ZPay；微信保持关闭；服务端退款和用户退款与参考实例一致。
- 使用不泄露凭证的摘要和哈希指纹确认源、目标商户凭证一致。
- `18082` 已重新构建并恢复健康，地址为 `http://127.0.0.1:18082`。

## 验证

- `go test ./internal/payment/provider ./internal/service` 通过。
- `go test ./internal/service -run 'TestBuildCreateOrderResponse|TestSanitizeCreatePaymentResponseDetails' -count=1` 通过。
- `node --test scripts/__tests__/configure-zpay-alipay-runtime.test.mjs`：3 项通过。
- Docker 生产构建中的 `vue-tsc -b && vite build` 通过。
- 本机没有前端 `node_modules`，因此未直接执行 Vitest；Docker 构建已完成生产级 TypeScript 校验。

## 未执行

- 没有创建真实 ZPay 订单、扫码或退款，避免本地 `18082` 订单被参考站点回调。
