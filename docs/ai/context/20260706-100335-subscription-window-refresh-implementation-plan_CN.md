# 订阅窗口刷新本地实现计划

## 目标

- 参考 `docs/ai/context/20260706-095853-subscription-daily-window-refresh-requirements_CN.md`，修复 API 计费资格检查读取旧订阅窗口用量的问题。
- 修复范围限定在本地 `main` 分支代码，不触碰公网容器、数据库、Redis 或 nginx。

## 现状结论

- `BillingCacheService.checkSubscriptionEligibility()` 通过 `GetSubscriptionStatus()` 获取订阅状态后直接比较 `daily_usage_usd`、`weekly_usage_usd`、`monthly_usage_usd`。
- `SubscriptionCacheData` 当前只包含状态、过期时间、用量和版本，不包含 `daily_window_start`、`weekly_window_start`、`monthly_window_start`。
- `SubscriptionService.CheckAndResetWindows()` 可重置窗口，但不在 API 网关计费资格检查热路径稳定触发。
- `user_platform_quota` 缓存已有窗口字段和旧 schema 回源模式，可作为订阅缓存改造参考。

## 设计取舍

- 采用需求文档推荐的方案 B：在 `BillingCacheService` 订阅资格检查路径统一处理窗口刷新。
- 扩展 `SubscriptionCacheData` 和内部 `subscriptionCacheData`，缓存 hash 新增窗口字段；旧 Redis hash 缺窗口字段时视为 miss。
- 新增订阅仓储条件刷新方法：只在对应窗口已过期时把用量清零并推进窗口起点，避免并发请求重复覆盖。
- API 入口刷新后重新读取 DB 并同步重建 Redis 缓存；本次请求使用刷新后的 DB 数据继续限额判断。
- 日窗口使用 `timezone.StartOfDay(now)`，周窗口使用 `timezone.StartOfWeek(now)`，月窗口保持 30 天滚动语义。
- 不改展示层、套餐配置、流量包兜底、API Key 自动分组、错误码 HTTP 映射。

## TDD 步骤

1. 新增 service 单元测试，覆盖旧 daily 窗口超限在请求入口刷新后放行。
2. 新增 service 单元测试，覆盖缓存命中但缺窗口字段时回源 DB。
3. 新增 service 单元测试，覆盖当前窗口真实超限仍返回 `ErrDailyLimitExceeded`。
4. 新增 repository 单元或集成测试，覆盖条件刷新只在窗口过期时更新一次。
5. 运行目标测试确认失败后，再实现最小生产代码。
6. 实现后运行 `go test -count=1 -tags=unit ./internal/service ./internal/repository`；如仓储测试需要集成环境，再补跑对应 package 的可用测试。

## 风险

- 修改 `UserSubscriptionRepository` 接口会影响测试 stub，需要同步补齐默认 panic 实现。
- Redis 旧缓存兼容必须 fail safe：缺窗口字段不能继续用旧用量判断。
- 刷新和缓存重建应优先保证 DB 事实源正确；Redis 删除或写入失败只记录 warning，不阻断当前请求。
