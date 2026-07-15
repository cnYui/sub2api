# 支付宝与余额组合支付实现结果

## 背景

本轮在隔离 worktree `/Users/wujianxiang/CodeSpace/sub2api/.worktrees/codex-alipay-balance-hybrid-payment`、分支 `codex/alipay-balance-hybrid-payment` 上实现真实“支付宝 + 余额”组合支付，目标是替代原先“余额不足先充值，再依赖前端续接购买”的不可靠流程。

未调用真实支付宝，未修改生产数据库、Redis、Nginx、容器或运行态。

## 实现概要

- 新增 `payment_balance_holds`，下单事务内冻结可用余额，支付宝订单只创建差额 `gateway_amount`。
- `payment_orders` 新增资金拆分、Provider 初始化租约、支付解析状态、迟到补偿和混合退款字段；历史订单只回填资金摘要，不创建历史 hold。
- 混合支付创建订单时使用稳定 `out_trade_no` 和 Provider 初始化租约，避免前台/后台重复创建外部订单。
- 支付状态重构为 `PAID / UNPAID / UNKNOWN`：
  - `PAID`：捕获余额 hold，并同事务履约。
  - `UNPAID`：释放余额 hold。
  - `UNKNOWN`：最多确认到 `expires_at + 5m`，期间不释放余额。
- 余额释放后的迟到支付宝成功回调不补发旧商品，而是将支付宝实付转入用户站内余额，订单状态为 `COMPENSATED`。
- 混合退款首次固定拆分 `refund_balance_amount` 与 `refund_gateway_amount`，网关成功后的重试只续接本地余额退款和撤权收尾。
- `/purchase` 页面在 `0 < balance < pay_amount` 且选择支付宝时创建 mixed 商品订单，展示余额抵扣和支付宝差额，不再跳转独立余额充值。
- `PaymentStatusPanel` 对 `PENDING + UNKNOWN` 显示“正在确认支付结果”，`COMPENSATED` 显示“支付已转入余额”。

## 验证结果

已通过：

- `cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/payment/provider ./internal/handler`
- `cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run TestMigrationsRunner_AuthIdentityAndPaymentSchemaStayAligned`
- `cd frontend && pnpm test:run src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/PaymentStatusPanel.spec.ts src/api/__tests__/payment.spec.ts`
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm build`
- `cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=embed ./internal/web`
- `cd backend && go test -p=1 -count=1 ./cmd/server`
- `git diff --check`

完整前端 `cd frontend && pnpm test:run` 当前仍有 11 个稳定失败，失败文件均不在本分支改动范围内：

- `src/composables/__tests__/usePersistedPageSize.spec.ts`
- `src/components/charts/__tests__/GroupDistributionChart.spec.ts`
- `src/components/charts/__tests__/ModelDistributionChart.spec.ts`
- `src/components/payment/__tests__/PaymentQRDialog.spec.ts`
- `src/views/user/__tests__/UsageView.spec.ts`
- `src/components/admin/usage/__tests__/UsageTable.spec.ts`

复跑上述失败文件仍然失败；对应模块未被本轮修改，判断为既有测试/实现不一致，不作为本功能代码回退依据。

## 注意事项

- 本分支只实现本地代码和迁移，尚未部署到公网运行态。
- 发布前需要先部署数据库迁移，再发布应用代码；否则新代码依赖的订单字段和 `payment_balance_holds` 不存在。
- 旧的独立余额充值订单不会被迁移为组合支付订单；历史订单只保留资金摘要回填。
