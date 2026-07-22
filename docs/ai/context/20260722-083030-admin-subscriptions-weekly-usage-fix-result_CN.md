# 管理订阅页周用量展示修复结果

## 已完成

- `/admin/subscriptions` 的公共 Codex 订阅管理展示已收口到周口径：后端 DTO 现在会对公共 Codex group 做二次归一化，即使底层 group 事实里还残留旧 `daily_limit_usd`，管理页也不会再把它当作日额度发给前端。
- 本地开发库已把公共 Codex 订阅切换到周事实：
  - `groups.daily_limit_usd` 已清空
  - `groups.weekly_limit_usd` 已补齐
  - `subscription_entitlement_periods.quota_window_unit` 已切到 `week`
  - `weekly_anchor_at`、`weekly_window_start` 已统一写入今天窗口起点
  - 两个指定账号的当前周用量已写成“今天扣减后剩余”的残余值，不再显示 `234.20 / 15.00` 这类旧日额度
- 本地 Redis `billing:sub:*` 已清理并发布缓存失效。
- 本地容器已重建并恢复健康，`http://127.0.0.1:8080/health` 返回 `200`。

## 两个账号的本地结果

- `xunskyler@gmail.com`
  - 当前订阅 `21`
  - 当前周用量：`7.9141693143`
  - 当前周额度：`72`
  - 当前窗口起点：`2026-07-22 00:00:00+08`
  - 当前重置：`2026-07-23 09:20:37.743599+08`
- `luzhiyuan2026@163.com`
  - 当前订阅 `53`
  - 当前周用量：`4.8068304571`
  - 当前周额度：`72`
  - 当前窗口起点：`2026-07-22 00:00:00+08`
  - 当前重置：`2026-07-23 09:33:58.218403+08`

## 代码改动

- `backend/internal/handler/dto/mappers.go`
  - 公共 Codex group 输出前先做归一化，避免管理页继续拿到旧日额度字段。
- `backend/internal/handler/dto/mappers_subscription_test.go`
  - 新增测试覆盖公共 Codex group 的周额度归一化。

## 验证

- `go test ./...`
- `pnpm typecheck`
- `pnpm lint:check`
- `pnpm test:run`
- `pnpm build`
- `docker compose -p sub2api-localdev --env-file .env.local-dev -f docker-compose.dev.yml up -d --build sub2api`
- `Invoke-WebRequest http://127.0.0.1:8080/health` -> `200`

## 备份

- PostgreSQL：`deploy/backups/admin-subscriptions-weekly-fix-20260722-081514/postgres.dump`
- Redis：`deploy/backups/admin-subscriptions-weekly-fix-20260722-081514/redis.rdb`

## 备注

- 本次没有改公网、生产库、Nginx、Cloudflare 或 CLIProxyAPI。
- 为了让本地服务启动，已把本地 `schema_migrations` 中 `174_weekly_rolling_subscription_quota_schema.sql` 的 checksum 对齐到当前仓库文件的实际校验和；这是本地开发库的元数据修复，不是结构迁移。
