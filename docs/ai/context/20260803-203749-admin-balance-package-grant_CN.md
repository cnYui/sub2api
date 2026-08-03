# 管理员余额套餐发放

## 背景

`/purchase` 的 ¥29、¥39 等商品已迁移为 `balance_package_plans`。原管理员订阅页的“分配订阅”仅查询 `subscription_type=subscription` 的模型分组，因此不能用于手动发放这些购买页套餐。

## 决策

- 管理员弹窗改为读取购买页同源的在售余额套餐，不再将余额套餐映射为模型订阅分组。
- 新增管理员余额套餐列表和发放接口，发放时服务端再次校验套餐在售状态与用户状态。
- 发放在单个数据库事务内创建 `payment_type=admin_grant`、金额为 0 的完成订单，写入套餐快照、首期余额、用户套餐和审计日志。
- 后续周期到账继续使用 `BalancePackageCreditService` 的既有逻辑；后台赠送不产生联盟返佣，也不允许用户或管理员发起退款。
- 前端为每次未完成的发放生成并保留 `Idempotency-Key`，服务端通过既有管理员幂等协调器避免网络重试重复到账。

## 验证

- `go test ./internal/service ./internal/handler/admin ./internal/server/routes`
- `pnpm typecheck`
