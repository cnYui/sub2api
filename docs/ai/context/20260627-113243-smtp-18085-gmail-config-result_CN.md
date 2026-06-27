# 2026-06-27 18085 Gmail SMTP 非敏感配置结果

## 结果

已在本地隔离实例 `http://127.0.0.1:18085` 对应测试库 `sub2api-smtp-test-postgres` 写入 Gmail SMTP 非敏感配置。

## 已写入配置

- `registration_enabled=true`
- `email_verify_enabled=true`
- `password_reset_enabled=true`
- `smtp_host=smtp.gmail.com`
- `smtp_port=587`
- `smtp_username=xiaobianfuai@gmail.com`
- `smtp_from=xiaobianfuai@gmail.com`
- `smtp_from_name=AACCX`
- `smtp_use_tls=false`

## 未写入配置

- `smtp_password` 仍未配置。

原因：Gmail SMTP 密码必须使用 Google Account 生成的 16 位 App Password；它属于 secret，不应写入文档、提交或长期脚本。

## 如何获取密码

- SMTP 用户名：`xiaobianfuai@gmail.com`
- SMTP 密码：Google Account 的 App Password，不是 Gmail 登录密码。
- 需要先开启 2-Step Verification。
- 进入 Google Account 的 App passwords 页面，生成一个新的 App Password；该密码只显示一次，后续丢失只能重新生成。

## 后续验证

填入 App Password 后，验证顺序：

1. 管理后台测试 SMTP 连接。
2. 管理后台发送测试邮件。
3. 新注册邮箱触发 `/api/v1/auth/send-verify-code`。
4. 查看 `sub2api-smtp-test` 日志，确认不再出现 `EMAIL_NOT_CONFIGURED` 或 SMTP 认证错误。

## Git 记录要求

本次只把非敏感计划和结果文档纳入 git；不得把 App Password、SMTP 密码或任何 secret 纳入 git。
