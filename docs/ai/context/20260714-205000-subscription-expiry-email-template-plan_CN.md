# 订阅到期提醒邮件中文模板调整计划

## 背景

用户要求订阅到期提醒邮件内容使用中文，并写得自然一些。

当前运行态状态：

- `subscription_expiry_notify_enabled=true`
- SMTP 已配置
- `subscription.expiry_reminder` 没有运行态自定义模板覆盖，当前会使用内置默认模板
- `site_name` 未配置，运行时会回退为 `Sub2API`
- `frontend_url=https://aaccx.pw`

## 目标

把 `subscription.expiry_reminder` 的运行态邮件模板改为自然中文。

## 方案

为避免用户没有记录语言偏好时落到英文模板，运行态同时覆盖：

- `notification_email_template:subscription.expiry_reminder:zh`
- `notification_email_template:subscription.expiry_reminder:en`

两个 locale 使用相同中文内容。

## 文案口径

- 主题：`[天才程序员小站] 您的会员订阅还有 {{days_remaining}} 天到期`
- 正文说明：
  - 称呼用户
  - 告知订阅分组和剩余天数
  - 展示到期时间
  - 提醒及时续费以避免影响使用
  - 已续费可忽略
  - 保留退订此类提醒链接

## 边界

- 不修改代码。
- 不重启服务。
- 不修改 SMTP、用户、订阅、订单、API Key、Redis 或容器。
- 不发送测试邮件给真实用户。
- 写入前备份 PostgreSQL，写入后验证 settings 与健康检查。
