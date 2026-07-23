# 18086 内层临时余额放大热修计划

时间：2026-07-22 20:57 JST

## 目标

- 快速恢复当前双 Sub2API 链路可用性。
- 继续让 18080 作为唯一用户计费、套餐和流量卡事实源。
- 临时避免 18086 因自身 `standard` 模式余额校验返回 `INSUFFICIENT_BALANCE`。

## 当前事实

- 18080 外层桥接账号 `sub2api-latest-openai-upstream` 指向 `http://host.docker.internal:18086/v1`。
- 18086 内层唯一用户 `xiaobianfuai@gmail.com` 当前余额为 `-0.44224650`。
- 18086 内层唯一转发 Key `outer-sub2api-forwarder` 为 active，Key quota 为 `0`，即不做独立 Key quota 限制。
- 18086 直接请求 `/v1/models` 与最小 `/v1/responses` 当前返回 `403 INSUFFICIENT_BALANCE`。

## 变更方案

- 对内层 Postgres 做完整逻辑备份，并验证备份头部可读。
- 将 18086 内层 `users.email='xiaobianfuai@gmail.com'` 的 `balance` 临时更新为 `10000.00000000`。
- 不改 18080 计费表、不改公网用户余额、不改套餐/流量卡事实。
- 不改内层 OpenAI agent identity 账号、不清 Redis 调度池、不重启容器。

## 回滚

- 使用备份恢复内层 Postgres，或将该用户余额改回变更前的 `-0.44224650`。
- 本次只改 18086 内层用户余额，回滚边界清晰。

## 后续根治

- 稍晚应将 18086 切到 `RUN_MODE=simple` 或 `run_mode: simple`，让内层跳过 billing/balance checks。
- 再评估把 18086 内层用户并发从当前 `5` 提高到与真实账号池容量匹配的值。
