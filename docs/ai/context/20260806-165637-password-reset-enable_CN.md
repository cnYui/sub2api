# 忘记密码功能启用记录

时间：2026-08-06 16:56:37

## 变更

- 在运行中的 `18082` PostgreSQL 设置中原子写入 `email_verify_enabled=true`、`password_reset_enabled=true` 与 `frontend_url=https://aaccx.pw`。
- 密码重置功能依赖邮箱验证开关；因此新注册用户将进入现有的邮箱验证流程。
- SMTP 配置保持不变，继续使用现有 Gmail SMTP 主机、端口、发件人和 TLS 配置；未读取、修改或输出 SMTP 密码。

## 验证

- `GET http://127.0.0.1:18082/api/v1/settings/public` 返回 `email_verify_enabled=true` 与 `password_reset_enabled=true`。
- 对不存在的匿名测试邮箱调用 `POST /api/v1/auth/forgot-password` 返回通用成功响应，未触发实际邮件投递。
- `go test -tags=unit ./internal/service -run '^(TestSMTP|TestBuildPasswordResetEmailBody|TestEmailQueueTasksPreserveLocaleHints)$' -count=1` 通过。
- `sub2api-official-18082` 容器健康状态为 `healthy`。

## 边界

- 重置邮件会由现有异步队列使用已配置 SMTP 投递，令牌为一次性令牌并在重置成功后撤销用户会话。
- 未发送测试邮件，避免在未指定收件人的情况下产生外部投递；首次真实忘记密码请求将完成端到端 SMTP 投递验证。
