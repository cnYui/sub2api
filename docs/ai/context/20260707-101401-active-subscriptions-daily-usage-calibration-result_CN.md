# Active 订阅日用量校准结果

## 执行范围

当前公网候选环境：

- 应用：`sub2api-candidate`
- 数据库：`sub2api-candidate-postgres`
- Redis：`sub2api-candidate-redis`
- 公网链路：`127.0.0.1:18084 -> nginx 8080 -> api.aaccx.pw`

本轮未重启或替换任何容器，未修改 nginx 或 Cloudflare Tunnel。

## 备份

执行前已备份 PostgreSQL：

- `deploy/backups/20260707-101105-sub2api-candidate-before-daily-usage-calibration.dump`
- 大小：约 32MB
- 权限：`600`
- 已用 `postgres:18-alpine pg_restore -l` 校验可读

## 修复动作

使用 `usage_logs` 作为事实源，将所有 active、未删除、未过期订阅的日窗口校准为今天：

- `daily_window_start = 2026-07-07 00:00:00+08`
- `daily_usage_usd = 今天 0 点到明天 0 点之间 usage_logs.total_cost 聚合值`
- `updated_at = now()`

SQL 在事务中短暂锁定 `user_subscriptions`，避免并发扣费覆盖校准值。

更新结果：

- 修复前全量差异：38 条 active 订阅
- 实际更新：38 条 active 订阅
- 修复后全量差异：0 条

同时删除当时已有的 Redis 订阅缓存：

- `billing:sub:17:2`
- `billing:sub:13:5`
- `billing:sub:60:4`
- `billing:sub:57:2`

后续运行中已有请求重建部分缓存，新缓存 `daily_window_start` 均为 `1783353600`，即 `2026-07-07 00:00:00+08`。

## 截图 4 位用户修复后状态

| 用户 | 套餐 | daily_window_start | daily_usage_usd | 今日请求数 | 今日聚合费用 |
| --- | --- | --- | ---: | ---: | ---: |
| `897858381@qq.com` | `codex-pool-29-usd` | `2026-07-07 00:00:00+08` | `0.0000000000` | 0 | `0` |
| `3056163754@qq.com` | `codex-pool-89-usd` | `2026-07-07 00:00:00+08` | `2.7636820000` | 13 | `2.7636820000` |
| `qixiaocheng777@gmail.com` | `codex-pool-19-usd` | `2026-07-07 00:00:00+08` | `0.2609524500` | 14 | `0.2609524500` |
| `daleselaji@gmail.com` | `codex-pool-49-usd` | `2026-07-07 00:00:00+08` | `18.5090592500` | 161 | `18.5090592500` |

说明：`daleselaji@gmail.com` 校准后仍在持续请求，后续复核时缓存与 DB 数值继续增长属于正常使用。

## 验证

- 全量 active 订阅差异复查：`remaining_mismatch_count = 0`，`remaining_rows = []`
- 单独复查截图 4 位用户：`daily_usage_usd` 均等于今日 `usage_logs.total_cost` 聚合值
- 健康检查：
  - `http://127.0.0.1:18084/health` 返回 `{"status":"ok"}`
  - `http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`
  - `https://api.aaccx.pw/health` 返回 `{"status":"ok"}`

## 后续建议

仍需修代码，避免明天或后续老数据再次出现同类问题：

- `RefreshExpiredUsageWindows` 对 `daily_window_start IS NULL` 且 `daily_usage_usd > 0` 的订阅，应按当前日窗口语义清零或重算。
- 管理端订阅列表展示前不应依赖陈旧窗口字段，可在查询层或后台任务统一校准。
