# 开放注册与 SMTP 测试记录

## 目标

- 打开当前 `sub2api-official-18082` 实例的公开用户注册功能。
- 复用已保存的 SMTP 配置，向 `xiaobianfuai@gmail.com` 发送一封测试邮件。

## 决策

- 注册开关属于数据库 `settings` 表中的 `registration_enabled`，不修改源码默认值，避免影响其他部署。
- SMTP 测试使用现有管理端 `POST /api/v1/admin/settings/send-test-email`，不读取或输出 SMTP 密码。

## 验证

- `GET http://127.0.0.1:18082/api/v1/settings/public` 返回 `registration_enabled=true`，确认注册已开启并持久化。
- `POST /api/v1/admin/settings/send-test-email` 返回 `code=0`、`Test email sent successfully`。
- `sub2api-official-18082` 日志记录该发送接口 HTTP 200，耗时约 2522 ms。
- 当前 SMTP 发件配置为 `smtp.gmail.com:587`、发件人 `xiaobianfuai@gmail.com`；密码未读取或输出。
