# 18080 与 18086 API Key 并发统一为 200 结果

时间：2026-07-26 08:33:34

## 结果

已将当前外层 `sub2api-dev:18080` 和内层 `sub2api-upstream-latest:18086` 的用户入口并发统一为 `200`。

并发仍按 API Key 独立计数，每把 Key 的上限取所属用户的 `users.concurrency`；本次未调整上游账号的 `accounts.concurrency`。

## 实际改动

- 18080：`deploy/data/config.yaml` 的 `default.user_concurrency` 从 `5` 改为 `200`；本地开发库 `130` 个未删除用户均已设为 `200`，其中 `129` 个 active、`1` 个 disabled。
- 18086：`D:\CodeWorkSpace\sub2api-upstream-latest\deploy\upstream_data\config.yaml` 的 `default.user_concurrency` 从 `5` 改为 `200`；内层库唯一的 active 用户已从 `100` 设为 `200`。
- 已分别重启 `sub2api-dev` 与 `sub2api-upstream-latest`；未重建镜像，未重启 PostgreSQL、Redis、Nginx、Cloudflared 或 CLIProxyAPI。

## 备份

变更前的配置和 PostgreSQL custom-format dump 位于：

`C:\tmp\sub2api-concurrency-200-backups\20260726-083334`

包含 `18080-config-pre.yaml`、`18080-postgres-pre.dump`、`18086-config-pre.yaml` 与 `18086-postgres-pre.dump`。

## 验证

- `http://127.0.0.1:18080/health`：`200 {"status":"ok"}`。
- `http://127.0.0.1:18086/health`：`200 {"status":"ok"}`。
- 两个容器均为 `healthy`。
- 两端容器内 `/app/data/config.yaml` 均显示 `default.user_concurrency: 200`。
- 18080 的未删除用户分布为 `active=129 / 200` 与 `disabled=1 / 200`；18086 的未删除用户分布为 `active=1 / 200`。
