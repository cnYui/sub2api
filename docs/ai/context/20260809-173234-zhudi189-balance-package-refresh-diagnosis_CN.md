# zhudi189@gmail.com 余额套餐刷新核查

## 结论

- 2026-08-09 16:27:50（UTC+8），用户 `zhudi189@gmail.com`（ID `502`）的有效余额套餐 `user_balance_packages.id=19` 已完成第 `3/4` 次周额度到账。
- 本次到账金额为 `781 USD`。套餐 `remaining_usd` 与用户 `users.balance` 均为 `781 USD`，套餐保持 `active`，无欠费暂停状态。
- 下一次额度刷新时间为 2026-08-16 16:27:22（UTC+8），套餐到期时间为 2026-08-23 16:27:22（UTC+8）。

## 容器重启关联

- `sub2api-official-18082` 于 2026-08-09 16:26:49（UTC+8）启动；PostgreSQL 和 Redis 已持续运行约 6 天，套餐状态未因应用容器替换而丢失。
- 刷新服务会在应用启动后立即扫描，并在之后每分钟执行一次。启动约 61 秒后，数据库写入审计 `payment_audit_logs.id=1518`：`BALANCE_PACKAGE_WEEKLY_CREDIT_3`，内容为 `{"credit_usd":781,"credited_count":3}`。
- 因此本次容器重启没有造成刷新遗漏，反而触发了启动后的补扫并完成到账。

## 核查范围

- 未修改用户余额、套餐、订单或审计记录。
- 未发现余额套餐刷新错误、欠费暂停或调度执行失败。
