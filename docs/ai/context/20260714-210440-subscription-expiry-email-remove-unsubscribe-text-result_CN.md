# 订阅到期提醒邮件删除退订文案结果

## 结论

已按用户要求删除订阅到期提醒邮件末尾的退订链接文字。

## 备份

写入前备份 PostgreSQL：

- `deploy/backups/20260714-210257-sub2api-candidate-before-remove-subscription-expiry-unsubscribe-text.dump`
- 权限 `600`
- 已用容器内 `pg_restore -l` 验证可读，TOC Entries 为 `930`

## 写入内容

更新 settings：

- `notification_email_template:subscription.expiry_reminder:zh`
- `notification_email_template:subscription.expiry_reminder:en`

主题保持：

```text
[天才程序员小站] 您的会员订阅还有 {{days_remaining}} 天到期
```

正文末尾从退订链接收尾改为：

```text
这是一封自动提醒邮件。
```

## 验证

- 两个模板中用户要求删除的退订文字位置均为 `0`。
- 两个模板中 `unsubscribe_url` 占位位置均为 `0`。
- 两个模板中 `这是一封自动提醒邮件。` 均存在。
- `sub2api-candidate`、PostgreSQL、Redis 容器均 healthy。
- `http://127.0.0.1:18084/health`：HTTP 200，`{"status":"ok"}`
- `https://api.aaccx.pw/health`：HTTP 200，`{"status":"ok"}`

本轮未改代码、未重启服务、未修改 SMTP、用户、订阅、订单、API Key、Redis、镜像或容器，未向真实用户发送测试邮件。
