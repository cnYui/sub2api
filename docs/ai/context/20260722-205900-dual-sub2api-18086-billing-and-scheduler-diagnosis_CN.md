# 双 Sub2API 18086 余额拦截与调度排查

时间：2026-07-22 20:59 JST

## 当前链路

- 公网/本机入口经 nginx 转发到外层 Sub2API：`127.0.0.1:8080 -> 127.0.0.1:18080`。
- 外层 `sub2api-dev` 负责用户认证、套餐/流量卡计费和用量事实。
- 外层 OpenAI 上游账号 `accounts.id=2 / sub2api-latest-openai-upstream` 指向内层：`http://host.docker.internal:18086/v1`。
- 外层旧 CPA 账号 `accounts.id=1 / cliproxy-local-openai` 当前 `schedulable=false`，不参与调度。
- 内层 `sub2api-upstream-latest` 负责模型通讯，真实 OpenAI agent identity 账号在内层 `accounts.id=1..10`。

## 并发事实

- 外层桥接账号 `sub2api-latest-openai-upstream` 并发为 `100`，绑定所有外层 OpenAI 付费/流量卡分组。
- 内层唯一管理用户 `xiaobianfuai@gmail.com` 当前用户并发为 `5`。
- 内层 10 个 OpenAI OAuth 账号每个 `concurrency=1`。
- 因外层通过内层 API Key 调用 18086，内层会先按 `user_id=1/api_key_id=1/group_id=2` 走用户并发槽；当前内层入口有效并发上限先被用户并发限制为 `5`，不是账号总数 `10`。
- 日志已出现 `timeout waiting for user concurrency slot`，说明内层用户并发曾经成为瓶颈。

## 18086 账号调度事实

- 内层 10 个 OpenAI OAuth 账号均为 `active` 且 `schedulable=true`。
- 账号 `id=1` 在 `2026-07-22 19:19:20 +08` 触发 OpenAI 5 小时限速，`rate_limit_reset_at=2026-07-22 22:37:20 +08`。
- 账号 `id=2` 在 `2026-07-22 19:29:51 +08` 触发 OpenAI 5 小时限速，`rate_limit_reset_at=2026-07-22 22:39:16 +08`。
- Redis active 调度池 `sched:2:openai:single:v25` 当前只包含账号 `3..10`，说明 1/2 已被限速剔出，调度会继续使用其余账号。
- 内层 `usage_logs` 显示 10 个账号均被使用过，不是单个账号一直使用。
- Redis 存在 `sticky_session:2:openai:*`，说明调度不是纯轮询；会结合 sticky session、调度池、账号状态和 failover。

## 当前不可用根因

- 内层 18086 当前运行在 `standard` 模式，配置文件没有设置 `run_mode: simple`，Compose 默认 `RUN_MODE=standard`。
- `standard` 模式会执行内层自己的余额/计费校验。
- 内层唯一用户余额为 `-0.44224650`，唯一 Key `outer-sub2api-forwarder` 仍 active，但没有独立 quota。
- 直接请求 `http://127.0.0.1:18086/v1/models` 和最小 `/v1/responses` 均返回 `403 INSUFFICIENT_BALANCE`。
- 因此当前故障不是“18086 账号全 429”，而是“18086 自己的计费余额校验挡住了外层已放行请求”。

## 设计判断

- 用户计费只能发生在 18080；18086 作为内部模型通讯层，不应因余额为负阻断请求。
- 上游 latest 已内建 `simple` 运行模式，注释明确用于内部自用并跳过 billing/balance checks。
- 更合理的内层运行方式是 `RUN_MODE=simple` 或 `run_mode: simple`，而不是给内层用户不断充值。
- 切到 simple 后仍会写 `usage_logs`，但不会扣内层余额；账本继续以 18080 为准。
- 切到 simple 后，已有 admin 用户并发为 5 时，启动迁移会升级到 30；如需更高吞吐，仍需显式调整内层用户并发或评估账号并发。

## 建议修复路径

1. 先备份内层配置、Postgres 和 Redis。
2. 将内层 `deploy/upstream_data/config.yaml` 增加 `run_mode: simple`，或在内层 Compose 环境设 `RUN_MODE=simple`。
3. 重启 `sub2api-upstream-latest`。
4. 验证启动日志出现 simple mode 提示，确认 `/v1/models` 不再返回 `INSUFFICIENT_BALANCE`。
5. 用外层公网 Key 打最小 `/v1/responses`，确认 18080 计费落库、18086 仅记录模型通讯 usage。
6. 如吞吐仍不足，再把内层用户并发从 30 调到与账号池容量匹配的值。

## 回滚边界

- 回滚只需恢复 `run_mode: standard` 或移除 `RUN_MODE=simple` 并重启内层容器。
- 若只改运行模式，不需要改 OpenAI agent identity 账号，不需要清 Redis 调度池，不需要修改 18080 计费事实。
