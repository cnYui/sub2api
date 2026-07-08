# 东八区订阅日用量自动刷新设计

时间：2026-07-08

## 背景

当前订阅日用量刷新主要发生在 API 计费入口：请求进入 `CheckBillingEligibility` 后发现窗口过期，就写库推进 `daily_window_start` 并清零 `daily_usage_usd`。这能兜住活跃用户，但不是东八区 00:00 后对所有 active 订阅的真实刷新。

只读审计已确认：公网 18084 存在大量 active 订阅窗口仍停留在前一天，虽然 `/subscriptions` 和管理列表会在返回前做展示归一化，但数据库事实并没有全量刷新。`/v1/usage` 还存在自动 Key 不支持和窗口未归一化的问题。

本设计采用新的口径：删除 API 入口写库式惰性刷新，改为东八区 00:00 后后台任务主动推进所有 active 订阅；真实扣费按请求完成并写入 `usage_logs` 的时间归属额度。

## 已确认决策

- 删除 `CheckBillingEligibility` 内的 DB 写入式惰性刷新：API 请求进入计费准入时，不再因为窗口过期而清零或推进订阅表窗口。
- 新增东八区 00:00 后台任务：任务负责主动推进所有 active 订阅的 daily window。
- 保留完成时记账窗口自愈：请求完成后真实扣费时，按完成时间计算东八区当天窗口，确保 00:00 后完成的请求计入今天额度。
- 修复 `/v1/usage`：自动 Key 能查询 usage，返回前做窗口归一化，前端只展示今天真实窗口。
- 后台任务必须有重试、stale 监控和告警，避免任务失败后用户额度长期停留在昨天。

## 目标

- 东八区每天 00:00 后，所有未删除且未过期的 active 订阅都能被后台任务推进到当天窗口。
- 跨零点请求按完成时计费：23:59 发起、00:01 完成并写日志的请求，计入今天额度。
- API 入口不再承担“刷新所有用户窗口”的副作用，避免准入路径做写库清零。
- 展示口径统一：用户订阅页、管理订阅页、Key Usage `/v1/usage` 都不把昨天用量混入今天。
- 任务可重复执行，失败可恢复，多实例部署不会重复破坏数据。

## 非目标

- 不改变订阅额度模型本身，`user_subscriptions.daily_usage_usd` 仍是准入判断的快速事实字段。
- 不把每次准入判断改成实时聚合 `usage_logs`，避免每个 API 请求都打重聚合查询。
- 不改变流量卡、余额、返利等其他计费链路。

## 核心数据口径

订阅日用量以东八区自然日为窗口：

```text
today_start = Asia/Shanghai 当天 00:00:00
today_end   = today_start + 24h
```

真实用量归属以完成时为准：

```text
usage_billing_time = usage_logs.created_at
```

因此：

- `usage_logs.created_at >= today_start AND < today_end` 的订阅日志属于今天。
- 00:00 前发起但 00:00 后完成的请求属于今天。
- 失败请求不写成功用量日志，不进入订阅额度。

## 后台任务设计

新增订阅窗口校准任务，例如 `SubscriptionUsageWindowScheduler`，由 server 启动时注册。

任务行为：

- 每天东八区 00:00 后触发，建议首轮在 `00:00:05` 到 `00:01:00` 之间执行，避免时间边界抖动。
- 启动时如果发现当前 active 订阅存在 stale daily window，也执行一次补偿扫描。
- 使用 Redis `SETNX` leader lock，Redis 不可用时降级使用 PostgreSQL advisory lock，避免多副本同时执行。
- 分批扫描 active subscriptions，避免一次事务锁住过多行。
- 对 daily window 早于今天或为 NULL 的订阅，推进到今天窗口。
- 写入后失效对应订阅 Redis billing cache。

单条订阅更新口径：

```text
如果 daily_window_start IS NULL 或 daily_window_start < today_start：
  daily_window_start = today_start
  daily_usage_usd = SUM(usage_logs.total_cost)
    WHERE usage_logs.subscription_id = 当前订阅
      AND usage_logs.created_at >= today_start
      AND usage_logs.created_at < 执行时刻
```

说明：

- 使用今天已完成日志聚合，而不是无条件清 0，是为了保留 00:00 后已经完成并写入的请求。
- 已经是今天窗口的订阅不覆盖，避免后台任务与实时扣费并发时把刚累加的值回写掉。
- 任务可重复执行：过期行推进后再次执行不会再次清零。

## 完成时记账自愈

删除 API 入口写库刷新后，完成时扣费必须承担窗口归属责任。

在订阅扣费写入处统一实现：

```text
now = 请求完成并准备写 usage_logs/扣费的时间
today_start = Asia/Shanghai 当天 00:00:00

如果订阅 daily_window_start IS NULL 或 < today_start：
  daily_window_start = today_start
  daily_usage_usd = 本次费用

如果订阅 daily_window_start = today_start：
  daily_usage_usd += 本次费用
```

weekly/monthly 同理按现有窗口规则推进，但本次重点是 daily。

这个逻辑不是 API 入口惰性刷新。它属于扣费事务的一部分，保证账务事实正确。没有它，00:00 后完成的请求可能继续累加到昨天窗口，后台任务再怎么跑也会留下竞态。

