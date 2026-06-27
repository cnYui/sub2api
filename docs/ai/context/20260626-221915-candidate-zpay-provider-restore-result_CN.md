# 18084 候选环境 ZPay 支付方式恢复结果

## 结论

ZPay 支付链路在 main 中已经打通；18084 显示“充值功能暂未开放/支付功能未开放”的原因不是代码链路缺失，而是候选环境清洗脚本把已克隆的 ZPay provider 和支付宝可见支付方式关掉了。

## 证据

恢复前 18084 checkout：

- `plans=4`
- `traffic_packs=3`
- `methods={}`
- `RECHARGE_FEE_RATE=1`

恢复前候选 DB：

- `payment_enabled=true`
- `payment_visible_method_alipay_enabled=false`
- `ENABLED_PAYMENT_TYPES=[]`
- `ZPay Alipay` provider 存在，但 `enabled=false`
- provider 配置仍为 `provider_key=easypay`、`supported_types=alipay`、`payment_mode=popup`

## 已恢复

只更新 18084 候选库：

- `payment_enabled=true`
- `payment_visible_method_alipay_enabled=true`
- `payment_visible_method_alipay_source=easypay_alipay`
- `payment_visible_method_wxpay_enabled=false`
- `ENABLED_PAYMENT_TYPES=alipay`
- `ZPay Alipay.enabled=true`
- `ZPay Alipay.supported_types=alipay`
- `ZPay Alipay.payment_mode=popup`
- `refund_enabled=false`
- `allow_user_refund=false`

## 脚本修正

候选 worktree 的 `deploy/sql/candidate-sanitize.sql` 已同步修正：

- 保留 `payment_enabled=true`。
- 保留支付宝-only ZPay provider。
- 继续禁用微信、退款、SMTP、监控和通知副作用。
- 其他非 ZPay 支付 provider 仍会被关闭。

候选脚本测试 `deploy/test-candidate-rehearsal-scripts.sh` 已新增断言，防止未来再次把 ZPay Alipay 或支付宝可见方式关掉。

## 验证

`GET /api/v1/payment/checkout-info` 当前返回：

- `method_keys=["alipay"]`
- `alipay.currency=CNY`
- `plan_count=4`
- `traffic_pack_count=3`
- `recharge_fee_rate=1`

浏览器验证：

- 18084 普通用户侧栏显示“购买订阅”和“我的订单”。
- `/purchase` 不再显示“充值功能暂未开放/支付功能未开放”。
- 页面进入支付中状态时显示“重新打开支付页面”和“等待支付...”。

公网验证：

- `GET http://127.0.0.1:18080/health` 为 200。
- 未重建公网 `sub2api`。
- 未修改公网 Postgres/Redis。

## 注意

18084 候选环境现在使用生产 DB 克隆中的 ZPay 商户配置。点击最终创建订单/重新打开支付页面会生成真实 ZPay 托管收银台 URL；测试支付时应按真实订单处理。
