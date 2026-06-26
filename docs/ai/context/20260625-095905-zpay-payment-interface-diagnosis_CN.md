# ZPay 支付接口现状排查

## 问题

确认当前项目支付接口是否已经使用 ZPay，以及是否已经完成可用实现。

## 结论

- 代码没有独立的 `zpay` provider。
- ZPay 被文档归类为“兼容 EasyPay 协议的第三方聚合支付”，应通过 `easypay` provider 接入。
- EasyPay 支付链路已经实现：后台服务商实例配置、订单创建、二维码/跳转支付、查询、异步回调验签、退款请求、前端支付模式分流均已有代码。
- 当前本地运行态未配置任何支付服务商实例，因此没有实际启用 ZPay 商户。

## 代码证据

- `backend/internal/payment/types.go` 定义了 `TypeEasyPay = "easypay"`，未定义 `zpay`。
- `backend/internal/payment/provider/factory.go` 只接受 `easypay`、`alipay`、`wxpay`、`stripe`、`airwallex`。
- `backend/internal/payment/provider/easypay.go` 实现 EasyPay 协议：
  - 必填配置：`pid`、`pkey`、`apiBase`、`notifyUrl`、`returnUrl`
  - 二维码模式调用 `/mapi.php`
  - 弹窗/跳转模式生成 `/submit.php`
  - 查询调用 `/api.php`
  - 回调按 EasyPay MD5 签名验签
- `backend/internal/handler/payment_webhook_handler.go` 注册并处理 `/api/v1/payment/webhook/easypay` 的 GET/POST 回调。
- `frontend/src/components/payment/providerConfig.ts` 后台表单支持 `easypay`，字段包含 `pid`、`pkey`、`apiBase`、支付宝/微信通道 ID，并自动生成回调地址。
- `docs/PAYMENT_CN.md` 明确推荐 ZPay 作为国内渠道 EasyPay 协议服务商。

## 运行态验证

通过本地 API 验证：

- `GET /health` 返回 `{"status":"ok"}`
- `GET /api/v1/admin/payment/config` 显示支付系统 `enabled=true`
- `GET /api/v1/admin/payment/providers` 返回空数组 `[]`

因此当前本地服务“支付系统开启，但没有配置 EasyPay/ZPay 服务商实例”。

## 后续接入方式

如果要真正使用 ZPay，应在管理后台“设置 → 支付设置 → 服务商管理”新增服务商实例：

- 服务商类型：EasyPay
- 支持方式：按需要选择支付宝、微信支付
- API 基础地址：填写 ZPay 提供的 EasyPay API 基础地址
- PID/PKey：填写 ZPay 商户后台给出的商户号和密钥
- 支付模式：二维码或弹窗

保存并启用后，还需要把前台可见支付方式来源设置为 EasyPay 支付宝 / EasyPay 微信，否则用户侧不会出现对应入口。
