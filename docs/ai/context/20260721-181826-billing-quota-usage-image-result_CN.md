# 额度削减、用量可见性与图片扣费修复结果

## 结论

- 本地开发链路已跑起并重建到当前代码：`sub2api-dev` 绑定 `127.0.0.1:8080`，`cliproxyapi-local-dev` 绑定 HTTPS `127.0.0.1:8317`，PostgreSQL/Redis 开发容器保持原数据卷。
- 已在本地开发库应用并登记迁移 `170_reduce_codex_subscription_daily_limits.sql` 与 `171_enable_user_error_request_view_default.sql`。
- 本轮只修改本地代码和本地开发库，没有触碰公网、生产 DB、Nginx、Cloudflare 或生产容器。

## 修复内容

### 1. 统一削减后的订阅额度

削减后额度已经固化到迁移、后端套餐接口、前端公开页/商店/用户页/管理页文案和测试 fixture：

| 套餐 | group | 新每日额度 |
|---|---|---:|
| 29 元订阅池 | `codex-pool-19-usd` | 15 USD |
| 39 元订阅池 | `codex-pool-29-usd` | 25 USD |
| 59 元订阅池 | `codex-pool-49-usd` | 39 USD |
| 79 元订阅池 | `codex-pool-69-usd` | 53 USD |
| 99 元订阅池 | `codex-pool-89-usd` | 66 USD |
| 149 元订阅池 | `codex-pool-135-usd` | 100 USD |
| 199 元订阅池 | `codex-pool-179-usd` | 133 USD |

后端 `GET /api/v1/payment/plans` 现在返回 group 派生字段：`group_name`、`daily_limit_usd`、`weekly_limit_usd`、`monthly_limit_usd`、`rate_multiplier`、`supported_model_scopes`，前端可直接使用后端事实源。

### 2. 用户看不到当前使用量/错误请求

- 根因：`allow_user_view_error_requests` 在快照库中未配置，`/api/v1/usage/errors` 被后端按设置返回 403。
- 修复：新增迁移 `171_enable_user_error_request_view_default.sql`，默认写入 `allow_user_view_error_requests=true`。
- 验证：本地登录态 `GET /api/v1/usage/errors` 返回 200，分页返回 20 条；`GET /api/v1/usage/dashboard/stats` 返回 200，当前用户累计请求数可见。

### 3. 图片真实扣费

- 根因 1：部分上游图片 usage 使用 `usage.image_tokens.input_tokens/output_tokens`，旧解析器没有读取。
- 根因 2：`/v1/images/*` 成功返回图片且缺少 `output_tokens_details.image_tokens` 时，`output_tokens` 实际代表图片输出 token，旧逻辑没有归入图片 token 成本。
- 修复：
  - `openAIUsageFromGJSON` 兼容 `image_tokens.input_tokens/output_tokens`。
  - 图片非流式与流式响应在已确认有图片输出且没有 image output token 拆分时，将 `OutputTokens` 推断为 `ImageOutputTokens`。
  - 新增/扩展服务测试覆盖 API Key 图片授权、非流式缺少 details、流式 JSON/SSE 多形态、usage fact 图片成本。

本地 CLIProxyAPI 账号池仍为空，未做 live 产图；图片扣费通过服务级成功响应 fixture 验证，不依赖真实上游账号。

## 本地运行态

- `sub2api-dev`: healthy，`127.0.0.1:8080`
- `cliproxyapi-local-dev`: healthy，`127.0.0.1:8317`
- `sub2api-postgres-dev`: healthy
- `sub2api-redis-dev`: healthy

迁移前备份：

- `deploy/backups/local-before-migrations-20260721-173927/sub2api-before-170-171.dump`

本地 DB 迁移记录已校正为应用运行时 checksum：

- `170_reduce_codex_subscription_daily_limits.sql`: `5b870d5ec73313d21cc2da65db6a6f8ac05cec0f05d7e57aa1e0cc0127eedd16`
- `171_enable_user_error_request_view_default.sql`: `3468cd09ed9f3304f5536bd5bef504788b3e5c0d461f43e16b63c6d69fd93d19`

说明：迁移 runner 对 SQL 内容 `TrimSpace` 后计算 SHA256；初次手动登记使用了原始文件 SHA256，重建容器后已按运行时算法修正本地 `schema_migrations`。

## 验证

- `docker compose --env-file .env.local-dev -p sub2api-localdev -f docker-compose.dev.yml -f docker-compose.cliproxy-local.yml up -d --build --no-deps sub2api` 成功；构建中已执行前端 `vue-tsc -b && vite build`。
- `curl http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`。
- 登录态 `GET /api/v1/payment/plans` 返回 7 个套餐，`daily_limit_usd` 为 `15/25/39/53/66/100/133`。
- 登录态 `GET /api/v1/admin/payment/plans` 返回新额度描述。
- `GET /api/v1/settings/public` 返回 `allow_user_view_error_requests=true`。
- 登录态 `GET /api/v1/usage/dashboard/stats` 返回 200。
- 登录态 `GET /api/v1/usage/errors` 返回 200。

测试命令：

```powershell
go test -count=1 ./migrations
go test -count=1 ./internal/handler
go test -count=1 ./internal/service
pnpm test:run src/components/payment/__tests__/PurchaseProductCard.spec.ts src/components/user/dashboard/__tests__/UserDashboardStats.spec.ts src/views/__tests__/HomeView.spec.ts src/views/admin/orders/__tests__/AdminPaymentPlansView.spec.ts src/views/user/__tests__/DashboardView.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/SubscriptionsView.spec.ts
```

结果：

- `migrations` 通过。
- `internal/handler` 通过。
- `internal/service` 通过。
- 前端 7 个文件 56 个测试通过。
