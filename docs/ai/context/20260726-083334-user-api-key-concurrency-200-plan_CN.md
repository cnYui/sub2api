# 18080 与 18086 API Key 并发统一为 200 计划

时间：2026-07-26 08:33:34

## 目标

将当前外层 `sub2api-dev:18080` 与内层 `sub2api-upstream-latest:18086` 的用户入口并发统一为 `200`。

当前入口并发按 API Key 独立计数，槽位键为 `concurrency:api_key:{apiKeyID}`；每把 Key 的上限来自所属用户的 `users.concurrency`。本次不调整上游账号的 `accounts.concurrency`。

## 变更范围

- 18080：`D:\CodeWorkSpace\sub2api\deploy\data\config.yaml` 的 `default.user_concurrency` 从 `5` 调整为 `200`；本地开发库所有未删除用户的 `concurrency` 统一为 `200`。
- 18086：`D:\CodeWorkSpace\sub2api-upstream-latest\deploy\upstream_data\config.yaml` 的 `default.user_concurrency` 从 `5` 调整为 `200`；内层库所有未删除用户的 `concurrency` 统一为 `200`。
- 两个容器均需重启以加载 YAML 默认值；数据库用户值在提交后立即生效，重启用于确保运行态一致。

## 已核对事实

- 18080：默认值为 `5`；未删除用户中 `128` 个 active 用户为 `5`，`1` 个 active 用户为 `0`，`1` 个 disabled 用户为 `5`。
- 18086：默认值为 `5`；唯一 active 用户为 `100`。
- `api_keys` 不存储独立并发字段，因此必须同时更新既有用户记录与新用户默认值。

## 执行与回滚

执行前备份两端运行态 `config.yaml` 和 PostgreSQL custom-format dump。更新在各自独立数据库事务中执行，只覆盖 `deleted_at IS NULL` 的 `users.concurrency`。

回滚方式：恢复对应配置备份，将对应库未删除用户的 `concurrency` 恢复为备份查询记录的原值，并重启对应容器。历史请求、计费、账号并发和套餐权益均不受影响。

## 验证

- 两端 `config.yaml` 均显示 `user_concurrency: 200`。
- 两端未删除用户的并发分布仅为 `200`。
- `127.0.0.1:18080/health` 和 `127.0.0.1:18086/health` 均返回 `200`。
- 容器状态保持 healthy，未重启 PostgreSQL、Redis、Nginx、Cloudflared 或 CLIProxyAPI。
