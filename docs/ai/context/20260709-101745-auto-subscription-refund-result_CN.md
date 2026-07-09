# 自动套餐退款结果

时间：2026-07-09 10:17 JST

## 已完成

- 用户侧 `/orders` 退款按钮改为支持自动退款：
  - 支付宝月度套餐：显示按钮条件仍要求 provider 在可退款列表中。
  - 余额支付月度套餐：即使没有 `provider_instance_id` 也显示按钮。
  - 流量卡不显示退款按钮。
- 后端 `POST /api/v1/payment/orders/:id/refund-request` 已从“提交申请”改为“用户自动退款”：
  - 只允许 `order_type=subscription`。
  - 拒绝 `traffic_pack`。
  - 支付宝订单仅允许 `payment_type=alipay` 且 provider 开启 `refund_enabled/allow_user_refund`。
  - 余额订单仅允许 `payment_type=balance`。
- 退款金额规则：
  - 剩余天数按 `floor((expires_at - now) / 24h)`，还剩 `24.2` 天按 `24` 天。
  - 支付宝退款不退 1% 手续费，基数为 `amount`。
  - 余额退款退手续费，基数为 `pay_amount`。
  - 金额保留到 1 位小数，四舍五入。
- 退款成功后撤销当前 active subscription。
- 网关失败时不撤销套餐，订单标记 `REFUND_FAILED`。
- 余额退款撤销套餐失败时尝试扣回余额，并记录 `REFUND_ROLLBACK_FAILED` 审计。

## 运行态配置

- 写库前备份：
  - `deploy/backups/20260709-100455-sub2api-candidate-before-zpay-auto-refund.dump`
  - 已用容器内 `pg_restore -l` 验证可读。
- 已更新公网候选库 `payment_provider_instances.id=1 / ZPay Alipay`：
  - `provider_key=easypay`
  - `supported_types=alipay`
  - `enabled=true`
  - `payment_mode=popup`
  - `refund_enabled=true`
  - `allow_user_refund=true`
  - `apiBase=https://zpayz.cn`
  - config 中已写入 PID/KEY，但本文档不记录完整密钥。
- 支付宝可见入口仍为：
  - `payment_enabled=true`
  - `payment_visible_method_alipay_enabled=true`
  - `payment_visible_method_alipay_source=easypay_alipay`

## 验证

- RED：`go test -count=1 -tags=unit ./internal/service -run 'Test(CalculateSubscriptionRefundAmountFloorsRemainingDaysAndRoundsToOneDecimal|RequestRefundAutomatically|RequestRefundRejectsTrafficPackAutoRefund)'` 初始失败于缺少 `calculateSubscriptionRefundAmount`。
- GREEN：
  - `go test -count=1 -tags=unit ./internal/service -run 'Test(CalculateSubscriptionRefundAmountFloorsRemainingDaysAndRoundsToOneDecimal|RequestRefundAutomatically|RequestRefundRejectsTrafficPackAutoRefund)'`
  - `go test -count=1 -tags=unit ./internal/service -run 'Test.*Refund|TestBalancePay'`
  - `go test -count=1 -tags=unit ./internal/service`
  - `pnpm vitest run src/views/user/__tests__/paymentRefund.spec.ts`
  - `pnpm vitest run src/views/user/__tests__/paymentRefund.spec.ts src/views/user/__tests__/paymentUx.spec.ts`
  - `pnpm typecheck`
  - `git diff --check`
- 运行态只读确认：
  - `ZPay Alipay.refund_enabled=true`
  - `ZPay Alipay.allow_user_refund=true`
  - `apiBase=https://zpayz.cn`

## 未做

- 未执行真实用户订单退款。
- 未构建镜像、未替换或重启 18084 容器。
- 当前代码改动尚未部署到公网容器；运行态 DB 退款按钮已打开，但公网自动退款逻辑要等新镜像发布后生效。
