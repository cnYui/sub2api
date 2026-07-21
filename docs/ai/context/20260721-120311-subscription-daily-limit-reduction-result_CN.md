# 套餐每日额度调整结果

## 执行时间

2026-07-21。

运行态数据库时间核对为 `2026-07-21 10:58:29+08`，时区 `Asia/Shanghai`。

## 备份

变更前已创建 PostgreSQL custom dump：

`deploy/candidate/dumps/sub2api-candidate-before-daily-limit-reduction-20260721-115852.dump`

备份大小：92MB。

SHA-256：

`503560932a1a3a6086294aaa048517d161791febdb809189a6e16462f6188c7e`

`pg_restore --list` 可读，TOC Entries 为 993，数据库版本和 dump 工具版本均为 PostgreSQL 18.4。

## 实际变更

单事务更新完成：

- `groups.daily_limit_usd`：7 行。
- `subscription_entitlement_periods.daily_limit_usd`：1 行未过期 active 权益快照。

未修改套餐价格、有效天数、有效单位、订阅起止时间、订阅状态、用户余额、订单、流量卡或 Docker 服务。

## 最终额度

| 套餐 | 价格 | 有效期 | 分组 | 更新后每日额度 |
|---|---:|---:|---|---:|
| 29 元订阅池 | 29 | 30 天 | codex-pool-19-usd | 15 USD |
| 39 元订阅池 | 39 | 30 天 | codex-pool-29-usd | 25 USD |
| 59 元订阅池 | 59 | 30 天 | codex-pool-49-usd | 39 USD |
| 79 元订阅池 | 79 | 30 天 | codex-pool-69-usd | 53 USD |
| 99 元订阅池 | 99 | 30 天 | codex-pool-89-usd | 66 USD |
| 149 元订阅池 | 149 | 30 天 | codex-pool-135-usd | 100 USD |
| 199 元订阅池 | 199 | 30 天 | codex-pool-179-usd | 133 USD |

## 缓存处理

已清理 Redis `apikey:auth:*` 认证缓存 11 条，并向 `auth:cache:invalidate` 发布 L1 失效消息。后续活跃请求已重新写入少量认证缓存，复查时缓存总数 9，旧日限额匹配数为 0。

## 验证

- 7 个目标分组的 `daily_limit_usd` 均匹配目标值。
- 1 条未过期 active 权益快照已从 19 更新为 15。
- `subscription_plans.price`、`validity_days`、`validity_unit` 保持不变。
- `curl http://127.0.0.1:18084/health` 返回 `{"status":"ok"}`。

## 回滚

如需回滚，仅恢复旧额度即可：

| group_id | 分组 | 旧每日额度 |
|---:|---|---:|
| 2 | codex-pool-19-usd | 19 |
| 3 | codex-pool-29-usd | 29 |
| 4 | codex-pool-49-usd | 49 |
| 9 | codex-pool-69-usd | 69 |
| 8 | codex-pool-89-usd | 89 |
| 11 | codex-pool-135-usd | 135 |
| 12 | codex-pool-179-usd | 179 |

也可使用本次变更前的 custom dump 做完整数据库恢复。
