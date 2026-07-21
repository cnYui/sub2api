# 2026-07-22 周滚动订阅额度与 28 天周期可见缺口、缓存与退款收口结果

## 范围

本轮承接 `docs/ai/context/20260721-222923-weekly-rolling-subscription-quota-28day-design_CN.md` 与前序实现，继续补齐公共 Codex 订阅“28 天有效期 + 每 7 天按订阅锚点刷新额度”的本地实现缺口。

仅修改本地代码、测试和上下文文档；未操作公网、生产数据库、Nginx、Cloudflare、CLIProxyAPI 或项目运行态 Docker；未执行 `backend/migrations/tools/weekly-quota-cutover.sh --apply`；未 stage、未 commit、未 push。

## 本轮补齐

1. 购买页 legacy payload 兜底。
   - 即使后端旧响应仍携带 `daily_limit_usd=15`、`validity_days=30`，公共 Codex 商品仍按 group 名称归一显示周额度、28 天总额度、每 7 天刷新、28 天有效。
   - 29/39/59/79/99/149/199 元公共 Codex 套餐映射为 72/97/148/198/248/374/500 USD 每周。
   - 购买确认区同步断言不再展示“日限额 / 24 点刷新 / 30 天”。

2. 前端公共 Codex 周额度工具统一。
   - `frontend/src/utils/subscriptionQuota.ts` 集中提供公共 Codex 分组识别、28 天默认有效期、周额度映射、rolling weekly 判断和整数 USD 展示。
   - 购买页、订阅页、顶部进度、Dashboard、Key 用量和管理端相关页面优先消费后端有效窗口字段；只在 legacy 公共 Codex payload 下做展示兜底，前端不参与额度判断。

3. 后端滚动周旧窗口用量污染修复。
   - 新增并统一使用 `UserSubscription.RollingWeeklyUsageUSD(window)`。
   - `CheckUsageLimits`、`ValidateAndCheckLimits`、预授权、成功结算、进度计算、DTO mapper、Gateway usage response 均基于当前滚动周窗口读取用量。
   - 当持久化 `weekly_window_start` 仍停留在旧窗口时，请求前判断按当前窗口零用量开始，避免旧 `weekly_usage_usd` 污染新窗口。

4. Redis/L1 订阅缓存窗口事实补齐。
   - `SubscriptionCacheData` 增加 `EntitlementExpiresAt`。
   - Redis hash 增加 `entitlement_expires_at`。
   - `updateSubUsageScript` 对 `quota_window_unit='week'` 的 rolling weekly 缓存跨窗先归零再累加，并只更新周用量，不再同时累加 daily/monthly。
   - 缓存命中、DB fallback 和持久化结算统一使用订阅锚点、权益段到期和周额度快照。

5. 退款与支付测试夹具同步新契约。
   - 普通退款必须能唯一归属到 `payment_order` entitlement period；否则进入 quote/manual review。
   - 普通退款只撤销目标权益段，后续续费权益保留原起止时间，空档期由“当前无生效 entitlement period”判定不可用。
   - 管理员负向调整测试改为验证截断 overlapping entitlement period，不再期望删除整个订阅。
   - 支付测试夹具补真实 group seed，因为订单快照逻辑会在事务内锁定真实 group 行。

6. 周额度错误与页面空档提示补齐。
   - rolling weekly 超额返回 `WEEKLY_LIMIT_EXCEEDED` 并携带精确 `window_resets_at`，Gateway 可映射 HTTP 429 与 `Retry-After`。
   - 前端 rolling weekly 缺少后端 `weekly_window_resets_at` 时展示“当前周额度窗口尚未激活/未激活”，不再自行用 `weekly_window_start + 7 天` 推导。

## dry-run 与 cutover 状态

本地 cutover 仍保持 dry-run 门禁，未执行 `--apply`。既有 dry-run 阻塞对象仍需人工处理或隔离：

- 公共 Codex active 订阅：63。
- 阻塞对象：51。
- `completed_without_entitlement`：5。
- `overlapping_entitlement`：43。
- `refund_in_progress_order`：5。
- `usage_fact_unallocated`：3。

设计要求异常对象禁止自动迁移或自动退款，因此本轮没有自动修改历史权益、历史订单或历史 usage facts。

本地已扣减的历史超额订阅 `21` 与 `53` 后续本地 cutover 只能写入 `already_applied` 债务审计记录，不能再次扣减。生产未来执行必须重新锁定生产事实并按生产实时 overage 独立计算。

## 验证结果

已完成并通过：

```bash
cd backend && go test ./...
cd backend && go test -tags unit ./internal/service
cd backend && go test -tags integration ./internal/repository -run "TestBillingCacheSuite/TestSubscriptionCache/rolling_weekly_update_resets_stale_window_before_increment" -count=1 -v
cd frontend && pnpm typecheck
cd frontend && pnpm lint:check
cd frontend && pnpm test:run
cd frontend && pnpm build
git diff --check
```

`git diff --check` 仅提示既有 LF/CRLF 转换 warning，没有空白错误。前端测试和构建仍有既有 warning：`router-link` 未 resolve、Browserslist 数据旧、部分测试故意打印错误日志、Vite chunk / dynamic import 提示；命令退出码均为 0。

## 工作树状态

- 当前工作树仍包含前序日额度超额顺延、图片/用量修复、周滚动订阅、迁移工具、前端文案与测试等大量未提交改动。
- `docs/ai/context/` 下有多份未跟踪上下文文档，本文件也是新建未跟踪文档。
- 后续提交前必须运行 `git ls-files --others --exclude-standard docs/ai/context`，复核无敏感信息后纳入同一次功能提交，或单独提交为 `docs: archive ai context`。

## 后续门禁

1. 人工处理或隔离 dry-run 阻塞对象。
2. dry-run 清洁后才允许在本地执行 `weekly-quota-cutover.sh --apply`。
3. apply 后复核订阅、权益段、债务审计、usage facts、Redis/L1 缓存一致性。
4. 生产切换前必须重新备份 PostgreSQL 与 Redis，并基于生产实时事实重新 dry-run，不能复用本地结论。
