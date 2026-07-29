# OpenAI 计费授权 Task 3 契约迁移记录

## 决策

- 新 migration 将 `traffic_credit_reservations` 原地改名为 `billing_authorizations`，并将 `usage_facts.reservation_id` 原地改名为 `authorization_id`；不复制或删除历史记录。
- 服务层新增 `AuthorizationID` 与 `OpenAIActualCost`，`UsageFact` 优先携带 `AuthorizationID`。
- `TrafficCreditReservationID` 和 `UsageFact.ReservationID` 暂时保留为兼容字段，直到后续任务替换旧结算仓储与入口；因此此时不得把 migration 应用到候选应用或启动候选应用。

## 原因与边界

旧仓储仍直接查询旧表与旧列。先完成 schema 和领域契约，随后在固定来源结算任务中原子替换仓储，避免在中间提交产生半迁移的运行环境。

本阶段只修改本地隔离分支与临时 Testcontainers 数据库，未停止、重启、迁移或写入公网服务、数据库、Redis、Nginx 或流量。

## 验证证据

```powershell
go test ./migrations -run TestMigration180 -count=1
go test -tags=unit ./internal/service -run 'TestUsageFact|TestUsageBilling' -count=1
go test -tags=integration ./internal/repository -run TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate -count=1
```

三项均通过。集成测试通过临时 PostgreSQL 18.1 和 Redis 8.4 容器运行，结束后业务测试容器已退出。
