# 渠道监控目录探测生产发布结果

## 发布范围

仅重建并替换 `sub2api-official-18082` 应用容器；PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷未重建。

## 生产核验

- 应用容器：`running healthy`。
- `http://127.0.0.1:18082/health`：200。
- 本地 Nginx `/health`：200；公网 `https://aaccx.pw/health`：200。
- 数据库迁移 `208_channel_monitor_interval_30m.sql` 已执行。
- 12 条监控均为 `api_mode=models`、`interval_seconds=1800`；5 个监控模板均为 `models`；默认间隔为 `1800`。
- runner 启动日志显示重新加载 12 条任务；最新历史记录的 `ping_latency_ms` 为空，目录探测返回的是 `/v1/models` 的鉴权/目录结果。

## 迁移完整性说明

首次发布尝试因修改已在生产执行的 `207` 迁移触发 checksum 保护而自动退出；已恢复 `207` 原始内容，并将间隔更新拆到新的不可变 `208` 迁移后重新发布成功。数据库未发生半执行迁移或业务数据回滚。
