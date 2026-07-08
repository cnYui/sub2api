# 3876129758@qq.com 当前订阅用量清零与真实请求计划

时间：2026-07-08 17:40 JST

## 目标

- 优先让 `3876129758@qq.com` 当前 59 元套餐可用且显示为新额度。
- 清零当前 59 元套餐对应订阅的用量计数字段。
- 使用该用户当前 active 自动 API Key 真实请求 `https://api.aaccx.pw/v1/responses`。
- 核对新请求是否实际扣到 59 元套餐对应订阅。

## 边界

- 不暴露完整 API Key。
- 写库前备份公网候选 Postgres。
- 只修改 `user_subscriptions.id=90 / codex-pool-49-usd` 的用量计数字段，不改订单、Key、余额、旧 29 元订阅和流量卡。
- 本轮不构建镜像、不部署、不重启服务，除非复核证明缓存阻塞请求。

## 预期

- 清零后 `daily_usage_usd/weekly_usage_usd/monthly_usage_usd` 为 0，窗口仍保持当前周期。
- 真实请求返回 200，新增 `usage_logs`，`subscription_id=90`，`group_id=4 / codex-pool-49-usd`。
