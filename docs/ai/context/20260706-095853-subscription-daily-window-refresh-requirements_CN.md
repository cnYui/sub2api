# 订阅套餐日额度刷新问题需求文档

## 背景

用户 `daleselaji@gmail.com` 在 2026-07-06 反馈 API 请求失败：

- URL：`https://aaccx.pw/responses`
- request id：`be711a86-2855-48ef-b3a4-b3355ad9901e`
- 错误：`unexpected status 403 Forbidden: daily usage limit exceeded`

日志确认该请求在 Sub2API 网关计费资格检查阶段被拒绝：

- `user_id=60`
- `api_key_id=76`
- `group_id=4`
- `group_name=codex-pool-49-usd`
- `error=DAILY_LIMIT_EXCEEDED`
- HTTP `403`

该用户 2026-07-06 自然日成功用量为 0，但订阅记录仍保留 2026-07-05 的超限用量，导致请求被拒绝。

## 当前影响面

排查时间：`2026-07-06 08:58:00+08`

运行环境确认：

- 应用容器 `TZ=Asia/Shanghai`
- 应用容器系统时间：`+0800`
- PostgreSQL `TimeZone=Asia/Shanghai`
- PostgreSQL `date_trunc('day', NOW())=2026-07-06 00:00:00+08`

因此“东八区 24 点刷新”的时区配置仍然存在，问题不是时区配置丢失，而是订阅日窗口刷新没有在 API 请求入口稳定触发。

公网库统计：

- 带日额度的 active 订阅：43 条
- `daily_window_start` 已早于今天 00:00 的订阅：37 条
- 过期日窗口且 `daily_usage_usd > 0` 的订阅：37 条
- 过期日窗口且已超过日额度、可能直接触发 403 的订阅：3 条
- 当前日窗口内已超过日额度的订阅：0 条

当前仍处于过期日窗口且超过日额度的订阅：

| subscription_id | email | group | daily_limit_usd | stale_daily_usage_usd | daily_window_start | OpenAI 流量卡 |
|---:|---|---|---:|---:|---|---:|
| 72 | `859591608@qq.com` | `codex-pool-19-usd` | 19 | 19.1392950000 | 2026-07-05 00:00:00+08 | 0 |
| 64 | `cnfoxian@gmail.com` | `codex-pool-19-usd` | 19 | 19.0282566500 | 2026-06-27 00:00:00+08 | 19.8934673500 |
| 2 | `milesyang987@gmail.com` | `codex-pool-19-usd` | 19 | 19.0075800000 | 2026-07-05 00:00:00+08 | 19.2678460000 |

说明：

- `daleselaji@gmail.com` 已被手动重置日窗口并验证 API 请求恢复，因此不再出现在上述 3 条里。
- 有流量卡的用户在套餐超限后可能走流量卡兜底，不一定表现为不可用，但套餐日额度仍是错误的旧窗口值。
- 未超限但窗口过期的 34 条订阅也受影响：它们不会立即 403，但今天可用额度会被昨天或更早的用量错误占用。

## 根因

现有系统有两套与窗口刷新相关的逻辑：

1. `SubscriptionService.CheckAndResetWindows()`
   - 可以把过期的 `daily_window_start` 重置到当天 00:00。
   - 会把 `daily_usage_usd` 归零。
   - 但它只在部分订阅服务读取/维护路径中触发。

2. API 网关计费资格检查
   - 请求进入网关后调用 `BillingCacheService.CheckBillingEligibility()`。
   - 订阅模式下继续调用 `checkSubscriptionEligibility()`。
   - 该路径通过 `GetSubscriptionStatus()` 读取 Redis 缓存或 DB 中的 `daily_usage_usd`。
   - 该路径不检查 `daily_window_start` 是否过期，也不触发 `CheckAndResetWindows()`。

因此当订阅在前一天已经接近或超过额度后，跨过东八区 00:00，如果没有其他路径触发窗口维护，API 请求仍会拿旧的 `daily_usage_usd` 做限额判断。

用户侧看到“今天自然日用量为 0”，但 API 入口仍按旧窗口超限值拦截，两者同时成立。

## 目标

修复后必须满足：

