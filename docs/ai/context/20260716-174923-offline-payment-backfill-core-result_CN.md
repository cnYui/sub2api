# 私下付款历史订单补录核心结果

## 本地代码范围

- 生产入口 `RunOfflinePaymentBackfill()` 仍只使用固定来源 `offline_paid_backfill_20260716`、计划 `1`、分组 `2` 与五笔确认的 29.00 CNY 订阅事实；可变批次包装器仍仅在 `integration` build tag 中编译。
- schema 预检继续要求 `162_refund_state_machine.sql`、`163_alipay_balance_hybrid_payment.sql` 及 `payment_orders` 关键列；缺失时返回 `OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY`，未添加隐式迁移。
- raw SQL 继续明确写入 `offline/COMPLETED`、零手续费、零余额/网关金额和三个空 Provider 字段；未调用支付履约、返利、余额、流量卡或 hold 链路。

## 本轮修复

- dry-run 在完成 advisory lock、表锁、订阅/用户/套餐和既有订单校验后直接返回 `Planned`，不再执行订单或审计 INSERT。原实现虽然回滚事务，但会永久消耗 `payment_orders` 与 `payment_audit_logs` 的 `BIGSERIAL` 序列；新增回归测试已防止该残余。
- 既有批次重跑现在要求每个确定性 `out_trade_no` 恰好一条订单，且每个订单恰好一条 `OFFLINE_PAYMENT_RECORDED` 审计。重复订单或重复同字段审计均返回 `OFFLINE_PAYMENT_BACKFILL_EXISTING_RECORD_MISMATCH`，不会被视为 no-op。

## TDD 证据

- dry-run 序列测试在修复前实际失败：期望下一个订单 ID 为 `2`，实际为 `7`；修复后通过。
- 重复审计测试在修复前实际失败：期望错误但得到 `nil`；修复后通过。
- 重复订单测试在修复前实际失败：期望错误但得到 `nil`；修复后通过。
- 两个重复记录测试仅在 Testcontainers 临时库中移除唯一索引，并通过 `t.Cleanup` 删除副本并重建索引；未触碰运行态容器或数据库。

## 验证

以下命令在本地实际退出成功：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 ./internal/service -run '^TestEnsureOfflinePaymentBackfillSchema|TestDefaultOfflinePaymentBackfillBatchContainsOnlyApprovedPaymentFacts'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run '^TestOfflinePaymentBackfill' -v
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
```

- 固定 schema/事实单测：`ok`，0.660s。
- 补录集成测试：全部通过，4.576s，覆盖创建、dry-run、序列无残余、精确重跑、空执行人、八类前置失败、订单/审计字段不匹配、重复审计和重复订单。
- service unit：`ok`，88.227s。

## 未执行事项

- 未部署。
- 未执行任何运行态 dry-run 或 execute。
- 未访问或写入运行态 PostgreSQL、Redis、容器、备份、迁移或用户数据。
