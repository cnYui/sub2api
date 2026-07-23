# 377293029@qq.com 手动补发计划修正

## 修正点

- 对比线上成功的 29 元套餐权益周期后，补发时不使用 `daily_limit_usd=NULL`。
- 实际写入：
  - `daily_limit_usd=15`
  - `weekly_limit_usd=58`
  - `period_total_quota_usd=232`
  - `quota_window_unit='week'`
  - `quota_window_days=7`

## 原因

- 当前公共 Codex 套餐已经以周窗口额度为主，但历史兼容字段仍由 Dashboard、退款 quote 或部分旧读模型引用。
- 保留 `daily_limit_usd=15` 可与现有 `group_id=2 / codex-pool-19-usd` 的权益事实形态一致，避免补发用户成为异常形态。
