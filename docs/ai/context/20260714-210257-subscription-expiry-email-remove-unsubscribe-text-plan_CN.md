# 订阅到期提醒邮件删除退订文案计划

## 背景

用户要求删除订阅到期提醒邮件最后部分的退订链接文字。

当前运行态已有两个自定义模板：

- `notification_email_template:subscription.expiry_reminder:zh`
- `notification_email_template:subscription.expiry_reminder:en`

两个模板内容一致，均为中文。

## 目标

删除邮件末尾的退订链接文字，只保留自然收尾：

```text
这是一封自动提醒邮件。
```

## 执行方案

1. 写入前备份 PostgreSQL。
2. 更新 `zh` 与 `en` 两个订阅到期提醒模板。
3. 复核模板主题不变，正文不再包含末尾退订链接文字。
4. 复核内外 health 与容器健康状态。

## 边界

- 不修改代码。
- 不重启服务。
- 不修改 SMTP、用户、订阅、订单、API Key、Redis、镜像或容器。
- 不发送测试邮件给真实用户。
