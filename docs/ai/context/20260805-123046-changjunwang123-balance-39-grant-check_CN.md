# changjunwang123@gmail.com 39 元余额套餐发放核验

- 时间：2026-08-05 12:30（Asia/Tokyo）
- 目标用户：`users.id=452`，状态为 `active`。
- 目标套餐：`balance-39`（`balance_package_plans.id=22`），价格 39 元，首周及每周额度 102 USD，28 天有效、每 7 天刷新一次、共 4 次。
- 当前记录：用户已有 `user_balance_packages.id=108`，同为 `balance-39`，开始于 2026-07-11 21:26:57 +08，到期于 2026-08-08 21:26:57 +08，已完成全部 4 次额度发放，当前窗口剩余 0.00186471 USD。
- 关联订单：`payment_orders.id=572`，为迁移生成的 `COMPLETED` 余额套餐订单；迁移审计记录保留于 `payment_audit_logs.id=1181`。

## 结论

当前套餐尚未到期。后台 `POST /api/v1/admin/payment/balance-packages/grant` 会在同一用户仍有有效套餐时返回 `BALANCE_PACKAGE_ACTIVE`，管理员手动发放不支持覆盖或叠加。为保持单一有效套餐与订单审计约束，未执行直接数据库写入或伪造订单。

后续需由管理员决定：在到期后再发放，或单独授权修改管理员发放规则以支持同款套餐续期。
