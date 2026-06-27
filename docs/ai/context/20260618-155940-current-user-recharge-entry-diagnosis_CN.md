# 当前个人用户充值入口核查

## 结论

- 代码中内置支付系统存在，支持余额充值、订阅购买、订单列表，以及 Stripe、支付宝、微信、EasyPay、Airwallex 等服务商。
- 普通个人用户入口是左侧菜单的「充值/订阅」，路由为 `/purchase`。
- `/purchase` 使用 `frontend/src/views/user/PaymentView.vue`，页面内有两个 tab：
  - `充值`：余额充值，受 `balance_disabled` 控制。
  - `订阅`：购买订阅套餐。
- `/orders` 是用户订单列表。
- `/payment/qrcode`、`/payment/result`、`/payment/stripe`、`/payment/airwallex`、`/payment/stripe-popup` 是支付过程和结果页，不是主入口。

## 当前实例状态

- `http://127.0.0.1:18080/api/v1/settings/public` 返回 `payment_enabled=false`。
- `https://aaccx.pw/api/v1/settings/public` 返回 `payment_enabled=false`。
- 前端路由守卫会在 `payment_enabled=false` 时拦截 `requiresPayment` 页面：
  - 普通用户访问 `/purchase` 会跳回 `/dashboard`。
  - 侧边栏的「充值/订阅」和「我的订单」入口也会被功能开关隐藏。

## 管理员配置位置

- 管理后台进入「设置」→「支付设置」启用支付。
- 启用后在支付服务商管理中配置服务商实例。
- 支付服务商代码位于 `backend/internal/payment/provider/`：
  - `stripe.go`
  - `alipay.go`
  - `wxpay.go`
  - `easypay.go`
  - `airwallex.go`
- 用户支付 API 路由位于 `backend/internal/server/routes/payment.go`，用户侧接口前缀为 `/api/v1/payment/*`。

## 判断

当前项目确实有 Stripe、支付宝等支付能力，但当前本地和公网实例都没有启用支付，所以个人用户现在没有可见充值入口。若要让用户自助充值，需要先启用 `payment_enabled` 并配置至少一个可用支付服务商实例。
