# 订阅到期邮件提醒启用结果

## 结论

已让当前运行态 `sub2api-candidate` 的订阅到期邮件提醒生效。

## 执行内容

- 写入前备份 PostgreSQL：
  - `deploy/backups/20260714-204501-sub2api-candidate-before-enable-subscription-expiry-email.dump`
  - 大小约 `71M`
  - 权限 `600`
  - 已用容器内 `pg_restore -l` 验证可读，TOC Entries 为 `930`
- 仅更新 settings 单个 key：
  - `subscription_expiry_notify_enabled=false -> true`
  - 数据库返回更新时间：北京时间 `2026-07-14 19:46:26.301793+08`

## 复核结果

当前 settings：

- `subscription_expiry_notify_enabled=true`
- `smtp_host=smtp.gmail.com`
- `smtp_port=587`
- `smtp_username=xiaobianfuai@gmail.com`
- `smtp_from=xiaobianfuai@gmail.com`
- `smtp_from_name=天才程序员小站`
- `smtp_password=[CONFIGURED]`
- `smtp_use_tls=false`

健康检查：

- `sub2api-candidate`：healthy
- `sub2api-candidate-postgres`：healthy
- `sub2api-candidate-redis`：healthy
- `http://127.0.0.1:18084/health`：HTTP 200，`{"status":"ok"}`
- `https://api.aaccx.pw/health`：HTTP 200，`{"status":"ok"}`

日志复核：

- 最近 2 分钟未发现 `SubscriptionExpiry`、`EMAIL_NOT_CONFIGURED`、SMTP、`subscription.expiry`、`expiry reminder` 或 notification email 相关错误。
- 同期存在若干与本次无关的 OpenAI 上游 400：`Invalid Value: 'tools'. Function 'image_gen.imagegen' conflicts with a hosted tool in the same request.`；不影响本次订阅到期提醒开关。

## 生效口径

已有后台任务每分钟扫描 active 订阅。后续当订阅剩余完整 24 小时块正好为 `7`、`3`、`1` 天时，会按事件 `subscription.expiry_reminder` 发送邮件；发送层按订阅、收件人和档位去重。

本轮未修改代码、SMTP 密码、用户、订阅、订单、API Key、Redis、镜像或容器，未重启服务。
