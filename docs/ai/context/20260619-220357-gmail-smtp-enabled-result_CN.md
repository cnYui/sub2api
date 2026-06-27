# Gmail SMTP 配置结果

## 结果

- 已为当前运行态 Sub2API 配置 Gmail SMTP。
- 发件邮箱：`xiaobianfuai@gmail.com`
- SMTP host：`smtp.gmail.com`
- SMTP port：`587`
- `smtp_use_tls=false`，由发送逻辑在 587 端口走 STARTTLS。
- `smtp_from_name=AACCX`
- `frontend_url=https://aaccx.pw`
- `email_verify_enabled=true`
- `password_reset_enabled=true`

## 验证

- 数据库脱敏读取确认 `smtp_password=<configured>`，开关已开启。
- Python `smtplib` 使用当前数据库中的 SMTP 凭据登录 Gmail SMTP 成功，输出 `SMTP_LOGIN_OK`。
- `GET /api/v1/settings/public` 返回：
  - `registration_enabled=true`
  - `email_verify_enabled=true`
  - `password_reset_enabled=true`
- `POST /api/v1/auth/forgot-password` 对不存在邮箱返回 200，不暴露用户是否存在。
- `POST /api/v1/auth/send-verify-code` 对 `xiaobianfuai@gmail.com` 返回 `EMAIL_EXISTS`，说明该邮箱已是当前用户，注册验证码接口按预期拒绝重复注册。
- `POST /api/v1/auth/forgot-password` 对 `xiaobianfuai@gmail.com` 返回 200，并且日志确认队列 worker 已发送密码重置邮件。

## 注意

- 不要在文档、提交或日志里记录 Gmail 应用专用密码。
- 该应用专用密码已经在对话中出现过；为了长期安全，建议后续重新生成一个新的 Gmail 应用专用密码并替换当前 `smtp_password`。
- 现有迁移用户里仍有 `@phone.com` 这类不可收信地址；这些用户使用忘记密码时不会收到邮件，需要后续把用户邮箱替换成真实邮箱，或保留管理员人工重置路径。
