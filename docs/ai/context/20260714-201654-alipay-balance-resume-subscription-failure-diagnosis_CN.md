# 支付宝充值续接余额购买套餐失败诊断

## 结论

2026-07-13 用户 `1510623550@qq.com`（`users.id=41`）未成功获得新套餐，不是支付宝回调失败，也不是后端套餐履约事务报错。真实原因是当前页面并未实现“支付宝支付一部分、余额支付一部分”的组合订单；余额不足时只会创建一笔支付宝余额充值订单，并清空原套餐选择。充值成功后页面只刷新余额，不会自动续接 `POST /api/v1/payment/orders/balance-pay`，因此后端从未收到套餐购买请求，自然没有套餐订单可供发放。

## 运行态证据

### 金额完全对应余额补差流程

- 目标套餐为 `subscription_plans.id=7/group_id=9`，基础价 `79.00` 元。
- 运行设置 `RECHARGE_FEE_RATE=1`，商品应付金额为 `79.79` 元。
- 用户充值前余额可由最终余额反推为 `6.32` 元。
- 精确差额为 `79.79 - 6.32 = 73.47` 元；前端对差额执行 `Math.ceil`，所以创建 `74` 元充值，金额完全吻合。
- 充值后用户余额为 `80.32` 元，当前仍为 `80.32` 元，说明这笔余额没有被套餐购买扣除。

### 支付宝充值本身成功

余额充值订单 `payment_orders.id=175`：

- `order_type=balance`
- `payment_type=alipay`
- `amount/pay_amount=74.00`
- `created_at=2026-07-13 15:09:18+08`
- `paid_at=2026-07-13 15:09:43+08`
- `completed_at=2026-07-13 15:09:43+08`
- `status=COMPLETED`

审计链完整包含 `ORDER_CREATED -> ORDER_PAID -> RECHARGE_SUCCESS`，兑换记录 `PAY-175-39086` 已被 user 41 使用并入账。因此支付宝、回调、余额发放均正常。

### 套餐购买请求根本没有发生

- 2026-07-13 全天容器访问日志中，`/api/v1/payment/orders/balance-pay` 请求数为 0。
- 同日数据库 `payment_type=balance` 的持久化订单数为 0。
- user 41 只有历史人工流量包订单和本次余额充值订单，没有任何套餐购买订单，也没有套餐订单失败记录。
- 同日其他用户的支付宝套餐订单 `172/176/177` 均正常形成 `ORDER_PAID -> SUBSCRIPTION_ASSIGNED -> SUBSCRIPTION_SUCCESS`，证明套餐履约链路本身可用。

### 后续套餐是管理员人工发放

- `user_subscriptions.id=110/group_id=9` 创建于 `2026-07-13 15:18:01+08`。
- `assigned_by=13`，对应管理员 `xiaobianfuai@gmail.com`。
- 同时容器日志记录 `POST /api/v1/admin/subscriptions/assign` 返回 HTTP 200。
- 因此该套餐不是支付系统发放，而是失败后由管理员人工补发。

## 代码根因

`frontend/src/views/user/PaymentView.vue` 当前流程：

1. `ensureBalanceEnough()` 计算差额后调用 `openRechargeConfirm(shortage)`。
2. `openRechargeConfirm()` 立即把 `selectedPlan` 和 `selectedTrafficPack` 清空，只保留一笔余额充值上下文。
3. `confirmRecharge()` 只创建 `order_type=balance/payment_type=alipay` 的充值订单。
4. 充值成功回调 `onPaymentSuccess()` 只刷新用户余额；不会恢复原套餐，也不会调用 `balancePayProduct()`。
5. `balancePayProduct()` 只有用户重新选择套餐、再次选择余额并再次确认时才会调用。

这不是后端“发放失败”，而是前端续接状态缺失导致套餐购买步骤没有发生。当前交互看起来像组合支付，实际只是独立充值，产品语义与用户预期不一致。

## 修复方向

若产品目标是真正的“余额 + 支付宝补差后自动购买”，应显式保存待购买商品、应付金额和一次性续接状态；余额充值完成且金额到账后，重新校验商品、活跃套餐和当前余额，再由用户确认或自动提交余额购买。不能仅靠页面内 `selectedPlan`，也不能把两笔独立订单误称为单笔组合支付。

本轮仅做只读诊断；未修改订单、余额、订阅、数据库、Redis、容器、支付配置或业务代码。
