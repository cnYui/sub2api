# 私下付款历史订单补录核心结果

## 结论

已完成固定五笔私下付款历史订单补录核心的本地代码验证。生产入口 `RunOfflinePaymentBackfill()` 只使用固定来源 `offline_paid_backfill_20260716`、套餐 `1`、分组 `2` 和五条确定事实；可变批次入口只存在于 `integration` build tag，未新增 HTTP、CLI、配置或 Ent 初始化路径。

## 代码与测试

- 使用 `database/sql` 的 serializable 事务、事务级 advisory lock 和 `payment_orders` 的 `SHARE ROW EXCLUSIVE` 表锁；任一前置条件失败、已有记录部分存在或已存在记录不完全匹配时整批回滚。
- dry-run 执行完整校验后回滚；五条订单和审计完全匹配时返回幂等 no-op。
- 订单明确写入 `offline/COMPLETED`、29.00 CNY、零手续费、`funding_mode=offline`、余额与网关金额为零、精确订阅关联和历史时间；补录审计使用实际写入时间及非空执行人。
- 新增固定批次事实回归测试，直接锁定 source、plan/group、金额、30 天、五个订阅/用户、精确付款与到期时间、确定性订单号。
- schema 预检新增 `provider_snapshot`：该字段由 `117_add_payment_order_provider_snapshot.sql` 定义为可空 `JSONB`，raw INSERT 显式写入 `NULL` 安全；缺列现在会在启动写事务前返回 `OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY`。该项已完成真实 RED/GREEN：修复前测试得到 `expected error, got nil`，修复后通过。

## 验证

以下命令均以实际退出成功完成：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 ./internal/service -run '^TestEnsureOfflinePaymentBackfillSchema'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run '^TestOfflinePaymentBackfill' -v
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
GOMAXPROCS=2 go test -p=1 -count=1 ./internal/service
git diff --check
```

集成测试覆盖首次创建、dry-run、精确重跑、空 operator、部分已有订单、其他订阅订单、订阅/用户/分组/状态/到期/套餐异常，以及订单与审计不匹配时的失败关闭。

## 未执行事项

- 未执行任何运行态补录。
- 未访问运行态 PostgreSQL、Redis、容器、部署、备份或迁移。
- 未创建一次性命令、未暴露 HTTP 路由，也未改动用户余额、订阅期限、返利、流量卡或支付 Provider 数据。
