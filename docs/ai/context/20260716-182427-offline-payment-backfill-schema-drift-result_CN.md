# 私下付款补录 Schema Drift 失败关闭结果

## 修复范围

- `ensureOfflinePaymentBackfillSchema()` 继续只读预检，不执行 migration。
- 在原有 `162_refund_state_machine.sql`、`163_alipay_balance_hybrid_payment.sql` 与 `payment_orders` 关键列检查后，新增 `payment_audit_logs` 表及实际使用列 `id`、`order_id`、`action`、`detail`、`operator`、`created_at` 检查。
- 新增 PostgreSQL catalog 预检，要求：
  - `payment_orders.paymentorder_out_trade_no` 为唯一索引，索引列严格为 `out_trade_no`，partial predicate 规范化后严格匹配 `out_trade_no <> ''`。
  - `payment_audit_logs.idx_payment_audit_logs_order_action_uniq` 为唯一索引，索引列严格为 `order_id,action`，且没有 partial predicate。
- 缺表、缺列、缺索引、非唯一索引、列顺序不符或 predicate 不符均在开启事务前返回 `OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY`。
- 原有重复订单/审计集成测试临时移除唯一索引后，现在会更早得到 schema-not-ready；这保持无写入，并比进入批次校验后才拒绝更严格。
- 既有重跑 mismatch 集新增“5 笔订单均存在但一笔 `OFFLINE_PAYMENT_RECORDED` 审计缺失”场景，仍返回 `OFFLINE_PAYMENT_BACKFILL_EXISTING_RECORD_MISMATCH`。

## TDD 证据

- RED：缺任一唯一索引时，旧实现实际返回 `OFFLINE_PAYMENT_BACKFILL_TRANSACTION_FAILED`，因为它跳过索引预检并尝试 `Begin`；缺审计表或 `operator` 列时旧实现实际返回 `nil`。
- GREEN：SQLMock 测试通过，缺两个索引中的任一个、缺审计表/列、索引非唯一、列顺序不符和 partial predicate 不符都返回 `OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY`。索引缺失测试不设置 `Begin` 期望，并由 `ExpectationsWereMet()` 确认没有启动事务。

## 本地验证

以下命令均以退出码 0 完成：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 ./internal/service -run '^TestEnsureOfflinePaymentBackfillSchema|TestDefaultOfflinePaymentBackfillBatchContainsOnlyApprovedPaymentFacts' -v
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run '^TestOfflinePaymentBackfill' -v
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
```

- schema 与固定事实目标测试：`ok`，0.927s。
- Testcontainers 补录 integration：全部通过，6.428s，真实 PostgreSQL catalog 查询已执行；覆盖创建、dry-run、幂等、前置失败、审计缺失、重复审计和重复订单。
- 完整 service unit：`ok`，92.119s。

## 未执行事项

- 未部署。
- 未执行任何运行态 dry-run 或 execute。
- 未访问或写入运行态 PostgreSQL、Redis、容器、备份、迁移或用户数据；integration 仅使用测试框架创建的临时容器。
