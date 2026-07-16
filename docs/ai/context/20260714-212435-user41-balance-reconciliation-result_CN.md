# user 41 支付宝补差余额核销结果

## 结果

已按用户授权核销 `1510623550@qq.com/users.id=41` 因组合支付未续接而留存的 `74.00` 元余额：

- `users.balance`：`80.32000000 -> 6.32000000`
- `users.total_recharged`：保持 `80.32000000`
- 新增调整流水：`redeem_codes.id=26`
- 调整类型：`admin_balance`
- 调整值：`-74.00000000`
- 幂等 code：`ADM-U41-O175-S110-20260714`
- 备注：`核销支付宝充值订单 175 留存余额；79 元套餐已通过人工订阅 110 发放`

支付宝充值订单 `payment_orders.id=175` 保持 `COMPLETED`，金额和实付均为 `74.00`；原充值兑换记录 `redeem_codes.id=24` 保留。人工套餐 `user_subscriptions.id=110/group_id=9` 保持 active，未重复发放或延长。

## 安全措施

写入前备份 PostgreSQL：

- 文件：`deploy/backups/20260714-212400-sub2api-candidate-before-user41-balance-reconciliation.dump`
- 大小：`74,535,566` 字节，约 71MB
- 权限：`600`
- `pg_restore -l` 验证可读，共 941 行

运行态未配置可复用管理员 API Key，因此未生成或替换全局管理员凭据。核销通过 PostgreSQL 单事务完成，使用用户 advisory lock、余额精确前置条件和确定性审计 code；任一前置条件不成立都会整体回滚。

## 缓存与健康

- `DEL billing:balance:41` 返回 0，表示原余额缓存不存在。
- user 41 当前 API Key `id=45` 的 L2 auth cache 删除返回 0，表示原缓存不存在。
- 向 `auth:cache:invalidate` 发布该 Key 的 SHA-256 cache key，`publish=1`，当前实例 L1 订阅者已接收。
- 复核 `billing:balance:41` 不存在。
- `http://127.0.0.1:18084/health` 返回 `{"status":"ok"}`。
- `https://api.aaccx.pw/health` 返回 HTTP 200。

## 未改动范围

- 未修改支付宝订单状态、交易号或支付审计历史。
- 未修改用户状态、API Key、套餐期限、额度或用量。
- 未修改 Redis 订阅缓存、Nginx、容器、镜像或支付配置。
- 未实现组合支付代码；组合支付仍处于设计阶段。
