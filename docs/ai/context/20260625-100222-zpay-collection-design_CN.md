# ZPay 收款功能设计

## 背景

用户提供 ZPay 商户信息、网关 `https://zpayz.cn/`、支付宝收款图片，以及 Java/Python demo。现有项目已经有 EasyPay 聚合支付实现，ZPay 文档也声明“已集成易支付接口则无需另外开发”。

## 必须解决的问题

- 让当前项目可以通过 ZPay 完成支付宝/微信收款。
- 不把商户密钥写入代码、文档、提交或日志。
- 保留 Sub2API 作为唯一支付订单、用户 Key、计费与履约事实源。
- 用户支付完成后，仍走现有订单回调、查单、履约和订阅发放链路。

## 现有实现判断

- `easypay` provider 已覆盖 ZPay 主接口：
  - `submit.php` 页面跳转支付
  - `mapi.php` API 支付
  - `api.php?act=order` 查单
  - `api.php?act=refund` 退款
  - EasyPay MD5 签名与回调验签
- 当前本地运行态未配置任何 provider instance，因此不是“已在用 ZPay”。
- ZPay `mapi.php` 成功响应除了 `payurl`、`qrcode`，还可能返回 `img`，表示二维码图片地址。现有后端未暴露该字段，前端也只能把 `qr_code` 当二维码内容重新生成。如果 ZPay 只返回 `img`，用户会进入无法展示支付二维码的场景。

## 设计方案

采用“增强现有 EasyPay 兼容层”的方式，不新增独立 `zpay` provider。

### 后端

- 在 `payment.CreatePaymentResponse` 增加 `QRImageURL` 字段。
- EasyPay `mapi.php` 响应解析新增 `img` 字段：
  - `qrcode` 继续作为可重新生成二维码的支付内容。
  - `img` 作为已生成好的二维码图片地址透传。
- `service.CreateOrderResponse` 新增 `qr_image_url` JSON 字段。
- `buildCreateOrderResponse` 将 provider 的 `QRImageURL` 传给前端。
- 不把 `img` 写入 `payment_orders.qr_code`，避免把“二维码内容”和“二维码图片地址”混在同一个持久字段里。

### 前端

- `CreateOrderResult` 与支付恢复快照增加 `qrImageUrl`。
- `decidePaymentLaunch` 在 `qr_code` 或 `qr_image_url` 任一存在时进入 `qr_waiting`。
- 支付二维码页支持 `qr_image_url` 查询参数：
  - 有 `qr_image_url` 时直接展示图片。
  - 有 `qr_code` 时保持现有 canvas 二维码生成。
  - 两者都有时优先使用 `qr_code`，因为它可以叠加本地支付图标并保持现有体验。
- 移动端 fallback 判断也接受 `qr_image_url`。

### 配置

- 不在代码中写死用户的 PID/KEY。
- 保留后台“服务商管理 → EasyPay”作为正式配置入口。
- 可新增一份不含密钥的本地配置说明，明确：
  - 服务商类型选 EasyPay
  - API Base 填 `https://zpayz.cn`
  - PID/KEY 由管理员在后台填入
  - 支付模式建议先用 `qrcode`，验证成功后可切 `popup`
  - 前台可见支付方式来源需设置为 EasyPay 支付宝 / EasyPay 微信

## 取舍

- 不新增 `zpay` provider：ZPay 是 EasyPay 协议实现，新增 provider 会造成重复逻辑和后续维护分叉。
- 不自动写入数据库密钥：支付密钥属于敏感配置，不能通过提交或上下文文档固化。
- 不改订单表结构：`qr_image_url` 是创建订单时的前端展示数据，支付结果仍靠订单号、查单和回调闭环。

## 验收标准

- EasyPay `mapi.php` 返回 `img` 时，创建订单响应包含 `qr_image_url`。
- 前端收到只有 `qr_image_url`、没有 `qr_code` 的响应时，进入等待支付态并展示图片。
- 原有 `qr_code`、`pay_url`、Stripe、Airwallex、微信 JSAPI 流程不退化。
- 相关 Go/Vitest 测试通过。
