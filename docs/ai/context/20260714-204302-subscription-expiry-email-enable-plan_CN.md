# 订阅到期邮件提醒启用计划

## 背景

用户要求让当前项目的会员/订阅到期邮件提醒生效，并说明当前 SMTP 已配置，可直接使用现有配置。

上一轮只读核验确认：

- 代码已有 `SubscriptionExpiryService`，服务启动后每分钟执行一次。
- 到期提醒事件为 `subscription.expiry_reminder`。
- active 订阅剩余完整 24 小时块为 `7`、`3`、`1` 天时各发送一次。
- 发送层按事件、订阅 ID、收件人和提醒档位去重。
- 当前运行态 SMTP 已配置，但 `subscription_expiry_notify_enabled=false`，因此不会发送到期提醒。

## 目标

将当前运行态 `sub2api-candidate` 的订阅到期邮件提醒开关打开，让已有后台任务在下一轮扫描时按既有规则自动发送提醒。

## 边界

- 不修改业务代码。
- 不修改 SMTP 配置或打印 SMTP 密码。
- 不修改用户、订阅、订单、API Key、Redis 缓存或容器镜像。
- 不重启服务。
- 只写入 `settings.subscription_expiry_notify_enabled=true`。

## 执行方案

1. 只读确认当前 settings 中 SMTP 配置处于可用状态，密码仅显示 `[CONFIGURED]`。
2. 只读确认 `subscription_expiry_notify_enabled=false`。
3. 直接在 PostgreSQL `settings` 表中 upsert 单个 key：`subscription_expiry_notify_enabled=true`。
4. 复核该 key 已为 `true`。
5. 通过公网/本地 health 检查确认服务仍健康。
6. 可选查看近期日志，确认没有因开关读取或邮件配置导致的明显错误。

## 验证口径

- 数据库 settings 中 `subscription_expiry_notify_enabled=true`。
- SMTP 关键配置仍存在，密码为 `[CONFIGURED]`。
- `sub2api-candidate`、PostgreSQL、Redis 容器健康。
- `http://127.0.0.1:18084/health` 返回 200。
