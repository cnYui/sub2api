# 2026-07-22 周滚动订阅额度与 28 天周期本地实施审计结果

## 范围

- 承接 `20260721-222923-weekly-rolling-subscription-quota-28day-design_CN.md` 的本地实施。
- 保留前序日额度超额顺延、图片计费和用量修复。
- 本轮未操作公网、生产数据库、Nginx、Cloudflare 或 CLIProxyAPI。
- 本轮未执行 cutover `--apply`，只做本地 dry-run、代码验证和结果记录。

## 本轮确认与补齐

- 后端公共 Codex 订阅已具备 28 天有效期、每 7 天按订阅锚点刷新、尾窗按权益周额度比例折算、周额度耗尽返回 429 与 `Retry-After`。
- 订单创建、余额支付、回调履约和补单使用订单 `subscription_snapshot` 发放权益，不再依赖付款后的可变套餐配置。
- 权益周期保存 `weekly_limit_usd`、`period_total_quota_usd`、`quota_window_unit`、`quota_window_days`，usage fact 支持 `entitlement_period_id`。
- 退款 quote 使用订单本金 `amount`、不可退手续费、权益周期总额度和已归属 usage facts 计算；重复权益、重叠权益或无法归属历史用量进入人工审核。
- 前端购买页、订阅页、Dashboard、顶部进度、Key 用量、用户退款、管理端套餐/订单/退款/兑换码/默认订阅均改为周额度与 28 天语义；用户可见额度使用整数 USD formatter。
- 本轮新增迁移回归测试，显式断言 `174_weekly_rolling_subscription_quota_schema.sql` 只新增 schema 字段、索引、约束和 `subscription_quota_debt_adjustments`，不自动改写历史套餐或权益事实。

## 本地 cutover dry-run

命令：在 `sub2api-postgres-dev` 容器内执行 `weekly-quota-cutover.sh --migration-at=2026-07-22T00:00:00+09:00`，未传 `--apply`。

结果：

- 公共 Codex active 订阅预览：63 条。
- 需人工处理对象：51 个。
  - `completed_without_entitlement`：5 个订单。
  - `overlapping_entitlement`：43 组权益重叠。
  - `refund_in_progress_order`：5 个订单。
  - `usage_fact_unallocated`：3 条 usage facts。
- dry-run 明确输出“未写入任何订阅、权益、额度或缓存”。

结论：

- 本地库当前存在迁移门禁对象，符合设计中“异常对象禁止自动迁移或自动退款”的要求。
- 因存在阻塞对象，本轮没有也不应执行 `--apply`。

## 验证

- `backend`: `go test ./...` 通过。
- `backend`: `go test ./migrations` 通过。
- `frontend`: `pnpm typecheck` 通过。
- `frontend`: `pnpm lint:check` 通过。
- `frontend`: `pnpm test:run` 通过。
- `frontend`: `pnpm build` 通过。
- `backend/migrations/tools/weekly-quota-cutover.sh`: `bash -n` 通过。
- `git diff --check` 通过；仅提示 Windows 下 `AGENTS.md` 与 `backend/go.sum` 未来可能 LF -> CRLF。

前端测试与构建输出中仍有既有 mock 错误日志、`router-link` / `el-tooltip` 测试 warning、Browserslist 过期提示、Vite chunk/dynamic import warning；不影响本轮通过结果。

## 未提交状态

- 当前工作树包含前序多轮大批后端、前端、迁移和文档改动，本轮没有提交、推送或建 PR。
- `docs/ai/context/` 下存在多份前序未跟踪上下文文档；后续提交前需要统一复核敏感信息后纳入提交，或明确保留原因。
- 本轮新增结果文档：`docs/ai/context/20260722-024537-weekly-rolling-subscription-quota-28day-local-audit-result_CN.md`。
