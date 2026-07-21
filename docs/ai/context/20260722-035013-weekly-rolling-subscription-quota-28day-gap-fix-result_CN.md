# 周滚动订阅额度与 28 天周期缺口补齐结果

## 范围

- 承接 `20260721-222923-weekly-rolling-subscription-quota-28day-design_CN.md` 与 `20260722-031541-weekly-rolling-subscription-quota-28day-gap-fix-plan_CN.md`。
- 只修改本地代码、测试与上下文文档；未操作公网、生产数据库、Nginx、Cloudflare 或 CLIProxyAPI。
- 未执行 cutover `--apply`，因为本地 dry-run 仍存在需要人工处理的历史对象。

## 本轮补齐

- 管理员普通订阅退款在 `ExecuteRefund` 内改为事务内锁定订单、订阅、目标权益段和 usage facts 后二次计算 quote，再持久化不可变 `refund_basis`，不再信任前端金额或 Prepare 阶段金额。
- 修复退款 quote 原始 Ent 查询 rows 类型，避免运行时出现 `invalid type **sql.Rows. expect *sql.Rows`。
- 普通退款路径不再兜底撤销整条 subscription；支付退款必须通过目标 entitlement period 收尾，避免误伤后续续费权益。
- 强制人工退款保留管理员输入金额，但必须填写 reason，并写入 `FORCE_REFUND_REQUESTED` 审计。
- 余额订阅退款和网关退款收尾都支持目标权益段撤销；`RefundGatewayNotRequired` 可进入订阅权益收尾。
- 公共 Codex 订单履约优先消费订单 `subscription_snapshot`：即使支付完成时当前 group 已被禁用，只要订单快照包含 rolling weekly quota，就按快照发放 28 天、周额度、周期总额度和窗口规则。
- 顶部迷你订阅进度展示后端返回的 `weekly_window_resets_at`，并使用统一整数 USD formatter 展示有效周额度和已用量。

## 本地 dry-run 状态

- 前序本地 dry-run 使用 `sub2api-postgres-dev` 和迁移锚点 `2026-07-22T00:00:00+09:00`，只执行只读 SQL，未执行 `--apply`。
- 公共 Codex active 订阅：63 条。
- dry-run 阻塞对象：51 个。
- 分类：
  - `completed_without_entitlement`: 5
  - `overlapping_entitlement`: 43
  - `refund_in_progress_order`: 5
  - `usage_fact_unallocated`: 3
- 因存在重叠权益段、退款中订单和无法归属 usage facts，本地 cutover apply 仍应保持禁止。

## 复跑验证

- `go test ./...`：通过。
- `bash -n backend/migrations/tools/weekly-quota-cutover.sh`：通过。
- `pnpm typecheck`：通过。
- `pnpm lint:check`：通过。
- `pnpm test:run`：通过。
- `pnpm build`：通过。
- `git diff --check`：通过；仅有既有工作树 LF/CRLF 警告：
  - `AGENTS.md`
  - `backend/go.sum`

## 非阻塞警告

- 前端测试输出中有预期错误场景的 stderr 和组件 stub 警告，命令退出码为 0。
- 前端构建输出中有 Browserslist 数据偏旧、Vite chunk 大小与动态/静态导入混用提示，命令退出码为 0。

## 未提交状态

- 工作树仍包含前序任务的大量修改和未跟踪上下文文档；本轮没有 reset、revert、删除历史文件，也没有 stage/commit/push。
- 本轮新增本结果文档。
- 生产未来执行必须重新锁定生产事实、重新 dry-run、备份 PostgreSQL/Redis 并验证备份可读，不能复用本地 dry-run 结果作为生产迁移依据。
