# 所有用户订阅延期与 10 USD 流量卡发放计划

## 背景

- 用户要求：给当前所有用户的过期时间都往后增加一天，从 29 天增加到 30 天，同时给所有用户发放一张 10 USD 的额度流量卡。
- 当前实际公网链路与旧 AGENTS 记忆不同：`api.aaccx.pw/health`、本机 nginx `127.0.0.1:8080/health` 均可用，nginx 配置已指向 `127.0.0.1:18080`。
- 当前运行容器为 `sub2api`、`sub2api-postgres`、`sub2api-redis`；旧 `sub2api-candidate*`、`sub2api-main-preview*` 已退出。本次应操作 `sub2api-postgres`。

## 表结构依据

- 订阅过期时间：`user_subscriptions.expires_at`。
- 用户流量卡：`user_traffic_credits`。
- 流量卡流水：`traffic_credit_ledger`。
- `user_traffic_credits.order_id` 非空且唯一，并引用 `payment_orders.id`，因此手工发卡必须同步创建系统用途的 `payment_orders`，不能绕过外键。

## 执行原则

- 先做 PostgreSQL dump 备份到 `deploy/backups/`，权限设为 `600`，并校验可读。
- 只改公网运行库，不重启容器，不改 Redis，不改 nginx，不改源码。
- 使用单个数据库事务批量执行，失败自动回滚。
- 不输出完整 API Key、内部 token、SMTP 密码或其它敏感字段。

## 计划 SQL 语义

1. 统计执行前用户数、未删除有效订阅数、10 USD OpenAI 流量卡数量和 ledger 数量。
2. 创建一个临时用户集合，范围为未删除用户。
3. 为每个用户创建一条 `payment_orders` 系统订单，订单金额为 0，状态为 `paid`，订单号带本次批次号。
4. 为每个订单插入一张 `user_traffic_credits`：`initial_usd=10`、`remaining_usd=10`、`platform='openai'`、`credited_at=NOW()`、`expires_at=NOW()+365 days`，`pack_id` 优先绑定现有 `gpt_traffic_10usd_3cny`。
5. 为每张新卡插入一条 `traffic_credit_ledger` purchase 流水，`amount_usd=10`、`balance_after_usd=10`。
6. 将未删除且 active 的 `user_subscriptions.expires_at` 加 `INTERVAL '1 day'`。
7. 提交后复查 health、用户数、发卡数、ledger 数和订阅延期数。

## 风险与回滚

- 风险：如果运行库存在多个订阅状态，直接更新历史已删除订阅会污染历史记录；因此只更新 `deleted_at IS NULL AND status='active'` 的订阅。
- 风险：如果重复执行，会重复发放流量卡；因此订单号使用固定批次前缀，并在 SQL 中以订单号唯一性和 `ON CONFLICT DO NOTHING` 保持本批次幂等。
- 回滚：使用本次备份 dump 恢复，或按批次订单号删除本批次新增的 `traffic_credit_ledger`、`user_traffic_credits`、`payment_orders`，并把本批次延期的 active 订阅减一天。

## 验证

- `http://127.0.0.1:18080/health`、`http://127.0.0.1:8080/health`、`https://api.aaccx.pw/health` 均返回 200。
- 新增 10 USD 流量卡数量等于未删除用户数。
- 新增 purchase ledger 数量等于新增流量卡数量。
- active 未删除订阅数量等于延期数量。