1. 所有 OpenAI/GPT API 请求进入计费资格检查时，都按东八区自然日窗口判断套餐日额度。
2. 当 `daily_window_start < 今日 00:00+08` 时，请求入口必须在限额判断前完成日窗口刷新。
3. 刷新必须持久化到 DB，不能只在内存里把用量当作 0。
4. 刷新后必须失效或更新 Redis billing subscription cache，避免继续读旧值。
5. 多个并发请求同时跨日进入时，只能产生一致的窗口结果，不能出现重复重置、旧值回写或用量丢失。
6. 修复不能破坏已有流量包兜底逻辑：套餐正常可用时走套餐，套餐真实超限时才走流量包。
7. 修复不能改变周/月窗口语义；如果发现周/月也有同类 stale window，应按同一模式处理。

## 非目标

- 不调整套餐价格、日额度数值或分组绑定。
- 不改变用户 API Key 的自动分组解析规则。
- 不修改自然日展示口径；展示层只应与计费层使用同一窗口事实源。
- 不批量清空所有历史用量，除非作为一次性运行态修复并先备份。

## 推荐方案

推荐在 `BillingCacheService` 的订阅资格检查路径中做窗口刷新，作为唯一强一致入口。

### 方案 A：在网关每个 handler 调用 `SubscriptionService.CheckAndResetWindows()`

优点：

- 复用现有订阅服务方法。
- 改动直观。

缺点：

- OpenAI responses、chat、images、embeddings、Gemini 等多个入口都需要补调用，容易漏。
- 网关和订阅服务耦合更深。
- Redis billing cache 仍需要额外处理。

不推荐作为主方案。

### 方案 B：在 `BillingCacheService.checkSubscriptionEligibility()` 内部统一刷新

优点：

- 所有 API 入口都已经经过 `CheckBillingEligibility()`，覆盖面完整。
- 限额判断、流量卡兜底、RPM 检查仍保持一个入口。
- 可集中处理 DB 条件更新、缓存失效和并发 singleflight。

缺点：

- `subscriptionCacheData` 当前不包含窗口起点，需要扩展缓存结构。
- 需要小心旧 Redis cache 兼容和并发请求。

推荐采用。

### 方案 C：只在读取 DB 时逻辑上把过期窗口用量视为 0

优点：

- 改动最小。

缺点：

- 如果不持久化 DB，后续 `IncrementUsage()` 会在旧 `daily_usage_usd` 上继续累加。
- Redis cache 可能继续缓存旧用量。
- 只能缓解资格判断，不能修复事实源。

不推荐。

## 详细需求

### R1. 订阅缓存数据补齐窗口字段

`subscriptionCacheData` 和 `SubscriptionCacheData` 需要包含：

- `DailyWindowStart`
- `WeeklyWindowStart`
- `MonthlyWindowStart`

Redis `billing:sub:{userID}:{groupID}` hash 需要新增字段：

- `daily_window_start`
- `weekly_window_start`
- `monthly_window_start`

兼容要求：

- 旧缓存缺少窗口字段时，视为缓存不可用于窗口判断，应回源 DB 并重建缓存。
- 发布后无需手动清 Redis；旧缓存 TTL 只有约 5 分钟，但代码仍要安全处理旧字段缺失。

### R2. 请求入口执行过期窗口刷新

在 `BillingCacheService` 内新增统一逻辑：

1. 读取订阅状态时拿到窗口起点。
2. 使用 `timezone.StartOfDay(time.Now())` 计算东八区今日 00:00。
3. 如果 `DailyWindowStart == nil`，按现有首次激活语义处理窗口。
4. 如果 `DailyWindowStart < 今日 00:00`，在限额比较前执行 DB 条件更新：
   - `daily_usage_usd=0`
   - `daily_window_start=今日 00:00`
   - `updated_at=NOW()`
   - 条件必须包含 `daily_window_start < 今日 00:00`，避免并发重复覆盖。
5. 更新成功或发现其他请求已更新后，重新读取订阅状态并重建 Redis cache。
6. 再使用刷新后的 `daily_usage_usd` 判断是否超过日额度。

### R3. 周/月窗口保持一致

如果当前订阅分组存在周/月额度，也需要同一机制：

