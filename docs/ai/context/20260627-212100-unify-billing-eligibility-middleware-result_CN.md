# 认证中间件统一计费准入修复结果

## 结果

已修复“订阅每日额度用完后，OpenAI/GPT 流量卡无法接管”的前置拦截问题。

## 改动

- `backend/internal/server/middleware/api_key_auth.go`
  - 认证中间件仍加载订阅并校验订阅存在、过期、暂停等状态。
  - `ErrDailyLimitExceeded` / `ErrWeeklyLimitExceeded` / `ErrMonthlyLimitExceeded` 不再在中间件直接 429。
  - 非订阅模式余额为 0 不再在中间件直接 403。
  - 最终额度、余额、流量卡兜底、RPM/API Key 限额统一由 handler 中的 `BillingCacheService.CheckBillingEligibility()` 判断。
- `backend/internal/server/middleware/api_key_auth_google.go`
  - Google 风格入口同步上述行为，避免 `/v1beta` 保留旧拦截。
- `backend/internal/server/middleware/api_key_auth_test.go`
  - 更新订阅超限测试：中间件应放行到下游，并保留 subscription context。
  - 新增零余额放行测试。
- `backend/internal/server/middleware/api_key_auth_google_test.go`
  - 更新 Google 入口零余额和订阅超限测试。
- `backend/internal/service/traffic_pack_test.go`
  - 增加后扣命令测试：流量卡计费不应落到余额或订阅成本。
  - 增加边界测试：订阅本次请求会越过日限额且有 OpenAI 流量卡时，应切换到流量卡。

## 验证

```bash
go test -count=1 -tags=unit ./internal/server/middleware
```

结果：

```text
ok github.com/Wei-Shaw/sub2api/internal/server/middleware 0.466s
```

```bash
go test -count=1 -tags=unit ./internal/service
```

结果：

```text
ok github.com/Wei-Shaw/sub2api/internal/service 88.061s
```

## 后续运行态动作

代码修复后仍需重新构建/部署公网候选容器，当前运行中的 `sub2api-candidate` 不会自动包含本地代码改动。
