# 公共 Codex 周额度提升 30% 实施结果

## 范围

- 分支：`codex/public-codex-weekly-quota-30pct`
- Worktree：`D:\CodeWorkSpace\sub2api\.worktrees\codex-public-codex-weekly-quota-30pct`
- 新增迁移：`backend/migrations/176_increase_public_codex_weekly_quota_amounts.sql`
- 未触碰公网 DB、Redis、容器、Nginx 和任何运行态数据。

## 额度变化

| 价格 | 分组 | 周额度变化 | 28 天总额度 |
|---:|---|---:|---:|
| 29 CNY | `codex-pool-19-usd` | 58 -> 76 USD | 304 USD |
| 39 CNY | `codex-pool-29-usd` | 78 -> 102 USD | 408 USD |
| 59 CNY | `codex-pool-49-usd` | 118 -> 154 USD | 616 USD |
| 79 CNY | `codex-pool-69-usd` | 158 -> 206 USD | 824 USD |
| 99 CNY | `codex-pool-89-usd` | 198 -> 258 USD | 1032 USD |
| 149 CNY | `codex-pool-135-usd` | 299 -> 389 USD | 1556 USD |
| 199 CNY | `codex-pool-179-usd` | 400 -> 520 USD | 2080 USD |

## 改动

- 后端公开 Codex 周额度固定映射已更新到新整数值。
- 数据库迁移会前向更新 `groups`、`subscription_plans`、有效 `subscription_entitlement_periods` 和未完成发放的 `payment_orders.subscription_snapshot`。
- 迁移保留历史用量、当天/本周用量、窗口起点、锚点和所有 usage facts。
- 退款按新 28 天总额度计算；历史已完成退款不回写。
- 前端中英文首页文案、购买页、订阅页、Key 用量页、Dashboard、管理端相关展示和测试 fixture 已同步新额度。

## 迁移安全边界

- 不包含 `codex-pool-local-unlimited`。
- 不包含 `DELETE`、`DROP`、`TRUNCATE`。
- 不更新 `usage_logs`、`usage_facts`。
- 未完成发放订单快照只覆盖订阅订单、公开 Codex 分组、`plan_id IS NOT NULL`、`subscription_id IS NULL` 的订单。
- `PENDING` 订单必须 `expires_at > NOW()`；`FAILED` 订单必须 `paid_at IS NOT NULL`。

## 验证

- `go test ./...`：通过。
- `go test -tags unit ./internal/handler ./internal/handler/admin ./internal/handler/dto ./internal/repository ./internal/service ./migrations -run "Test.*(PublicCodex|PaymentConfig|ConfirmPaymentFulfillsPublicCodex|Refund.*Quota|PaymentHandlerGetPlansIncludesGroupLimits|PaymentHandlerGetCheckoutInfoUsesPublicCodexQuotaSnapshot|NormalizesPublicCodex|Migration176)"`：通过。
- `go test -tags unit ./...`：未通过，原因是既有 `internal/server` 和 `internal/server/middleware` 的 unit 测试 stub 未实现 `service.APIKeyRepository.GetActiveBySHA256Hash`，不在本次改动范围。
- `pnpm typecheck`：通过。
- `pnpm lint:check`：通过。
- `pnpm test:run`：通过。
- `pnpm build`：通过；仍有既有 Vite chunk、动态导入和 Browserslist 警告。

## 发布提醒

公网发布前必须先开启维护、备份 PostgreSQL 并验证备份可读，再部署包含迁移 176 的版本。公网恢复后如果需要降级，不直接恢复旧库覆盖新增订单和用量，应写前向修正迁移。
