# Dashboard 套餐额度实时展示设计

## 已确认目标

Dashboard 不再把“实际扣费 / 标准价”作为核心消费展示，改为面向用户的额度口径：

- 有有效套餐：`今日使用 / 每日套餐额度`，以及 `本订阅周期使用 / 本订阅周期理论额度`。
- 本订阅周期理论额度为发放时的每日额度快照乘以周期天数；正常购买和续费当前为 30 天。
- 同套餐提前续费不切换当前展示周期，新周期从旧周期 `expires_at` 开始，旧周期结束后再切换。
- 换套餐必须先退款，因此同一用户最多一个有效套餐，不需要合并不同套餐额度。
- 流量卡、余额和套餐消费都计入分子；分子允许大于分母，不截断、不钳制。
- 没有有效套餐时，今日显示 `今日消费 / $0`，第二项显示 `最近 30 天消费 / $0`。
- 页面可见时每 15 秒刷新额度数据，恢复到前台时立即刷新。

## 根因与数据边界

现有 `user_subscriptions` 只保存可变的 `starts_at`、`expires_at` 和窗口累计值。同套餐提前续费只延长 `expires_at`，无法可靠还原当前正在生效的 30 天周期；`monthly_usage_usd` 也只累计订阅计费，不能覆盖流量卡。

当前有效订阅中，部分来自支付订单，另有历史迁移、兑换码和人工发放记录；其中存在 60、150、3650 天等长期记录。不能把这些记录强行拆成看似精确的 30 天订单周期。

## 目标模型

新增不可变表 `subscription_entitlement_periods`，每条记录表示一次实际发放的套餐权益：

- `id`、`user_id`、`subscription_id`、`group_id`
- `source_type`、`source_id`，用于支付订单、兑换码、后台发放、默认发放与历史回填的幂等关联
- `starts_at`、`expires_at`、`days`
- `daily_limit_usd`，发放时的额度快照；未限额时保留空值，Dashboard 对外显示分母 `0`
- `status`，至少包含 `active` 与 `revoked`
- 审计时间字段，以及来源唯一约束

权益发放必须与订阅更新、订单关联和审计在同一数据库事务内落库。需要把当前独立事务的订阅续期调用收敛到可复用的事务边界，避免订阅已延长但周期事实缺失，或反向出现孤立周期。

周期生成规则：

- 新建或已过期后续订：`[now, now + days)`。
- 同套餐提前续费：`[old_expires_at, old_expires_at + days)`。
- 兑换码、后台和默认发放复用同一规则，按实际 `ValidityDays` 写入。
- 退款撤销权益时，在同一事务将未失效的对应周期标为 `revoked`；退款仍处于人工核验或未知状态时不改变周期。

## 实时消费读模型

Dashboard 读接口扩展为单次返回原有统计和新的 `quota` 块，避免前端拼接多个来源。

消费分子使用 `actual_cost`，按用户聚合：

1. 优先读取 `usage_facts` 中 `pending`、`settling`、`settled`、`debt` 的 payload 用量，按 `completed_at` 划分时间窗。
2. 对没有同一 `(request_id, api_key_id)` usage fact 的历史 `usage_logs` 进行补集，按 `created_at` 划分时间窗。
3. 不按 `billing_type`、`subscription_id` 或流量卡来源过滤，因此所有成功消费都进入分子；两源去重避免结算后重复计入。

有当前有效周期时：

- 今日窗口：服务端 `Asia/Shanghai` 当天 `[00:00, next 00:00)`。
- 本期窗口：`[period.starts_at, min(now, period.expires_at))`。
- 分母：`daily_limit_usd` 与 `daily_limit_usd * days`。

无当前有效周期时：

- 今日窗口保持当天，分母为 `0`。
- 第二窗口为 `[now - 30 days, now]`，分母为 `0`。

## 历史回填

- 只为可由原始 `starts_at/expires_at` 和单一发放事实明确证明的历史订阅回填 `legacy_backfill` 周期。
- 不能证明当前 30 天边界的历史有效订阅不伪造周期；在首次精确发放前，Dashboard 使用“最近 30 天使用 / 每日额度 x 30”的兼容口径，并由 API 明确返回 `period_mode=rolling_30d_legacy`。
- 新购买、续费、兑换码和后台发放后即改用精确周期口径。

## 前端与刷新

- 替换当前“今日消费”卡片中的 `actual / standard` 双金额，展示两行额度值；使用 i18n 文案，金额保持 USD 固定四位小数。
- 保留其他请求、Token、性能卡片。
- 仅轮询额度统计，不重复轮询图表和最近用量；可见页面每 15 秒请求一次，`visibilitychange` 和窗口 `focus` 时立即刷新，卸载时清理计时器。
- 不引入 WebSocket；数据库 durable fact 已保证刷新后的数值包含响应已完成但尚未结算的成功请求。

## 验证范围

- migration 与 repository：周期创建幂等、提前续费连续性、退款撤销、各发放来源、历史回填与降级。
- read model：usage fact 与 usage log 去重、pending/debt、流量卡超过套餐额度、无套餐、跨东八区零点。
- service/handler：认证隔离、唯一有效周期、错误与空数据响应。
- 前端：有套餐、无套餐、流量卡超额、历史降级、15 秒轮询、前后台切换和清理。

## 非目标

本设计不改变套餐购买约束、退款金额或状态机、实际扣费、流量卡预留、余额逻辑，也不修改当前运行态或部署配置。
