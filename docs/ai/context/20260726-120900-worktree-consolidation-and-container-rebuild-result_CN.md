# 本地工作区整理与双容器重建结果

时间：2026-07-26 12:09:00

## 工作区整理

本次已将当时本地全部业务改动、未跟踪上下文文档和已完成分支整理到本地 `main`：

- `00c706e4d fix: enforce OpenAI passthrough preauthorization`
- `63f3337bc feat: apply billing unit price multiplier`
- `620ff7834 docs: track local operation context`
- `80c73138a merge: fallback to traffic credit after weekly quota`

合并前后均执行 `go test ./...`，通过。未发现未合并的本地分支。

## 迁移链修复

重建外层 `18080` 时发现其从迁移 `177` 升级会在 `178_seed_codex_249_299_subscription_plans.sql` 失败：`168_image_token_billing_and_traffic_credit_events.sql` 已删除 `groups` 的五个历史图片定价字段，但 `178`、`179` 新套餐 seed 仍在读取和写入这些字段。

已从两份未应用迁移中移除以下过期字段：

- `image_rate_independent`
- `image_rate_multiplier`
- `image_price_1k`
- `image_price_2k`
- `image_price_4k`

同时扩展 `backend/migrations/auth_identity_payment_migrations_regression_test.go`，确保 `178/179` 不会重新引用已删除列。未重新添加废弃列，图片定价继续由当前 token 计费链路负责。

## Docker 重建

重建前已生成并通过 `pg_restore -l` 校验 PostgreSQL 备份：

- `backups/20260726-085217-before-18080-app-rebuild.dump`
- `backups/20260726-085217-before-18086-app-rebuild.dump`

已重建应用容器，未重建 PostgreSQL、Redis 或其他项目容器：

- 外层：`sub2api-dev`，`127.0.0.1:18080`
- 内层：`sub2api-upstream-latest`，`127.0.0.1:18086`

最终核对：

- 两个容器均为 `healthy`。
- `GET /health` 均返回 `{"status":"ok"}`。
- 外层数据库的 `schema_migrations` 已记录 `178_seed_codex_249_299_subscription_plans.sql` 与 `179_seed_codex_49_subscription_plan.sql`。

## 验证

- `go test ./...`（`backend/`）通过。
- `go test ./migrations`（`backend/`）通过。
- `git diff --check` 通过。
