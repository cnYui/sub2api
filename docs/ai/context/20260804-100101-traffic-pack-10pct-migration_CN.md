# 18080 到 18082 流量卡 10% 抽样迁移记录

## 执行范围

- 源库：`sub2api-postgres-dev`
- 目标库：`sub2api-official-18082-postgres`
- 抽样比例：10%
- 用户、登录身份、API Key、账号全量迁移。
- 普通订单、使用记录按 10% 抽样。
- 流量订单、用户流量额度和流水按 10% 抽样；管理员 `xiaobianfuai@gmail.com` 的全部流量卡关联订单、额度和流水强制纳入。
- 目标流量包目录保持 18082 已核验的 28 天配置，不被源库当前 365 天目录描述覆盖；历史额度保留源库实际 `expires_at`。

## 已提交结果

| 项目 | 数量/值 |
| --- | ---: |
| 用户 | 145 |
| 活动管理员 `xiaobianfuai@gmail.com` | 1 |
| 账号 | 2 |
| API Key | 194 |
| 使用记录 | 25,800 |
| 目标订单 | 86 |
| 目标流量订单 | 74 |
| 用户流量额度 | 40 |
| 流量额度流水 | 1,975 |
| 余额套餐 | 9 |
| 管理员未用流量卡额度 | 40 USD |
| 管理员额度预留值 | 已保留，目标总预留值 6.287259 USD |

逐条校验通过：抽样额度的用户邮箱、订单 ID、套餐、初始额度、剩余额度、预留额度、到账时间和过期时间与源库一致；用户密码哈希逐邮箱一致；使用记录、账号、流量额度和流水外键均无孤儿。

## 目标服务

- 18082 应用已恢复运行，容器状态 `healthy`。
- `http://localhost:18082/health` 返回 HTTP 200。
- `http://localhost:18082/api/v1/settings/public` 返回 HTTP 200。

## 备份

迁移前完整备份：`D:\CodeWorkSpace\migration-backups\18080-to-18082-20260804-095657`

- `source-18080.dump`
- `target-18082-before.dump`

## 尚未迁移的源库专有表

源库还存在 `billing_authorizations`、`billing_authorization_traffic_credit_items`、`traffic_credit_exhaustion_events`，但 18082 当前结构没有这些表，且本次目标为 198 迁移定义的流量包目录、独立额度和额度流水。它们未被强行写入目标库，后续若需要保留这些预留/耗尽审计，必须先为 18082 增加对应 schema 和业务读取逻辑。
