# 2026-07-22 周滚动订阅额度与 28 天周期完成收口记录

## 范围

- 按 `docs/ai/context/20260721-222923-weekly-rolling-subscription-quota-28day-design_CN.md` 继续本地代码实施与验证。
- 仅修改本地代码、测试和上下文文档；未操作公网、生产数据库、Nginx、Cloudflare、CLIProxyAPI 或 Docker 运行态。
- 保留前序日额度超额顺延、图片计费与用量修复改动。

## 本轮补齐

- 后端 group 服务归一化：
  - `backend/internal/repository/api_key_repo.go` 在 `groupEntityToService` 返回前调用 `service.NormalizePublicCodexSubscriptionQuota`。
  - 目的：历史 DB group 即使还保留旧日额度，服务层和 `/v1/usage` 等路径也稳定看到公共 Codex 的周额度与 28 天配置。
  - 增加 `TestGroupEntityToService_NormalizesPublicCodexSubscriptionQuota`。
- 兑换码批量更新：
  - `RedeemCodeBatchUpdateFields` 增加内部 `ValidityDays` 字段。
  - 批量切换到公共 Codex 订阅组时，服务层强制写入 28 天，避免管理员批量入口把旧 30 天写回兑换码。
  - repository batch update 支持更新 `validity_days`。
  - 增加 `TestRedeemService_BatchUpdate_PublicCodexGroupForces28DayValidity`。
- 前端旧 group 兜底：
  - 用户订阅页、顶部迷你进度、管理端订阅列表优先展示 rolling weekly / effective weekly，不再在公共 Codex 场景回退显示旧日额度。
  - 补充用户订阅页和顶部迷你进度测试。
- 退款币种展示：
  - 用户订单退款弹窗和管理端退款弹窗锁定：订单本金、手续费、预计退款使用订单币种；订阅额度继续使用 USD 整数显示。

## 已确认的主实现状态

- 周窗口计算器、预授权、成功结算、Redis/L1 缓存、429 `Retry-After`、订单快照、支付履约、退款 quote、Dashboard quota、Key 用量与前端主要文案均已接入 rolling weekly。
- 迁移 `backend/migrations/174_weekly_rolling_subscription_quota_schema.sql` 是前向 schema，不自动修改历史权益或套餐数据。
- cutover 脚本默认 dry-run；本轮没有执行 `weekly-quota-cutover.sh --apply`。

## dry-run / cutover 边界

前序 dry-run 仍存在阻塞对象，设计要求异常对象禁止自动迁移或自动退款：

- 公共 Codex active 订阅：63 条。
- 阻塞对象：51 个。
- `completed_without_entitlement`：5。
- `overlapping_entitlement`：43。
- `refund_in_progress_order`：5。
- `usage_fact_unallocated`：3。

因此本轮只完成代码和测试收口，未执行实际历史权益迁移。

两个本地已扣减的历史超额订阅 `21` 与 `53` 应在本地 cutover 时只写入 `already_applied` 债务审计，不再次扣减。生产未来执行必须重新锁定生产事实并按生产实时 overage 独立计算。

## 验证结果

后端：

- `go test -tags unit ./internal/service -run TestRedeemService_BatchUpdate`：通过。
- `go test ./internal/repository -run TestGroupEntityToService_NormalizesPublicCodexSubscriptionQuota`：通过。
- `go test ./internal/service`：通过。
- `go test ./internal/repository`：通过。
- `go test -tags integration ./internal/repository -run TestSubscriptionQuotaDebtAdjustmentRepository_CreateGetAndRejectDuplicateSource`：通过。
- `go test ./internal/handler/admin -run Test.*Redeem`：通过。
- `go test ./...`：通过。

前端：

- `pnpm test:run src/views/user/__tests__/SubscriptionsView.spec.ts src/components/common/__tests__/SubscriptionProgressMini.spec.ts src/views/user/__tests__/UserOrdersView.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts src/views/__tests__/KeyUsageView.spec.ts`：通过。
- `pnpm test:run src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts`：通过。
- `pnpm typecheck`：通过。
- `pnpm lint:check`：通过。
- `pnpm test:run`：通过。
- `pnpm build`：通过。

通用检查：

- `git diff --check`：通过；仅提示 `AGENTS.md`、`backend/go.sum` 下次 Git 写入会 LF -> CRLF。

前端全量测试输出包含既有 warning（模拟网络错误、未 stub 组件等），构建输出包含既有 Browserslist 与 chunk size / dynamic import 提示，命令退出码均为 0。

## 工作树状态

- 未提交、未推送、未部署。
- 当前工作树包含大量前序任务改动和本轮收口改动；没有执行 stage/commit。
- `docs/ai/context/` 下仍有多份未跟踪历史上下文文档，本文件也是新建未跟踪文档；后续提交前必须按项目规则统一复核并纳入提交，或明确说明暂不提交原因。
