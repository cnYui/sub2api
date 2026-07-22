# 28 天订阅历史续费周窗口对齐结果

## 完成内容

- 公开 Codex 订阅继续保持 `28 天有效期 + 4 个完整 7 天窗口`；未改动套餐天数、支付订单快照或运行库。
- 当前滚动周窗口改为以当前 `subscription_entitlement_period.starts_at` 为锚点计算，不再以整条订阅最早的 `weekly_anchor_at` 截断新权益段。
- 历史 30 天权益段提前续费为 28 天时，旧段保留其自身尾段规则；新段从原到期时刻开始获得四个完整周窗口，旧 `weekly_usage_usd` 因窗口起点不同自动视为零并由既有维护逻辑重置。
- 用户仪表盘使用同一权益段起点计算周窗口，避免展示的刷新时间、限额和授权路径不一致。

## 新增回归

- 服务层：旧订阅锚点与新 28 天权益段起点不一致时，周窗口必须从新权益段起点开始，额度为完整周额度，旧窗口用量不得继承。
- 仓储集成：仪表盘在相同场景必须返回新权益段的周窗口起点、完整限额和零窗口用量。

## 验证

- `go test ./internal/service ./internal/repository -count=1`：通过。
- `go test -tags=integration ./internal/repository -run 'TestUsageLogRepository_(GetUserDashboardQuota|RollingWeeklyQuotaStartsAtCurrentEntitlementBoundary)' -count=1`：通过。
- `pnpm test:run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/SubscriptionsView.spec.ts src/utils/__tests__/subscriptionQuota.spec.ts`：通过，3 个文件、49 个测试。
- `git diff --check`：通过。

## 边界

- 本轮未修改本地数据库、容器、Nginx、支付提供商或公网链路。
- 历史权益段未做数据迁移；修复在代码发布后对后续授权、窗口维护和仪表盘读取生效。
