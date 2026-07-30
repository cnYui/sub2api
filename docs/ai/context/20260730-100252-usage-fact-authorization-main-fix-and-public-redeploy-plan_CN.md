# Usage Fact 授权 schema 修复与公网重新部署计划

时间：2026-07-30（Asia/Tokyo）

## 根因定论

- 外层数据库没有恢复旧备份。迁移 `180_openai_billing_authorizations.sql` 已于 2026-07-29 应用，`usage_facts.authorization_id`、`billing_authorizations` 和 `billing_authorization_traffic_credit_items` 都是当前正确 schema。
- 昨天公网运行的镜像 `sha256:43fe5202...` 是从 `.worktrees/codex-fix-usage-fact-authorization-column` 的未提交修改构建，包含迁移 180 的仓储兼容修复。
- 这些生产代码修改从未提交或合入 `main`。今天从干净 `main` 重建后，旧的 `reservation_id` 和旧表名 SQL 再次进入公网，导致上游成功后外层 usage fact 持久化失败。

## 修复范围

1. `usage_fact_repo.go` 统一读写 `authorization_id`，内存兼容字段继续映射同一授权 ID。
2. `NewUsageFact` 在旧流量卡入口只有 `TrafficCreditReservationID` 时，将其归一为 `AuthorizationID`，保证 payload 与事实列一致。
3. 流量卡授权与结算仓储统一访问迁移后的 `billing_authorizations` 和 `billing_authorization_traffic_credit_items`。
4. 同步修复 4 个集成/单元测试文件，锁定创建、认领、回读、预留、释放与结算行为。

## 验证与发布

1. 运行领域层单元测试、usage fact/授权/结算仓储集成测试和迁移 180 测试。
2. 发布前重新备份外层 PostgreSQL，并通过 `pg_restore --list` 校验；为当前镜像保留独立回滚标签。
3. 从修复后的本地 `main` 无缓存构建，只重建外层 `sub2api-dev`，不重建数据库、Redis、Nginx 或内层 `18086`。
4. 先确认启动后无旧字段错误，再用用户提供的 Key 发起一次最小真实请求，核对公网响应、内层 usage log、外层 usage fact 和最终结算状态。

## 回滚边界

- 若部署后 schema 错误或真实请求失败，恢复部署前镜像标签并仅重建外层服务。
- 不回滚迁移 180，不恢复整库备份；当前数据库 schema 是正确目标。
