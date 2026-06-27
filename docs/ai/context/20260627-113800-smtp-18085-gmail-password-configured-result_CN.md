# 2026-06-27 18085 Gmail SMTP 密码配置与发信验证结果

## 结果

已在本地隔离实例 `http://127.0.0.1:18085` 的测试库中写入 Gmail App Password，并完成一次注册验证码发信验证。

## 配置状态

脱敏查询确认：

- `smtp_host=smtp.gmail.com`
- `smtp_port=587`
- `smtp_username=xiaobianfuai@gmail.com`
- `smtp_from=xiaobianfuai@gmail.com`
- `smtp_from_name=AACCX`
- `smtp_use_tls=false`
- `smtp_password=[CONFIGURED]`
- `registration_enabled=true`
- `email_verify_enabled=true`
- `password_reset_enabled=true`

## 验证

触发接口：

- `POST http://127.0.0.1:18085/api/v1/auth/send-verify-code`

结果：

- HTTP 返回 `200`
- 后台日志显示邮件队列 worker 已成功发送验证码邮件
- 不再出现 `EMAIL_NOT_CONFIGURED`

## 安全

- 未在文档、git、日志摘要或最终回复中记录 Gmail App Password 明文。
- Gmail App Password 属于 secret，只能保留在运行态数据库中；后续提交只允许包含脱敏状态和验证结果。