## 准入判断变化

`CheckBillingEligibility` 不再写库刷新窗口。

准入时如果读到 stale daily window：

- 不做 DB 清零。
- 推荐准入口径采用只读归一化：如果窗口早于今天，则本次准入视为今天用量 0，但不写库；后续完成时扣费会把窗口推进到今天。

这样可以避免后台任务短暂失败时，00:00 后用户仍被昨天已用额度拒绝，同时保持“API 入口不写库刷新”的边界。

## `/v1/usage` 与前端展示

修复范围：

- 自动 Key 的 endpoint policy 增加 `/v1/usage`，或在 usage handler 内显式走 effective group 解析。
- `/v1/usage` 返回订阅块前做窗口归一化。
- 如果订阅窗口早于今天，展示用量为 0 或今天 `usage_logs` 聚合值，不能直接返回昨天的 `daily_usage_usd`。
- 用户订阅页和管理订阅页继续复用已有展示归一化逻辑。

展示事实源优先级：

- 今天窗口已推进：返回 `user_subscriptions.daily_usage_usd`。
- 窗口 stale：返回只读归一化结果，不把历史窗口用量展示为今天。
- Key 为自动 Key：先解析 effective group/subscription，再生成 usage 响应。

## 失败、重试与告警

后台任务失败不能静默。

需要记录：

- 任务开始、成功、失败、耗时、扫描数量、更新数量。
- Redis lock / DB advisory lock 获取失败。
- stale active subscription 数量。
- 校准后仍 stale 的订阅样本，日志中不得包含完整 API Key 或敏感 token。

重试策略：

- 00:00 首次失败后短间隔重试，例如 1 分钟、5 分钟、15 分钟。
- server 启动时补偿扫描，防止午夜进程重启错过任务。
- 后台巡检可以每小时只读检查 stale 数量，超过 0 触发告警或系统日志。

## 并发边界

关键竞态是后台校准与完成时扣费同时发生。

约束：

- 完成时扣费必须在同一事务中写 `usage_logs` 和更新订阅用量，或保持当前架构下的原子扣费语义。
- 后台任务只更新 `daily_window_start < today_start OR IS NULL` 的过期行。
- 如果完成时扣费先推进到今天，后台任务不覆盖该行。
- 如果后台任务先推进到今天，本次扣费看到今天窗口后正常累加。

这个策略会牺牲“后台任务强制覆盖今天已推进行”的修正能力，但换来不丢并发扣费。今天窗口内的错账应由单独的只读审计或人工校准处理，不在 00:00 自动任务里覆盖活跃行。

## 需要修改的代码边界

预计涉及：

- `backend/internal/service/billing_cache_service.go`
  - 删除或旁路 `CheckBillingEligibility` 中的 DB 写入式窗口刷新。
  - 保留只读归一化判断，避免 00:00 后短时误拒。
- `backend/internal/repository/usage_billing_repo.go`
  - `incrementUsageBillingSubscription()` 改为完成时窗口自愈累加。
- `backend/internal/repository/user_subscription_repo.go`
  - 增加后台任务批量推进/校准 active 订阅的方法。
- `backend/internal/service/*`
  - 新增订阅日窗口后台任务服务，复用现有 ops aggregation 的锁模式。
- `backend/cmd/server/wire.go`
  - 注册后台任务启动和停止。
- `backend/internal/server/middleware/effective_group.go`
  - 支持自动 Key 的 `/v1/usage`。
- `backend/internal/handler/gateway_handler.go`
  - usage 响应做窗口归一化。
- 前端 Key Usage 页面
  - 原则上以后端响应修复为主；只在必要时调整空态/错误态文案。

## 测试验收

后端测试：

- `CheckBillingEligibility` 遇到 stale daily window 不写库。
- stale window 的准入只读归一化不会因昨天用满而拒绝今天请求。
- 00:00 后完成扣费时，stale window 被推进到今天，`daily_usage_usd = 本次费用`。
- 今天窗口内继续扣费时，`daily_usage_usd += 本次费用`。
- 后台任务对 stale active subscription 聚合今天 `usage_logs` 并推进窗口。
- 后台任务不覆盖已经是今天窗口的订阅。
- 多实例 lock 下只有一个 worker 执行。
- 自动 Key 可以访问 `/v1/usage` 并返回订阅 usage 块。

运行态验收：

- 发布前备份 Postgres/Redis。
- 构造或选择一个 stale active 订阅，执行后台任务后窗口变为今天。
- 构造跨零点或模拟完成时在今天的扣费，确认写入今天窗口。
- 查询 `/subscriptions`、管理订阅页、`/v1/usage`，确认都不展示昨天用量。
- 检查任务日志无敏感信息，stale active subscription 数量为 0。

## 结论

最终方案是：

```text
删除 API 入口写库刷新
+ 东八区 00:00 后台主动校准
+ 完成时扣费窗口自愈
+ /v1/usage 自动 Key 与展示归一化
+ stale 监控和失败重试
```

这样既满足“按标准时间自动刷新所有 active 订阅”，也保证跨零点请求按完成时计入今天额度，并避免 API 准入路径继续承担写库刷新副作用。
