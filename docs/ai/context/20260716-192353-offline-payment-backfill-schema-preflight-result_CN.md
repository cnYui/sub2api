# Offline 私下付款历史补录 schema preflight 修复结果

## 背景

2026-07-16 的五笔私下付款历史补录只允许写入固定批次，且必须在运行库已经具备退款状态机和支付宝余额组合支付字段后执行。补录服务此前已做迁移与 `payment_orders` 列检查，但对补录依赖的唯一索引和审计表结构检查不足，若运行库 schema 不完整，可能在事务中才暴露问题。

## 本次修改

- `ensureOfflinePaymentBackfillSchema()` 增加 `payment_audit_logs` 表存在性检查。
- 增加 `payment_audit_logs` 必需列检查：`id`、`order_id`、`action`、`detail`、`operator`、`created_at`。
- 增加唯一索引检查：
  - `payment_orders.paymentorder_out_trade_no` 必须唯一，列为 `out_trade_no`，谓词为 `out_trade_no <> ''`。
  - `payment_audit_logs.idx_payment_audit_logs_order_action_uniq` 必须唯一，列为 `order_id,action`。
- 缺索引、索引定义不匹配、缺审计表或缺审计列时，统一在开启补录事务前返回 `OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY`。
- 补充 SQLMock 单测覆盖 fail-closed 路径，并补充集成测试覆盖“目标订单存在但对应补录审计缺失”的拒绝场景。

## 验证

已新鲜执行并通过：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 ./internal/service -run '^TestEnsureOfflinePaymentBackfillSchema|TestDefaultOfflinePaymentBackfillBatchContainsOnlyApprovedPaymentFacts'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run '^TestOfflinePaymentBackfill' -v
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
git diff --check
```

## 未执行事项

- 未连接运行态数据库。
- 未对候选库或生产库执行 dry-run。
- 未执行真实五笔历史补录。
- 未修改前端、Dockerfile 或命令行补录入口。
