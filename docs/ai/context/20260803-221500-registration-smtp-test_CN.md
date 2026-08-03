# 开放注册与 SMTP 测试记录

## 目标

- 打开当前 `sub2api-official-18082` 实例的公开用户注册功能。
- 复用已保存的 SMTP 配置，向 `xiaobianfuai@gmail.com` 发送一封测试邮件。

## 决策

- 注册开关属于数据库 `settings` 表中的 `registration_enabled`，不修改源码默认值，避免影响其他部署。
- SMTP 测试使用现有管理端 `POST /api/v1/admin/settings/send-test-email`，不读取或输出 SMTP 密码。

## 验证

- 待执行：确认 `registration_enabled=true` 持久化。
- 待执行：确认测试邮件接口成功，并检查 `sub2api-official-18082` 日志。
