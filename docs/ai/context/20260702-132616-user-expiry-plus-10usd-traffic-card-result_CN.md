# 所有用户订阅延期与 10 USD 流量卡发放结果

## 目标

- 给当前所有未删除用户发放一张 10 USD OpenAI/GPT 流量卡。
- 将当前未删除且 active 的用户订阅过期时间统一往后增加一天。

## 当前运行态

- 当前公网入口为 `api.aaccx.pw -> nginx 127.0.0.1:8080 -> sub2api 127.0.0.1:18080`。
- 当前运行容器为 `sub2api`、`sub2api-postgres`、`sub2api-redis`。
- 本次操作的是 `sub2api-postgres`，未改旧 `sub2api-candidate*`，未重启服务，未改 Redis/nginx/源码。

## 备份

- 执行前已备份 PostgreSQL：
  - `deploy/backups/20260702-125423-sub2api-before-user-expiry-plus-10usd-traffic-card.dump`
  - 文件权限：`600`
  - 已用 `pg_restore -l` 校验可读

## 执行情况

- 批次号：`grant-20260702-10usd-day30`
- 第一次事务执行前后遇到 Docker/Postgres 底层短暂异常：`could not open file ... Read-only file system`。当时事务未进入写入阶段，连接退出后回滚；复查本批次订单、流量卡、流水均为 0。
- 排查后确认：
  - `sub2api-postgres` health 为 healthy；
  - 数据目录和 `/tmp` 均可写；
  - `pg_controldata` 显示数据库状态为 `in production`；
  - `users` 与 `traffic_packs` 查询恢复正常；
  - Docker 事件显示 13:20 JST 附近容器被 compose replace/start 过，异常更像容器/VM 切换窗口的环境抖动。
- 随后重新执行同一个幂等事务并成功提交。

## 数据结果

- 未删除用户数：52。
- 新增系统订单：52 条，`status=COMPLETED`，`order_type=traffic_pack`，`amount=0`，`pay_amount=0`。
- 新增 OpenAI/GPT 流量卡：52 张。
- 本批次流量卡总额：`520.0000000000` USD。
- 每张卡：
  - `initial_usd=10`
  - `remaining_usd=10`
  - `platform=openai`
  - `credited_at=2026-07-02 04:24:07.777555+00`
  - `expires_at=2027-07-02 04:24:07.777555+00`
- 新增 purchase 流水：52 条。
- 本批次 purchase 流水总额：`520.0000000000` USD。
- 延期 active 订阅：36 条。

## 验证

- `http://127.0.0.1:18080/health` 返回 200。
- `http://127.0.0.1:8080/health` 返回 200。
- `https://api.aaccx.pw/health` 返回 200。
- 批次校验：
  - 52/52 张卡为 10 USD OpenAI 流量卡；
  - 52/52 条系统订单为完成态；
  - 52/52 条流水为 purchase 且金额/余额均为 10 USD。
- active 订阅当前数量为 36；执行事务返回延期 36 条。发放后有 2 条 active 订阅被真实请求再次更新 `updated_at`，但 `expires_at` 已在事务中一并往后推。

## 注意

- 本次没有记录任何完整 API Key、内部 token、SMTP 密码或 HMAC secret。
- 如需回滚，可用备份 dump 恢复，或按批次号 `grant-20260702-10usd-day30` 删除本批次 `traffic_credit_ledger`、`user_traffic_credits`、`payment_orders`，并将 active 订阅 `expires_at` 减一天。
