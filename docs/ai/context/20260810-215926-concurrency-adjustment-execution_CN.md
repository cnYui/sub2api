# 并发调整执行记录

## 执行范围

- 按 `20260810-213417-concurrency-adjustment-runbook_CN.md` 对生产运行时 `18082` 实例执行并发调整。
- 通过容器内项目自带 `jwtgen` 为现有管理员签发临时 JWT，再调用本机生产 API；未输出或保存令牌、密码、API Key。
- 所有配置均热生效，未重启应用、PostgreSQL、Redis、Nginx 或 Cloudflare Tunnel。

## 执行前核对

- 上游账号共 15 个，实际 `concurrency` 全部为 `50`，不是手册分析中的默认 `3`。
- 用户共 157 个，`concurrency` 全部为 `20`。
- `GET /api/v1/admin/settings` 返回 `default_concurrency=20`。

## 执行结果

### 操作 1：上游账号并发调整

调用 `POST /api/v1/admin/accounts/bulk-update`，目标账号 ID 为：

`1, 2, 3, 4, 5, 6, 1128, 1129, 1130, 1131, 1132, 1164, 1165, 1166, 1167`

请求只提交 `concurrency=100`。接口返回 `success=15`、`failed=0`。

回读确认 15 个账号的 `concurrency` 均为 `100`；`/admin/ops` 并发快照中各账号 `max_capacity=100`，当前在途 1、等待队列 0。

### 操作 2：现有用户并发调整

调用 `POST /api/v1/admin/users/batch-concurrency`，提交：

`{"all":true,"mode":"set","concurrency":20}`

接口返回 `affected=157`。回读确认 157 个用户的 `concurrency` 均为 `20`。

### 操作 3：新用户默认并发

读取设置已确认 `default_concurrency=20`，目标状态已经满足，因此没有重复执行完整设置对象 `PUT`，避免无必要地覆盖其它设置字段。

## 健康与运维快照

- `http://127.0.0.1:18082/health`：200。
- `http://127.0.0.1:8080/health`：200。
- 最近 1 小时运维快照（生成时间 `2026-08-10T12:58:14Z`）仍包含变更前历史：请求时长 `P99=84494ms`、错误 `109`（全部为 `503`）、等待队列为 0。该窗口跨越变更前后，不能据此宣称 P99 已改善；需在新的流量窗口或峰值压力期间继续观察。

## 回滚基线

- 操作 1 的实际回滚值应为变更前 `50`，不是手册示例中的 `3`。
- 操作 2 与操作 3 变更前后均为 `20`，无额外回滚动作。
