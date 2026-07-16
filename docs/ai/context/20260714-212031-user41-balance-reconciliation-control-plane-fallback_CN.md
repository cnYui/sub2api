# user 41 余额核销控制面降级说明

## 原计划偏差

运行态未配置 `admin_api_key`，当前进程环境也没有可复用的 `SUB2API_ADMIN_API_KEY`。临时生成管理员 API Key 会改变全局凭据状态，超出本次余额核销范围，因此不采用。

## 等价执行方式

本次改用 PostgreSQL 单事务执行与 `AdminService.UpdateUserBalance()` 等价的最小状态变更：

- 对 `users.id=41` 使用余额充足条件执行 `balance = balance - 74`。
- 保持 `total_recharged` 不变。
- 创建确定性 code 的 `admin_balance=-74` 已使用调整记录，作为幂等保护和审计。
- 不修改支付宝充值订单 `175`、原充值兑换记录 `24` 或人工订阅 `110`。

事务提交后按当前代码协议：

- 删除 `billing:balance:41`。
- 对 user 41 的每个 API Key 计算 SHA-256 auth cache key，删除对应 `apikey:auth:<hash>`。
- 向 `auth:cache:invalidate` 发布 `<hash>`，使当前实例 L1 缓存同步失效。

执行前仍需完成 PostgreSQL 备份和可恢复性验证；所有写入均使用确定性审计 code，重复执行时不得二次扣减。
