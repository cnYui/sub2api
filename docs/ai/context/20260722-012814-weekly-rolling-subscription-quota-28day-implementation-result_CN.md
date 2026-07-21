# 2026-07-22 周滚动订阅额度与 28 天周期实施结果

## 实施范围

- 已按 `20260721-222923-weekly-rolling-subscription-quota-28day-design_CN.md` 在当前工作树实现公共 Codex 订阅的 28 天周期、每 7 天按订阅锚点滚动刷新、尾窗比例额度。
- 保留前序日额度超额顺延、图片计费与用量可见性修复，不回退既有改动。
- 覆盖后端 schema/Ent、订单不可变订阅快照、权益段周额度快照、`usage_facts.entitlement_period_id`、周窗口计算器、预授权/结算/缓存/Dashboard/Key 用量字段、退款 quote 与普通退款复算。
- 覆盖前端购买、订阅、Dashboard、Key 用量、用户订单退款、管理端套餐/订单/退款的周额度与 28 天文案；用户可见 USD 额度走整数展示，额度判断仍以后端精确值为准。

## 本地运行态与迁移演练

- 仅操作本地开发环境：`sub2api-dev`、`sub2api-postgres-dev`、`sub2api-redis-dev` 均为本地容器。
- 未操作公网、生产数据库、Nginx、Cloudflare 或 CLIProxyAPI 配置。
- 为本地 schema 演练先做 PostgreSQL 备份并验证可读：
  - 备份目录：`D:\CodeWorkSpace\sub2api\deploy\backups\weekly-quota-local-schema-20260722-011833`
  - 文件大小：`100337508` bytes
  - SHA256：`c7f066f88e39a2b59cea29b1467b28a41ebd5faeeda93326a29b398f9a89cf69`
- 本地开发库已应用 `174_weekly_rolling_subscription_quota_schema.sql` 并写入 `schema_migrations`，只做前向 schema，不执行存量 cutover 数据改写。
- 已执行 cutover dry-run，锚点为 `2026-07-21T23:00:00+09:00`，未传 `--apply`。

## dry-run 结果

- 公共 Codex active 订阅：`63` 条。
- 需人工处理或阻断自动切换的对象：`48` 个。
  - `completed_without_entitlement`: `5`
  - `overlapping_entitlement`: `38`
  - `refund_in_progress_order`: `5`
- 本次 compact 计数未发现：
  - `pending_or_paid_order`
  - `completed_without_subscription`
  - `usage_fact_unallocated`
- dry-run 预览已包含每条 active 订阅的 `entitlement_weekly_limit_usd`、剩余时长、首窗结束时间和 `first_window_effective_limit_usd`。
- `weekly-quota-cutover.sh --apply` 已增加阻断门禁：只要 dry-run 分类存在异常对象，正式切换会直接报错退出，避免带病自动迁移或自动退款。

## 验证

- `backend`: `go test ./...` 通过。
- `frontend`: `pnpm typecheck` 通过。
- `frontend`: `pnpm lint:check` 通过。
- `frontend`: `pnpm test:run` 通过；输出中仍有既有模拟网络错误、`router-link`/`el-tooltip` 测试 stub 警告与 Browserslist 旧数据提示。
- `frontend`: `pnpm build` 通过；输出中仍有既有 Vite dynamic import/chunk 警告、Browserslist 旧数据提示和 Node `DEP0190` 警告。
- `git diff --check` 通过。

## 未执行事项

- 未执行 `weekly-quota-cutover.sh --apply`，因此未写入存量订阅锚点、首窗、套餐配置、历史 usage facts 回填、超额债务审计，也未清 Redis 缓存。
- 由于本地 dry-run 仍有 48 个阻断对象，正式 cutover 前必须先人工处理或明确隔离这些对象。
- 当前工作树仍有大量后端、前端、Ent 生成文件、迁移文件与 `docs/ai/context/` 上下文文档未提交；未提交并不表示忽略，后续提交前需要统一复核敏感信息并纳入提交。
