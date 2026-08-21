# dreamcmi@qq.com 20 USD 流量卡管理员发放

## 前置核验

- 用户 `dreamcmi@qq.com` 唯一匹配 `users.id=632`，状态为 `active`，未删除，角色为普通用户。
- 发放前普通余额为 `0 USD`，有效流量卡为 `0 USD`，流量卡欠费为 `0 USD`。
- 套餐为 `traffic_packs.id=3`、`gpt_traffic_20usd_5cny`，额度 `20 USD`、有效期 `28` 天、平台 `all`。

## 执行结果

- 在 PostgreSQL 单事务中创建零金额 `admin_grant` 完成订单 `690`。
- 创建流量卡额度 `user_traffic_credits.id=431`，初始和当前剩余均为 `20.0000000000 USD`。
- 创建 `traffic_credit_ledger.id=13702` 的 `purchase` 流水。
- 创建 `payment_audit_logs.id=1985`，动作 `ADMIN_TRAFFIC_PACK_GRANTED`，操作人 `admin:448`。
- 生效时间为 `2026-08-20 20:13:22 +08`，到期时间为 `2026-09-17 20:13:22 +08`。
- 未修改普通余额、历史订单、历史用量、API Key、已有套餐或模型配置。

## 核验

- 用户当前有效流量卡总额为 `20.0000000000 USD`，流量卡欠费仍为 `0 USD`。
- 本地应用、本地 Nginx、`aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 健康检查均返回 HTTP 200。
