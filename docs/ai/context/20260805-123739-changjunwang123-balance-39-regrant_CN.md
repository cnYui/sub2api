# changjunwang123@gmail.com 39 元余额套餐重新发放

- 时间：2026-08-05 12:37（Asia/Tokyo）。
- 管理员明确授权提前结束用户已完成全部四期发放的旧 39 元套餐，并重新发放同款 28 天套餐。

## 执行结果

- 旧套餐 `user_balance_packages.id=108` 未物理删除，以 `expired` 状态保留；其套餐专属剩余 `0.00186471 USD` 已清零，并从用户余额中移除，避免混入新套餐余额。
- 旧订单 `payment_orders.id=572` 和历史迁移审计完整保留；新增审计 `ADMIN_BALANCE_PACKAGE_EARLY_EXPIRED`（`payment_audit_logs.id=1222`）。
- 新建零金额、可审计的管理员发放订单 `payment_orders.id=584`，编号 `ADMIN-GRANT-584`，类型为 `admin_grant`，套餐快照为 `balance-39`（计划 ID 22）。
- 新套餐 `user_balance_packages.id=116` 已激活：首周余额 102 USD，下一次刷新为 2026-08-12 11:37:27 +08，到期为 2026-09-02 11:37:27 +08；用户当前余额为 102 USD。
- 新订单保留首期到账审计 `BALANCE_PACKAGE_INITIAL_CREDIT`（ID 1223）和管理员发放审计 `ADMIN_BALANCE_PACKAGE_GRANTED`（ID 1224）。

## 后续注意

用户另有自行创建但未支付的 39 元余额套餐订单 `payment_orders.id=583`（`PENDING`）。未擅自取消；若该订单随后支付成功，现有同款续费逻辑会将新套餐有效期延长 28 天。

应用健康检查 `http://127.0.0.1:18082/health` 返回 200。
