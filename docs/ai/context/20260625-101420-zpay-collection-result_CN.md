# ZPay 收款功能实现结果

## 分支

- 从本地 `main` 创建：`codex/zpay-payment-20260625`

## 已实现

- 后端 EasyPay provider 支持解析 ZPay `mapi.php` 成功响应里的 `img` 字段。
- 创建订单响应新增 `qr_image_url`，用于承载服务商托管的二维码图片地址。
- 前端支付 flow 支持只有 `qr_image_url`、没有 `qr_code` 时进入二维码等待态。
- 购买页内嵌支付面板 `PaymentStatusPanel` 支持直接展示服务商二维码图片。
- 旧路由 `/payment/qrcode` 对应的二维码页也支持 `qr_image_url` 查询参数。
- 支付恢复快照新增 `qrImageUrl`，并兼容旧快照。
- 新增测试覆盖 ZPay `img` 字段、前端决策和二维码图片展示。

## 未做

- 未新增独立 `zpay` provider。ZPay 按 EasyPay 协议接入，复用现有订单、查单、回调和退款链路。
- 未把商户密钥写入代码、文档或提交。
- 未自动向本地数据库写入商户密钥，需管理员在后台服务商管理里配置。

## 验证

- `go test ./internal/payment/provider ./internal/service`
- `pnpm vitest run src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/PaymentStatusPanel.spec.ts src/views/user/__tests__/PaymentQRCodeView.spec.ts`
- `pnpm typecheck`
- `git diff --check`
- 敏感信息扫描：未在 `backend`、`frontend`、`docs`、`deploy`、README 中发现用户提供的 PID 或密钥片段。

## 注意

- 当前 Git 工作区已有一条非本次改动的删除：`docs/ai/content/20260621-112205-local-frontend-branch-merge-record_CN.md`，本次未处理。
- `docs/ai/context/` 被仓库 `.gitignore` 忽略，本次上下文文档只作为本地协作记录保存。
