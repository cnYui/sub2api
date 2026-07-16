# 私下付款历史补录与订单展示收口结果

## 背景

用户确认 `offline` 仅用于 2026-07-16 这 5 笔已私下收款的历史订阅补录。每笔金额为 `29 CNY`，手续费为 `0`。这类记录需要出现在用户“我的订单”中，支付方式显示为“私下付款”，并且不能走用户或管理员侧的自动退款入口，只显示支付结果。历史订单中的 `manual_grant` 在前端显示为“赠送金额”。

## 已完成

- 已保留前序提交中的补录命令 `backend/cmd/offline-payment-backfill`：
  - 默认 dry-run；
  - 真实写入必须同时提供 `--execute --confirm=offline-paid-backfill-20260716 --operator=...`；
  - 启动时只加载配置并用 `database/sql` 直连，不调用 Ent 自动迁移；
  - 补录前检查 `payment_orders`、`payment_audit_logs` 所需字段和唯一索引；
  - 幂等键使用固定来源 `offline_paid_backfill_20260716`。
- 前端订单展示已补齐：
  - `payment_type=offline` 显示为“私下付款”；
  - `payment_type=manual_grant` 显示为“赠送金额”；
  - 管理端订单筛选支持 `offline`；
  - 支付方式分布页为 `offline` 使用中性色；
  - 用户结账可见支付方式中继续排除 `offline`，避免变成公开支付方式。
- 退款入口已收口：
  - 用户“我的订单”中 `offline` 不展示申请退款或重试退款入口；
  - 管理端共享订单表、订单详情页和订单管理页中 `offline` 不展示退款、通过退款、重试退款入口；
  - 管理端打开退款弹窗前对 `offline` 做防御返回。

## 验证

已在本地完成以下验证：

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 ./internal/payment
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/offline-payment-backfill
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run '^TestOfflinePaymentBackfill' -v
cd frontend && pnpm exec vitest run src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/orderUtils.spec.ts src/views/user/__tests__/paymentRefund.spec.ts src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts src/views/admin/orders/__tests__/AdminOrdersView.spec.ts src/views/admin/orders/__tests__/AdminPaymentDashboardView.spec.ts
cd frontend && pnpm typecheck
cd frontend && pnpm build
```

结果：

- 后端目标测试、补录命令测试、server 测试、补录 integration 测试均通过；
- 前端目标 Vitest `6 files / 46 tests` 通过；
- `pnpm typecheck` 通过；
- `pnpm build` 通过，仅有既有 Vite dynamic import/chunk 与 Browserslist 过期提示。

## 特意未做

- 未执行运行态 dry-run；
- 未执行运行态 `--execute`；
- 未修改运行态 PostgreSQL、Redis、容器或 Nginx；
- Docker 镜像构建中需要下载依赖的验证按用户要求跳过。

后续若要对真实环境补录，仍需再次明确授权，并按“备份数据库 → 运行 dry-run → 核对 5 笔目标用户/订阅/金额/时间 → execute → 复核订单与审计日志”的顺序执行。
