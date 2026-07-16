# 流量卡预授权与 Debt Gate 修复结果

## 结论

本地分支 `codex/fix-traffic-credit-billing-gap` 已完成第二阶段修复：OpenAI 流量卡兜底不再以 `remaining_usd > 0` 作为最终放行条件，而是在进入上游前按最终出站请求、输出上限和价格快照做预算，并通过 PostgreSQL reservation 原子预留额度。

套餐到期后 API Key 仍保持 active；如果用户有足够有效流量卡，仍可继续请求。如果余额和订阅不可用且流量卡不足以覆盖最低有效预算，或存在未结清 traffic credit debt，请求会在上游前拒绝。

## 主要改动

- 新增 migration `165_traffic_credit_reservations.sql`：`user_traffic_credits.reserved_usd`、`traffic_credit_reservations`、`traffic_credit_reservation_items`、`usage_facts.reservation_id`。
- 新增 FEFO reservation planner 和 repository：使用 `remaining_usd - reserved_usd`、`FOR UPDATE`、唯一 `(request_id, api_key_id)` 防并发超卖和重复预留。
- 新增 OpenAI 预算估算器：识别 `max_output_tokens`、`max_completion_tokens`、`max_tokens`；缺少输出上限时按可支付额度注入对应硬上限；残额小于 `0.01 USD` 或预算不足时返回 `ErrTrafficCreditInsufficient`。
- OpenAI Responses、Chat Completions converted path、raw Chat Completions 在上游前接入 `OpenAIBillingAuthorization`，同一请求 failover 复用同一 reservation。
- 上游发送前标记 `dispatched`；发送前失败释放 reservation；transport unknown 标记 `unknown`，不会被 TTL 自动释放。
- usage fact 和 `UsageBillingCommand` 贯穿 `reservation_id`，结算时在同一事务内扣实际费用、释放差额，若实际费用超过预留且无额外可用额度则写 `debt`，不回滚 usage fact 或 usage log。
- debt gate：用户存在 OpenAI traffic credit debt 时，新的流量卡请求在预授权前拒绝。
- stale cleanup：`UsageFactWorker` 定时只释放未 dispatched 的过期 `reserved` reservation；`dispatched/unknown` 保留待人工或后续对账。
- 增加进程内指标快照 `SnapshotTrafficCreditReservationMetrics()`，记录预授权成功/拒绝、debt 阻断、unknown、actual > reserved、过期 reserved 释放数。

## 验证

均按用户要求串行执行，未并行跑 Go 测试：

```bash
GOMAXPROCS=2 go test -p=1 -parallel=1 -count=1 -tags=unit ./internal/config ./internal/service ./internal/handler -run 'TrafficCredit|Reservation|BillingAuthorization|UsageFact'
GOMAXPROCS=2 go test -p=1 -parallel=1 -count=1 -tags=integration ./internal/repository -run 'TrafficCredit|Reservation|UsageBilling|MigrationsRunner'
GOMAXPROCS=2 go test -p=1 -parallel=1 -count=1 -tags=unit ./internal/service ./internal/handler
GOMAXPROCS=2 go test -p=1 -parallel=1 -count=1 ./cmd/server
git diff --check
```

以上均通过。`backend/cmd/server/wire_gen.go` 已由 `go generate ./...` 重新生成。

## 未做事项

- 未部署、未修改运行态 PostgreSQL/Redis/Nginx/容器。
- 未开启生产 reservation 强制模式；配置仍应先按 shadow 观察，再 canary，再全量。
- 未补扣历史缺失 usage fact 的请求。
- 未实现自动处理 `unknown` reservation 的对账闭环；当前只暴露状态和指标，避免误释放可能已产生上游费用的请求。

## 回退方式

代码层可关闭 `billing.traffic_credit_reservation_enabled`，保持 `traffic_credit_reservation_shadow=true` 做观察。若已执行 migration，`reserved_usd` 和 reservation 表为向前兼容结构；回退代码前应先确认没有未结算 `reserved/dispatched/unknown/debt` reservation。
