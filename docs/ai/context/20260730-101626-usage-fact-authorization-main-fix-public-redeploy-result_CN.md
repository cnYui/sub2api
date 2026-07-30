# Usage Fact 授权 schema 修复公网部署结果

时间：2026-07-30（Asia/Tokyo）

## 根因与数据结论

- 本次故障不是数据库恢复了旧备份。外层数据库从 2026-07-29 起已正确应用迁移 180，目标字段和表是 `authorization_id`、`billing_authorizations` 与 `billing_authorization_traffic_credit_items`。
- 昨天正常运行的镜像由 `codex/fix-usage-fact-authorization-column` 工作树中的未提交修改构建；这些修改没有进入 Git 提交，也没有进入 `main`。
- 今天从干净 `main` 重建时重新带入旧仓储 SQL，导致上游成功后外层 usage fact 持久化返回 503。

## 修复与提交

- 8 个生产/测试文件已完整移植昨天候选修复，并提交到本地 `main`。
- 修复提交：`5c8b677e1ae557a9ec51cc5b325f226ecfbc485c`。
- usage fact 仓储只读写 `authorization_id`；流量卡预留和结算只访问迁移后的授权表；旧入口字段归一到通用授权 ID。

## 测试

以下测试均通过：

```text
go test -tags=unit ./internal/service -run 'TestNewUsageFact|TestOpenAIGatewayService_RecordUsage' -count=1
go test ./migrations -run TestMigration180 -count=1
go test -tags=integration ./internal/repository -run 'TestUsageFactRepository_PersistsAndClaimsAuthorizationID|TestTrafficCreditReservationRepository|TestUsageBillingRepository' -count=1
```

## 备份与发布

- 发布前外层数据库备份：`deploy/backups/20260730-100912-before-usage-fact-main-fix-public-redeploy.dump`，已通过 `pg_restore --list` 校验。
- 发布镜像：`sub2api-localdev-sub2api:main-5c8b677e1-20260730`。
- 镜像 ID：`sha256:f7894d4749b7d81e44bb7a7b4c2dda83cd74b738271b01958b11ac2760a349b8`。
- 当前故障镜像保留为审计标签；紧急兼容回滚镜像仍保留为 `sub2api-openai-billing-candidate:authorization-column-fix-20260729`。
- 仅重建外层 `sub2api-dev`；数据库、Redis、Nginx、Cloudflare Tunnel 和内层 `18086` 未重建。

## 公网真实验证

- 使用用户提供但未记录的 API Key，经 `https://api.aaccx.pw/v1/responses` 调用 `gpt-5.5`。
- HTTP 状态 `200`，响应文本严格等于探针文本；输入 `4403` Token、输出 `16` Token。
- 内层 usage log 已落库，成本 `0.0052150000 USD`。
- 外层 usage fact `id=75519` 已落库并进入 `settled`；外层 usage log `id=248265` 已落库，实际成本 `0.0130375000 USD`，等于内层基础成本乘全局 `2.5` 倍率。
- 修复容器启动后未再出现旧字段、旧表名、`claim usage facts failed`、`persist_usage_fact_failed` 或 `billing_persistence_error`。
- `18080`、本机 Nginx `8080` 与 `https://api.aaccx.pw/health` 均正常；授权关联孤儿数为 `0`。

## 历史状态

当前仍有 `2902` 条历史 debt 和 `5` 条既有 pending usage fact；它们不是本次修复新增，也未在本次发布中批量处理。
