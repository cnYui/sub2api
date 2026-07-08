# 人民币余额支付与邀请返利重构结果

## 已完成

- 后端订单类型已收口到 `balance`、`subscription`、`traffic_pack`；非法非空 `order_type` 返回 `INVALID_ORDER_TYPE`，避免未知类型落到余额履约。
- 余额充值仍走支付宝外部支付，金额为 1:1 人民币入账：`amount=pay_amount`，`fee_rate=0`。
- 用户侧外部下单只允许支付宝；微信、Stripe、Airwallex 仍保留后台配置和历史订单能力，但不出现在用户购买/充值入口。
- 新增余额支付商品接口 `POST /api/v1/payment/orders/balance-pay`：只支持套餐和流量包，事务内条件扣减 `users.balance >= pay_amount`，创建 `payment_type=balance` 完成订单并发放权益。
- 余额支付不产生邀请返利；支付宝完成的余额充值、套餐和流量包订单按 `amount` 返利。
- 邀请返利默认值改为 8%、冻结 24 小时、有效期 365 天、单被邀请用户累计上限 ¥100，并新增迁移 `160_rmb_balance_payment_affiliate_defaults.sql` 覆盖运行态 settings。
- `/purchase` 第一张卡为余额充值；充值金额默认 1 元，限制 1-100 整数，只显示支付宝；产品确认页只显示支付宝和余额，余额不足直接进入充值确认页。
- 订单列表、后台订单详情/筛选、支付结果页和用户余额相关文案已统一人民币口径；后台订单筛选补齐 `payment_type=balance` 和 `order_type=traffic_pack`。

## 验证

- `cd backend && go test -count=1 -tags=unit ./internal/payment ./internal/service ./internal/handler ./internal/server`：通过。
- `cd backend && go test -count=1 ./migrations`：通过。
- `cd frontend && pnpm typecheck`：通过。
- `cd frontend && pnpm vitest run src/api/__tests__/payment.spec.ts src/components/payment/__tests__/paymentFlow.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts src/components/user/profile/__tests__/ProfileInfoCard.spec.ts src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts`：68 个测试通过。
- `cd frontend && pnpm build`：通过；Vite 仍输出既有 chunk/dynamic import 警告。
- `cd backend && go test -count=1 ./cmd/server`：通过。

## 未执行项

- 未启动本地服务做浏览器手工验收；本轮只完成代码、自动化测试、构建验证。
- 未连接、迁移、重启或替换任何公网容器/数据库/Redis/nginx。

## 运行态提醒

- 发布公网前先备份 `sub2api-candidate-postgres` 与 `sub2api-candidate-redis`。
- 迁移 `160_rmb_balance_payment_affiliate_defaults.sql` 会显式覆盖返利 settings：8%、24 小时、365 天、¥100。
- 余额支付依赖事务内条件扣款，不能改回允许透支的通用余额扣减路径。
