# Usage Fact 授权 schema 主分支修复结果

时间：2026-07-30（Asia/Tokyo）

## 实现

- `usage_fact_repo.go` 已统一使用 `usage_facts.authorization_id`。
- 旧 `TrafficCreditReservationID` 在构造 usage fact 时归一到 `AuthorizationID`，序列化 payload 与事实列引用同一授权。
- 流量卡预留和结算 SQL 已统一使用 `billing_authorizations` 与 `billing_authorization_traffic_credit_items`。
- 生产仓储不再引用旧 `traffic_credit_reservations`、`traffic_credit_reservation_items` 或 `usage_facts.reservation_id`。
- 8 个代码/测试文件与 2026-07-29 已验证候选工作树中的未提交修复逐文件哈希一致。

## 验证

以下命令均通过：

```text
go test -tags=unit ./internal/service -run 'TestNewUsageFact|TestOpenAIGatewayService_RecordUsage' -count=1
go test ./migrations -run TestMigration180 -count=1
go test -tags=integration ./internal/repository -run 'TestUsageFactRepository_PersistsAndClaimsAuthorizationID|TestTrafficCreditReservationRepository|TestUsageBillingRepository' -count=1
```

`gofmt`、`git diff --check` 和旧 schema 标识符定向扫描也均通过。

## 构建基线要求

本次修复必须先提交到本地 `main`，再从该提交构建公网镜像，避免重复 2026-07-29 候选修复只存在于脏工作树、后续重建丢失的问题。
