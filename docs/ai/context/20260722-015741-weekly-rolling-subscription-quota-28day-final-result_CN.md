# 2026-07-22 周滚动订阅额度与 28 天周期补齐结果

## 范围

- 基于 `docs/ai/context/20260721-222923-weekly-rolling-subscription-quota-28day-design_CN.md` 继续补齐本地代码实现。
- 未操作公网、生产数据库、Nginx、Cloudflare 或 CLIProxyAPI。
- 未执行 cutover `--apply`；本轮只做本地代码、测试和构建验证。

## 本轮补齐

- 购买页与购买确认页改为消费统一套餐窗口字段：
  - 公共 Codex 订阅展示“周额度 / 28 天总额度 / 每 7 天刷新 / 28 天有效期”。
  - 删除公共 Codex 购买卡和确认页上的“日限额 / 24 点刷新”路径。
  - 流量卡明细从“刷新时间”改为“有效期”，避免和订阅额度窗口混淆。
- Dashboard 额度卡改为通用窗口标签：
  - `week` 显示“本周额度”。
  - `period/month` 显示“本期”。
  - `none` 显示“不限额”。
  - 兜底才显示“今日”。
- 后端补齐兼容发布期判定：
  - 公共 Codex 分组名不再单独决定一条订阅已经可走滚动周窗。
  - 只有当前订阅具备 active entitlement period、周额度快照和可计算窗口时，预授权、缓存资格、窗口维护、进度展示、`/v1/usage` 剩余额度和成功结算才走滚动周窗。
  - 旧日额度事实尚未 cutover 时继续按 legacy 日额度运行，避免兼容发布阶段把有效订阅误判为无效。
- 前端测试夹具补齐固定周额度快照：
  - 72/97/148/198/248/374/500 USD 每周。
  - 28 天总额度分别为 288/388/592/792/992/1496/2000 USD。

## 验证

- `backend`: `go test ./internal/service -run "EffectiveGroupResolver|PublicCodex|WeeklyWindow|BalancePay|Subscription|RefundQuote|Refund"` 通过。
- `backend`: `go test ./internal/handler ./internal/server` 通过。
- `backend`: `go test ./...` 通过。
- `frontend`: `pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/PurchaseProductCard.spec.ts src/components/user/dashboard/__tests__/UserDashboardStats.spec.ts` 通过。
- `frontend`: `pnpm typecheck` 通过。
- `frontend`: `pnpm lint:check` 通过。
- `frontend`: `pnpm test:run` 通过。
- `frontend`: `pnpm build` 通过；仅保留既有 Vite chunk-size / dynamic-import warning 与 Browserslist 过期提示。
- `git diff --check` 通过；仅提示 Windows 下 `AGENTS.md`、`backend/go.sum` 未来可能 LF -> CRLF。

## 未做

- 未修改生产运行态。
- 未执行本地 cutover apply。
- 未提交、未推送。

## 工作树提醒

- 当前工作树包含大量前序修改和未跟踪上下文文档；本轮只新增本结果文档并补齐购买页、Dashboard、兼容周窗判定及相关测试。
- `docs/ai/context/` 下仍有前序未跟踪文档，后续提交前需要统一复核敏感信息并纳入提交或明确保留原因。
