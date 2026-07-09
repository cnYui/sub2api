# 购买页弹出“需要先和管理员联系来进行退款”排查

## 结论

当前购买页无法继续购买订阅不是支付通道异常，而是命中了 2026-07-08 新增的“同一用户只能有一个未结束 active 订阅”的业务保护。

截图中的当前登录用户为 `2799523972@qq.com`。该用户在公网候选库仍有一个未删除、未过期的 active 订阅：

- `users.id=31`
- `user_subscriptions.id=71`
- `group_id=9 / codex-pool-69-usd`
- 对应套餐：`subscription_plans.id=7 / 79 元订阅池`
- 状态：`active`
- 开始时间：`2026-07-03 10:35:29+08`
- 到期时间：`2026-08-02 10:35:29+08`
- `deleted_at=NULL`
- 备注：`manual repair to 79 yuan plan on 2026-07-03; preserve traffic credits`

因此前端点击任意订阅套餐时会直接弹出 `需要先和管理员联系来进行退款`；即使绕过前端，后端创建订阅订单也会返回 `ACTIVE_SUBSCRIPTION_EXISTS`。

## 证据

- Chrome 当前购买页可见 DOM 显示用户菜单：`2799523972@qq.com`，顶部订阅徽标为 `1`。
- `frontend/src/views/user/PaymentView.vue` 中 `hasActiveSubscriptionForPurchase` 只判断是否存在任意 `status === 'active'` 的订阅；存在时 `showActiveSubscriptionPurchaseBlocked()` 弹出 `payment.errors.ACTIVE_SUBSCRIPTION_EXISTS`。
- `frontend/src/i18n/locales/zh.ts` 中该错误码中文文案为：`需要先和管理员联系来进行退款`。
- `backend/internal/service/payment_order.go` 中 `validateSubOrder()` 调用 `ensureNoActiveSubscriptionForPurchase()`；后者通过 `ListActiveByUserID()` 查到任意 active 订阅就返回 `ErrActiveSubscriptionExists`。
- 数据库只读查询确认：
  - `user_subscriptions.id=71` 为当前有效订阅。
  - 旧订阅 `user_subscriptions.id=49 / codex-pool-19-usd` 虽仍是 `status=active`，但 `deleted_at=2026-06-27 21:02:42+08`，不计入当前 active。
  - 该用户最近订阅订单均为取消状态；当前 79 元订阅来自人工修复记录，不是本轮新订单。

## 判断

如果目标是“阻止用户在已有套餐未处理退款前重复购买”，当前行为符合既定规则。

如果目标是允许用户续费、升级或补差购买，那么当前规则过于粗：它会把同组续费、跨档升级和重新购买全部拦掉。后续需要单独设计“续费/升级/补差”业务流，而不是直接移除这个保护。

本轮未修改代码、未修改数据库、未重启容器。
