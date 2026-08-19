# 两位用户提前刷新周额度执行记录

## 授权与范围

按管理员要求处理 `xunskyler@gmail.com` 和 `3867878292@qq.com` 的下一周余额套餐额度，优先抵扣欠费，使用户恢复使用。

本次仅处理以下用户、余额套餐和关联订单；未修改订单金额、订单状态、退款金额、历史用量、API Key 配额、流量卡或套餐有效期。

| 用户 | 用户 ID | 套餐 ID | 订单 ID | 每周额度 | 执行前余额 | 执行前状态 |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| `xunskyler@gmail.com` | 454 | 124 | 603 | 128 USD | 0.28974170 USD | `debt_paused`，1/4 |
| `3867878292@qq.com` | 599 | 122 | 590 | 76 USD | -0.41669895 USD | `debt_paused`，1/4 |

两个用户最近 30 分钟均无用量记录。订单 603 状态为 `REFUND_FAILED`、退款金额 `36.16 CNY`，本次保持不变；订单 590 状态为 `COMPLETED`、退款金额为 `0 CNY`。

## 执行方式

两个用户分别在同一个 `SERIALIZABLE` PostgreSQL 事务中锁定用户、套餐和订单，并校验邮箱、余额、套餐状态、到账进度、周额度、剩余额度、原定刷新时间、有效期、订单状态与退款金额。任一前置条件不匹配即回滚。

## 执行结果

### xunskyler@gmail.com

- 第 `2/4` 期 `128 USD` 已提前结算。
- 余额：`0.28974170 -> 128.28974170 USD`。
- 当前套餐剩余：`128 USD`；状态：`debt_paused -> active`。
- 下次刷新：`2026-08-21 09:55:15.000931 +08:00`。
- 执行前余额已为正，欠费抵扣为 `0 USD`，未新增欠费还款账本。
- 支付审计：`payment_audit_logs.id=1656`，动作 `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2`。

### 3867878292@qq.com

- 第 `2/4` 期 `76 USD` 已提前结算。
- `0.41669895 USD` 优先抵扣全部欠费。
- 余额与当前套餐剩余均为 `75.58330105 USD`；状态：`debt_paused -> active`。
- 下次刷新：`2026-08-20 11:03:45.514540 +08:00`。
- 欠费还款账本：`balance_debt_ledger.id=19`，来源 `balance_package_early_weekly_credit`，引用 `package:122:credit:2`。
- 支付审计：`payment_audit_logs.id=1657`，动作 `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_2`。

## 缓存与核验

- 删除余额缓存 `billing:balance:454`、`billing:balance:599`，均返回 `0`（执行时不存在）。
- 两个用户共 `8` 个 API Key 鉴权缓存已删除，并发布 `auth:cache:invalidate` 失效通知。
- 数据库回读确认两个套餐均为 `active`、到账进度均为 `2/4`，余额、剩余额度、下次刷新时间、订单状态、审计和欠费账本均符合预期。
- 应用容器状态为 `healthy`，本地健康端点返回 HTTP `200`。

