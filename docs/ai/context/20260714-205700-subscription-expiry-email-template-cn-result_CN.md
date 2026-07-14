# 订阅到期提醒邮件中文模板调整结果

## 结论

已将当前运行态订阅到期提醒邮件模板改为自然中文，并同时覆盖 `zh` 与 `en` 两个 locale，避免用户没有语言偏好记录时回落到英文模板。

## 备份

写入前备份 PostgreSQL：

- `deploy/backups/20260714-205445-sub2api-candidate-before-subscription-expiry-email-template-cn.dump`
- 权限 `600`
- 已用容器内 `pg_restore -l` 验证可读，TOC Entries 为 `930`

## 写入内容

更新 settings：

- `notification_email_template:subscription.expiry_reminder:zh`
- `notification_email_template:subscription.expiry_reminder:en`

两个模板内容一致。

主题：

```text
[天才程序员小站] 您的会员订阅还有 {{days_remaining}} 天到期
```

正文核心文案：

```text
会员订阅快到期了

{{recipient_name}}，您好：

您的 {{subscription_group}} 会员订阅还有 {{days_remaining}} 天到期，到期时间是 {{expiry_time}}。

如果还需要继续使用，请记得提前续费，避免到期后影响 API 调用和相关服务。

如果您已经续费，忽略这封邮件就好。

这是一封自动提醒邮件。不再接收此类提醒
```

实际入库 HTML 使用内联样式，保留 `{{unsubscribe_url}}` 退订链接。

## 验证

- settings 中 `zh` 与 `en` 模板均存在，主题均为中文。
- `sub2api-candidate`、PostgreSQL、Redis 容器均 healthy。
- `http://127.0.0.1:18084/health`：HTTP 200，`{"status":"ok"}`
- `https://api.aaccx.pw/health`：HTTP 200，`{"status":"ok"}`

本轮未改代码、未重启服务、未修改 SMTP、用户、订阅、订单、API Key、Redis、镜像或容器，未向真实用户发送测试邮件。
