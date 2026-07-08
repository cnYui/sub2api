# 3876129758@qq.com 双 active 订阅排查结果

时间：2026-07-08 17:18 JST

## 结论

`3876129758@qq.com` 同时存在 29 元旧套餐和 59 元当前套餐，不是迁移残留，也不是手动赠送；公网候选库记录显示该用户先后完成了两笔支付宝订阅订单：

- 2026-07-05 14:59:28 东八区：`payment_orders.id=124`，`plan_id=1 / 29 元订阅池`，`subscription_group_id=2 / codex-pool-19-usd`，支付金额 `29.29`，生成 `user_subscriptions.id=76`
- 2026-07-08 15:53:48 东八区：`payment_orders.id=143`，`plan_id=3 / 59 元订阅池`，`subscription_group_id=4 / codex-pool-49-usd`，支付金额 `59.59`，生成 `user_subscriptions.id=90`

两笔订单的 `payment_audit_logs` 都有 `ORDER_CREATED -> ORDER_PAID -> SUBSCRIPTION_ASSIGNED -> SUBSCRIPTION_SUCCESS` 链路，创建者为 `user:56`，支付回调来自 `easypay`。

## 根因

当前订阅履约逻辑是“同分组续期，异分组新增”：

- `PaymentService.fulfillSubscriptionOrderInTx()` 调用 `SubscriptionService.AssignOrExtendSubscription()`
- `AssignOrExtendSubscription()` 只按 `(user_id, group_id)` 查已有订阅
- 如果用户买的是不同套餐分组，例如从 29 元池换到 59 元池，不会自动暂停或软删除旧套餐，而是创建新的 active 订阅
- 数据库唯一约束也是 `(user_id, group_id) WHERE deleted_at IS NULL`，允许同一用户在不同订阅分组下同时 active

因此旧 29 元订阅仍 active 的直接原因是：用户在 2026-07-05 买过 29 元套餐，2026-07-08 买 59 元套餐时系统没有“升级替换旧套餐”的规则。

## 当前影响

- 该用户是当前公网库唯一一个拥有多个未过期 active 订阅的用户。
- 自动 Key 真实请求会由 `EffectiveGroupResolver` 在 active OpenAI 订阅里按日限额优先选择，当前会选 `codex-pool-49-usd / user_subscriptions.id=90`。
- 2026-07-08 真实请求 `usage_logs.id=66004` 已证明扣费走的是 59 元套餐对应订阅 `id=90`，未走旧 29 元订阅，也未走流量卡。

## 建议

如果业务定义是“升级套餐后只保留新套餐”，后续应改成购买订阅时按用户维度关闭其它 active OpenAI 订阅，或只关闭日限额更低的旧订阅；同时补一条一次性数据修正，把该用户旧 `user_subscriptions.id=76` 软删除或置为非 active。

如果业务定义允许叠加订阅，则现状是符合代码设计的，但管理台/用户页应明确展示“当前实际扣费套餐”和“其它未过期套餐”，否则会造成误解。
