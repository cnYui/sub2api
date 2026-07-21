# 周滚动订阅额度与 28 天周期补齐结果

## 范围

- 继续承接 `20260721-222923-weekly-rolling-subscription-quota-28day-design_CN.md` 的本地实施。
- 本轮只修改本地代码、测试与上下文文档；未操作公网、生产数据库、Nginx、Cloudflare 或 CLIProxyAPI。
- 保留前序已经完成的日额度超额顺延、图片计费、用量事实、174 schema、cutover 工具、订单快照、退款 quote 等改动。

## 本轮补齐

### 用户购买页与商品卡

- `PaymentView.vue` 的公开订阅卡继续使用后端返回的周额度快照字段：`weekly_limit_usd`、`period_total_quota_usd`、`quota_window_unit`、`effective_validity_days`。
- 统一商品卡 `PurchaseProductCard.vue` 不再硬编码 `PLAN` / `Price`，改为由购买页传入 i18n 后的 `eyebrowText` / `priceLabel`。
- 流量卡确认视图不再把有效期误标成“刷新时间”，改为“有效期”；流量卡标题、可用额度、购买按钮和默认描述改为 i18n。

### 用户 Dashboard / 订阅页 / Key 用量

- Dashboard 套餐额度行统一走通用窗口字段，并用 `formatSubscriptionQuotaUSD` 做用户可见整数 USD 展示。
- Key 用量页中订阅限额的环图、详情行、周重置时间与剩余额度使用整数 USD；普通 API Key 额度、钱包余额和支付金额仍保留原精度。
- 用户订阅页的订阅额度展示使用 `effective_weekly_limit_usd` 优先；流量卡文案改为 i18n，但金额仍保留两位小数，因为它不是周订阅额度。

### 管理端套餐

- 后端 `PaymentConfigService.CreatePlan/UpdatePlan` 新增公开 Codex 订阅套餐有效期归一化：命中固定公共 Codex group name 且类型为 `subscription` 时，保存为 28 天 / day。
- 管理端套餐弹窗选中公开 Codex 组时禁用有效期输入和单位选择，保存 payload 固定为 28 / days；后端仍做最终兜底。
- 公共 Codex group 识别改为基于固定 group name + subscription 类型，不依赖 `weekly_limit_usd` 是否已经被历史迁移修正。

## 本地 dry-run 证据

- 使用本地容器 `sub2api-postgres-dev` 的 `psql` 执行 cutover 脚本第一个 dry-run SQL 块，迁移锚点为 `2026-07-22T00:00:00+09:00`。
- 查询只读执行，不包含 `--apply` 后的写入 SQL。
- 结果：
  - 公共 Codex active 订阅：63 条。
  - 需人工处理对象：51 个。
  - 分类：
    - `completed_without_entitlement`: 5
    - `overlapping_entitlement`: 43
    - `refund_in_progress_order`: 5
    - `usage_fact_unallocated`: 3
- 因仍存在阻塞对象，本地不执行 `--apply`。

## 验证

- 后端：
  - `go test ./internal/service -run "Test(CreatePlanNormalizesPublicCodexValidity|UpdatePlanNormalizesPublicCodexValidity)$"` 通过。
  - `go test ./internal/service -run "Test(CreatePlanNormalizesPublicCodexValidity|UpdatePlanNormalizesPublicCodexValidity|Build|Weekly|RefundQuote|Payment)"` 通过。
  - `go test ./...` 通过。
  - `go test ./migrations` 通过。
  - `bash -n backend/migrations/tools/weekly-quota-cutover.sh` 通过。
- 前端：
  - targeted vitest：购买页、商品卡、Dashboard、Key 用量、订阅页、管理套餐弹窗相关测试通过。
  - `pnpm typecheck` 通过。
  - `pnpm lint:check` 通过。
  - `pnpm test:run` 通过。
  - `pnpm build` 通过。
- 其他：
  - `git diff --check` 通过，仅提示既有 LF/CRLF 工作树警告。

## 未提交状态

- 工作树在本轮开始前已有大量前序修改和未跟踪上下文/迁移文件；本轮没有 reset、revert 或删除历史文件。
- 本轮新增：`frontend/src/views/admin/orders/__tests__/PlanEditDialog.spec.ts` 与本结果文档。
- 本轮修改集中在：
  - `backend/internal/service/payment_config_plans.go`
  - `backend/internal/service/payment_config_service_test.go`
  - `frontend/src/views/user/PaymentView.vue`
  - `frontend/src/components/payment/PurchaseProductCard.vue`
  - `frontend/src/components/payment/purchaseProductCard.ts`
  - `frontend/src/components/user/dashboard/UserDashboardStats.vue`
  - `frontend/src/views/KeyUsageView.vue`
  - `frontend/src/views/user/SubscriptionsView.vue`
  - `frontend/src/views/admin/orders/PlanEditDialog.vue`
  - 对应 i18n 与测试文件

## 保留边界

- 未执行 cutover `--apply`。
- 未清 Redis 缓存。
- 未提交、未推送、未部署。
- 生产未来执行仍需重新锁定生产事实，不得复用本地 dry-run 结果作为生产迁移依据。
