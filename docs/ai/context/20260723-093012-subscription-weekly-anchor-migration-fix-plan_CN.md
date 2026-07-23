# 订阅周窗口锚点迁移修复计划

时间：2026-07-23

## 问题

公共 Codex 订阅已经按周展示，但 `admin/subscriptions` 对部分历史迁移订阅显示 `0` 用量。历史批次的 `user_subscriptions.weekly_anchor_at` 和 `weekly_window_start` 已正确统一为 `2026-07-22 00:00 +08`，同时保留了当周已用额度。

最新周窗口实现改为直接使用当前 `subscription_entitlement_period.starts_at` 作为锚点，以防止续费权益段继承旧段窗口。历史权益段的 `starts_at` 是原始购买时间，和迁移锚点不同；窗口起点不一致时，`RollingWeeklyUsageUSD` 按保护逻辑返回 0，造成管理端和用户端把实际周用量显示为空，并可能在首次请求时错误清零。

## 目标规则

- 历史迁移批次：固定从 `2026-07-22 00:00 +08` 开始，每 7 天刷新一次，保留现有当周已用额度。
- 后续新订单：从实际购买时刻开始，每 7 天刷新一次。
- 提前续费：新权益段从其自身起点开始，不继承旧权益段的锚点或已用额度。
- 权益段到期后未使用额度清零；本次不回填或重算历史 `usage_facts`、`usage_logs`。

## 方案

1. 为 `subscription_entitlement_periods` 增加可空 `quota_window_anchor_at`，作为该权益段的周窗口不可变锚点。
2. 新增前向迁移：
   - 为当前生效的周额度权益段回填 `COALESCE(user_subscriptions.weekly_anchor_at, entitlement.starts_at)`；这会精确保留历史迁移批次的午夜锚点和已用额度。
   - 对未来权益段或没有订阅行锚点的记录使用权益段自身 `starts_at`，避免续费段错误继承历史锚点。
3. 新订单、后台发放和续费创建权益段时写入 `quota_window_anchor_at = entitlement.starts_at`。
4. 周窗口计算优先使用权益段锚点；只在旧记录未回填时降级到权益段起点。
5. 补充单元测试覆盖历史迁移锚点保留已用额度和新续费段独立锚点，并更新迁移 schema 断言。

## 运行态边界

- 写入本地运行库前先备份 PostgreSQL；迁移完成后核对锚点、窗口起点和周用量，并通过管理端读模型验证。
- 不修改已确认正确的历史批次锚点 `2026-07-22 00:00 +08`。
