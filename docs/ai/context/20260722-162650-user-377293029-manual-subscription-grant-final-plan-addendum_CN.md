# 377293029@qq.com 手动补发最终执行修正

## 最终执行口径

- 以今天线上成功支付后新生成的权益周期形态为准，而不是历史迁移后的旧权益形态。
- 实际补发字段：
  - `period_days=28`
  - `daily_limit_usd=NULL`
  - `weekly_limit_usd=58`
  - `period_total_quota_usd=232`
  - `quota_window_unit='week'`
  - `quota_window_days=7`

## 原因

- 今天 10:35 后的新支付权益周期已经按 28 天周窗口生成，`daily_limit_usd` 可为空。
- API 鉴权和额度扣减以周窗口权益为准，强行写旧日额度快照会让手动补发记录偏离当前服务层新发放形态。
