# 公网订阅窗口刷新重新部署结果

## 时间

2026-07-07 08:47 (北京时间)

## 发布摘要

- 目标：将本地 `main` 中的订阅窗口刷新修复部署到公网 `sub2api-candidate`
- 结果：**成功**

## 新镜像

- 镜像：`sub2api-candidate:20260707-084458-74e5a4bb0-subscription-window-refresh`
- Image ID：`sha256:82406485ee568f033aca7cc6ed3291a262be37329de19fbecab151792e879e5a`
- 基于本地 `main` HEAD：`74e5a4bb0`（`docs: add subscription window refresh redeploy plan`）
- 本次修复提交：`d6e337238`（`fix: refresh subscription windows before billing checks`）

## 发布前快照

| 指标 | 值 |
|------|-----|
| active_users | 62 |
| active_keys | 58 |
| active_subscriptions | 44 |
| migrations | 195 |
| latest_migration | 159_auto_api_key_effective_group.sql |
| stale daily windows | 40 |

## 备份文件

| 文件 | 大小 | 权限 |
|------|------|------|
| `deploy/backups/20260707-084458-sub2api-candidate-postgres-before-subscription-window-refresh.dump` | 32MB | 600 |
| `deploy/backups/20260707-084458-sub2api-candidate-redis-before-subscription-window-refresh.rdb` | 81KB | 600 |

## 发布后健康检查

| 端点 | 结果 |
|------|------|
| 18084/health | 200 |
| 8080/health | 200 |
| api.aaccx.pw/health | 200 |
| aaccx.pw/dashboard | 200 |
| aaccx.pw/purchase | 200 |

## 发布后数据库状态

- migrations：195（未变，本次修复无新 migration）
- stale daily windows：40（将在首次 API 请求时自动刷新）

## 容器状态

| 容器 | 状态 |
|------|------|
| sub2api-candidate | Up (healthy) - **新镜像** |
| sub2api-candidate-postgres | Up (healthy) - **未改动** |
| sub2api-candidate-redis | Up (healthy) - **未改动** |

## 回滚信息

旧容器已重命名为 `sub2api-candidate-before-window-refresh-20260707-084458` 并停止，保留用于应用层回滚。

## 修复内容

- `BillingCacheService` 在订阅套餐限额判断前按 `timezone.StartOfDay/StartOfWeek` 和 30 天滚动月窗口刷新过期窗口
- Redis `billing:sub:*` 订阅缓存新增 `daily_window_start`、`weekly_window_start`、`monthly_window_start` 字段
- 旧 Redis 订阅缓存缺窗口字段时回源 DB 自愈
- 新增条件式 `RefreshExpiredUsageWindows` 防止并发重复清零

## 说明

- 未修改 nginx 或 Cloudflare Tunnel
- 未停止或重建 Postgres/Redis
- 未执行回滚
- 旧容器 env 已清理