- `weekly_window_start < timezone.StartOfWeek(now)` 时重置 `weekly_usage_usd`。
- `monthly_window_start + 30天 <= now` 时重置 `monthly_usage_usd`。

当前主要问题是日额度，但修复不应留下同类周/月隐患。

### R4. 并发与缓存一致性

跨日后多个请求同时进入时：

- 使用 DB 条件更新保证只有过期窗口会被重置。
- 可使用 `singleflight` 以 `subscription-window:{userID}:{groupID}` 合并同一用户分组的窗口刷新。
- 刷新成功后必须删除或重建 `billing:sub:{userID}:{groupID}`。
- 如果 Redis 删除失败，不能阻断请求，但要记录 warning；本次请求必须使用刷新后的 DB 数据继续判断。

### R5. 错误状态与 HTTP 语义

现状中 `DAILY_LIMIT_EXCEEDED` 经过 handler 映射为 HTTP 403。修复窗口刷新后，只有真实当前窗口超限才允许返回该错误。

需求：

- 窗口过期导致的旧用量不得返回 `DAILY_LIMIT_EXCEEDED`。
- 真实当前窗口超限时，现有错误码可先保持不变。
- 后续可单独评估是否把套餐限额从 403 调整为 429 并附带 `Retry-After`，但不纳入本次必须范围。

## 验收标准

### 数据验收

构造一条 active 订阅：

- `daily_limit_usd=19`
- `daily_usage_usd=19.5`
- `daily_window_start=昨天 00:00+08`

调用 API 请求后：

- 请求不应因 `DAILY_LIMIT_EXCEEDED` 被拒绝。
- DB 中该订阅应变为：
  - `daily_window_start=今天 00:00+08`
  - `daily_usage_usd=本次请求实际费用`
- Redis `billing:sub:{userID}:{groupID}` 应包含新的窗口起点和新的 daily usage。

### 用户场景验收

1. 昨天用满套餐、今天 00:00 后第一次 API 请求：
   - 应正常通过。
   - 应从今天新窗口开始扣套餐用量。

2. 今天真实用满套餐：
   - 应继续返回限额错误。
   - 如果用户有 OpenAI/GPT 流量卡，仍应按现有逻辑走流量卡兜底。

3. 多个并发请求在 00:00 后同时发起：
   - 不应出现有的请求用旧窗口 403、有的请求用新窗口 200 的不一致。
   - 最终 `daily_usage_usd` 应等于这些成功请求费用之和，不应丢失用量。

### 回归验收

需要覆盖：

- `BillingCacheService.CheckBillingEligibility()` 订阅日窗口过期重置。
- Redis cache 命中但缺少窗口字段时回源 DB。
- Redis cache 命中且窗口过期时刷新 DB 后继续通过。
- DB 条件更新并发只重置一次。
- 有流量包的套餐超限用户仍可兜底。
- 未过期窗口且真实超限仍拒绝。

## 一次性运行态处理建议

源码修复上线前，不建议批量手动清零所有 active 订阅，因为这会改变当前事实源。

如果需要临时缓解受影响用户：

1. 先备份公网库。
2. 只处理 `daily_window_start < 今日 00:00+08` 的 active 订阅。
3. 将这些订阅的 `daily_usage_usd` 置 0，`daily_window_start` 置今日 00:00。
4. 删除对应 `billing:sub:{userID}:{groupID}` Redis key。
5. 对至少一名高风险用户做真实 API 请求验证。

当前最需要人工关注的是无流量卡且已 stale 超限的 `859591608@qq.com`。

## 发布要求

- 先在本地或 preview 环境使用生产库克隆验证。
- 部署前备份公网 PostgreSQL 和 Redis。
- 部署后检查：
  - `api.aaccx.pw/health`
  - `aaccx.pw/responses` 真实请求
  - stale daily window 统计应在真实请求后下降
  - 日志中不应继续出现过期窗口导致的 `DAILY_LIMIT_EXCEEDED`

## 回滚要求

如果修复导致计费异常：

- 回滚应用镜像。
- 不回滚正常产生的新 `usage_logs`。
- 如有错误窗口重置，使用部署前 DB 备份和 `usage_logs` 差异做定向修复，不做全库覆盖。
