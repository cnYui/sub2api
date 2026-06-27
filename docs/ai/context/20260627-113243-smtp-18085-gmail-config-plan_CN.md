# 2026-06-27 18085 Gmail SMTP 配置计划

## 目标

在本地 `http://127.0.0.1:18085` 的隔离测试实例中配置 Gmail SMTP，用于验证注册邮箱验证码和忘记密码邮件。

## 边界

- 只修改 `sub2api-smtp-test-postgres` 测试库。
- 不修改 `sub2api-candidate`、`sub2api-candidate-postgres`、公网 `8080 -> 18084` 链路。
- 不把 Gmail 应用专用密码、SMTP 密码或任何 secret 写入文档、命令输出或 git。
- 只把非敏感配置和结果文档纳入 git。

## 配置设计

使用 Gmail SMTP：

- `smtp_host=smtp.gmail.com`
- `smtp_port=587`
- `smtp_username=xiaobianfuai@gmail.com`
- `smtp_from=xiaobianfuai@gmail.com`
- `smtp_from_name=AACCX`
- `smtp_use_tls=false`

说明：当前后端在 `smtp_use_tls=false` 时会使用普通连接并在服务端支持时升级 STARTTLS；这与 Gmail `587` 端口匹配。`smtp_use_tls=true` 更接近直连 TLS，通常用于 `465`。

## 密码获取

Gmail SMTP 密码不是 Google 登录密码，而是 Google Account 的 16 位 App Password。需要账号开启 2-Step Verification 后，在 Google Account 的 App passwords 页面生成。

## 验证

1. 写入非敏感 SMTP 字段后，脱敏查询 settings。
2. 因密码仍为空，发送验证码预期仍会报 `EMAIL_NOT_CONFIGURED` 或认证失败；等填入 App Password 后再验证成功发送。
3. 把本计划和结果文档用 `git add -f` 纳入 git，避免再次被 `.gitignore: docs/*` 漏掉。
