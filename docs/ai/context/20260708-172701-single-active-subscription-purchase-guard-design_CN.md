# 单用户单生效订阅购买拦截设计

时间：2026-07-08 17:27 JST

## 背景

`3876129758@qq.com` 先在 2026-07-05 购买 29 元套餐，又在 2026-07-08 购买 59 元套餐。当前系统的订阅履约逻辑只按同 `(user_id, group_id)` 续期，异套餐分组会新增 active 订阅，导致同一用户可以同时拥有多个未过期 active 订阅。

这是业务规则缺失：用户套餐不够用时不能直接再买另一个订阅，应先联系管理员处理退款/调整。

## 目标规则

- 每个用户最多只能拥有一个生效订阅。
- 用户已经有任意 active 且未过期订阅时，不允许再创建订阅购买订单。
- 用户点击订阅套餐时提示：`需要先和管理员联系来进行退款`。
- 后端接口必须强制拦截，避免绕过前端直接调用 `/payment/orders` 或 `/payment/orders/balance-pay`。
- 流量包购买、余额充值不受影响。

## 设计

### 后端

在 `PaymentService.validateSubOrder()` 中增加用户维度 active 订阅检查：

- 仅在 `req.UserID > 0` 时执行检查。
- 调用 `subscriptionSvc.userSubRepo.ListActiveByUserID(ctx, req.UserID)` 或等价仓储接口。
- 如存在任意 active 且未过期订阅，直接返回 `409 ACTIVE_SUBSCRIPTION_EXISTS`。
- 错误 message 固定为：`需要先和管理员联系来进行退款`。

这样外部支付下单与余额支付订阅都会复用同一校验，因为 `BalancePayOrder.resolveBalancePayProduct()` 也调用 `validateSubOrder()`。

### 前端

在购买页点击订阅卡片时增加提前提示：

- 如果 `subscriptionStore.activeSubscriptions` 中已有 active 订阅，直接 `appStore.showError(t('payment.errors.ACTIVE_SUBSCRIPTION_EXISTS'))`，不进入确认页。
- 在 `payment.errors` 中补中英文错误码映射，中文严格使用：`需要先和管理员联系来进行退款`。
- 后端错误仍通过 `extractI18nErrorMessage()` 兜底映射，保证 API 直接返回时也展示同一句话。

### 不做项

- 不自动软删除旧套餐。
- 不自动把旧套餐余额折算到新套餐。
- 不改用户当前已有的双 active 数据；该数据需在退款后单独处理。
- 不改变自动 Key 的有效订阅选择逻辑。

## 3876129758@qq.com 退款参考

- 29 元订阅 `user_subscriptions.id=76` 开始：`2026-07-05 14:59:28.328526` 东八区。
- 59 元订阅订单 `payment_orders.id=143` 完成：`2026-07-08 15:53:48.749760` 东八区。
- 从旧订阅开始到新订阅完成：`3 天 00:54:20`，约 `3.04` 天。
- 从旧订阅开始到旧订阅最后一次 API 使用：`2 天 23:51:08`，约 `2.99` 天。
- 旧订阅实际 usage 记录：`480` 条，累计内部 USD 用量成本 `67.0562527500`。

退款口径建议按“已使用约 3.04 天”处理；如采用自然日口径，则覆盖 2026-07-05、07-06、07-07、07-08 四个日期，其中首尾为非完整日。
