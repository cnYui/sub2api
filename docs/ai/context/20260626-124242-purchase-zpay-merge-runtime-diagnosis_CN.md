# 2026-06-26 12:42 purchase / ZPay 分支合并与运行态排查

## 问题

本地 `http://127.0.0.1:5174/purchase` 选择 `29 元订阅池` 后显示“充值功能暂未开放”，确认支付按钮禁用。用户要求检查 ZPay 接入分支是否合并。

## 分支结论

- 当前工作区在 `main`，状态为 `main...origin/main [ahead 50, behind 47]`。
- `main` 已包含 ZPay 相关分支：
  - `codex/zpay-payment-20260625`
  - `codex/zpay-auto-payment-no-manual-20260626`
- 关键提交已进入 `main` 的 ancestry：
  - `cd85293c9 feat: support zpay easypay qr images`
  - `561a0febf merge: 合并 ZPay 收款与并发排查记录`
  - `d6443be85 feat: 下线购买页人工支付`
- 因此这次现象不是“ZPay 代码分支完全没合并”。

## 代码行为

- `/purchase` 当前走 `frontend/src/views/user/PaymentView.vue` 的内置支付页。
- 该页在 `enabledMethods.length === 0` 或 `!hasPaymentMethods` 时显示 `payment.notAvailable`，中文文案为“充值功能暂未开放”。
- 套餐卡能显示，说明 checkout 中的 subscription plans 有数据；支付按钮禁用说明 checkout 中可见支付方式为空。

## 运行态证据

公开设置接口：

- `payment_enabled=true`
- `purchase_subscription_enabled=false`

运行态 PostgreSQL `settings`：

```text
payment_enabled=true
payment_visible_method_alipay_enabled=false
payment_visible_method_alipay_source=
payment_visible_method_wxpay_enabled=false
payment_visible_method_wxpay_source=
```

运行态 PostgreSQL `payment_provider_instances`：

```text
(0 rows)
```

即当前本地/public DB 没有任何支付服务商实例，也没有启用支付宝或微信的前台可见支付方式。

## 根因

ZPay 接入代码已经在本地 `main` 中，但运行态没有完成支付 provider 配置。ZPay 是作为 EasyPay 协议实例接入，不是独立 `zpay` provider；代码不会自动写入真实商户 `PID/PKey`。没有 `payment_provider_instances` 时，后端无法向 checkout 暴露 `alipay`/`wxpay` 方法，前端只能显示“充值功能暂未开放”。

`d6443be85` 已下线购买页人工支付，因此旧的手动二维码兜底也不会再出现。当前设计要求订阅、充值、GPT 流量包都走 Sub2API 支付订单与 EasyPay/ZPay 动态二维码。

## 后续处理方向

必须在运行态配置至少一个 enabled 的 EasyPay provider instance：

- `provider_key=easypay`
- `supported_types` 至少包含 `alipay`，需要微信则加 `wxpay`
- `payment_mode=qrcode` 或符合当前 ZPay 返回形式的模式
- `config` 包含 `pid`、`pkey`、`apiBase=https://zpayz.cn`、`notifyUrl`、`returnUrl`

然后启用前台可见方式：

- `payment_visible_method_alipay_enabled=true`
- `payment_visible_method_alipay_source=easypay_alipay`
- 如启用微信：
  - `payment_visible_method_wxpay_enabled=true`
  - `payment_visible_method_wxpay_source=easypay_wxpay`

注意：不要在文档、日志、提交中记录完整 `PID/PKey` 或其他支付密钥。
