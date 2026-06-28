# 公网 18084 注册邮箱验证码无法发送排查结果

时间：2026-06-28 15:27 JST

## 问题

公网新用户注册时，`/api/v1/auth/send-verify-code` 表面返回成功或验证码任务入队，但用户邮箱收不到 SMTP 验证码。

## 当前运行态

- 公网应用容器：`sub2api-candidate`
- 镜像：`sub2api-candidate:20260627-221441-traffic-card-fix`
- 状态：healthy
- 端口：`127.0.0.1:18084->8080`
- 测试应用容器：`sub2api-smtp-test`
- 镜像：`sub2api-smtp-test:20260627-221441-traffic-card-fix`
- 状态：healthy
- 端口：`127.0.0.1:18085->8080`

## 日志证据

公网 18084 日志中出现：

```text
[Auth] Enqueueing verify code for: mf2604302@g.u-fukui.ac.jp
[EmailQueue] Enqueued verify code task for mf2604302@g.u-fukui.ac.jp
[Auth] Verify code enqueued successfully for: mf2604302@g.u-fukui.ac.jp
http request completed path=/api/v1/auth/send-verify-code status_code=200
[EmailQueue] Worker 0 failed to send verify code ... EMAIL_NOT_CONFIGURED email service not configured
```

说明：

- HTTP 接口成功只是“邮件任务入队成功”。
- 真正发信在异步 worker 中执行。
- worker 失败原因是 `EMAIL_NOT_CONFIGURED`。

## 运行态配置对比

公网 18084 对应数据库 `sub2api-candidate-postgres`：

- `registration_enabled=true`
- `email_verify_enabled=true`
- `password_reset_enabled=true`
- `smtp_host=[EMPTY]`
- `smtp_username=[EMPTY]`
- `smtp_from=[EMPTY]`
- `smtp_password=[EMPTY]`
- `smtp_port=587`
- `smtp_use_tls=false`
- `smtp_from_name=Sub2API Candidate`

18085 测试数据库 `sub2api-smtp-test-postgres`：

- `registration_enabled=true`
- `email_verify_enabled=true`
- `password_reset_enabled=true`
- `smtp_host=smtp.gmail.com`
- `smtp_username=[CONFIGURED]`
- `smtp_from=[CONFIGURED]`
- `smtp_password=[CONFIGURED]`
- `smtp_port=587`
- `smtp_use_tls=false`
- `smtp_from_name=AACCX`

## 根因

公网 18084 当前运行的是最新应用镜像，但它保留的是 `sub2api-candidate-postgres` 这套候选公网数据库。该数据库中的 SMTP 配置仍为空。

之前“邮箱验证码已修复”的状态发生在 18085 测试库 `sub2api-smtp-test-postgres`，那里写入过 Gmail SMTP 配置和 App Password；这些是运行态数据库 settings，不在源码、迁移或镜像里。后来替换 18084 应用容器时按要求保留了 18084 数据层，因此 18085 的 SMTP 配置没有自动进入 18084。

## 结论

这不是最新代码没有部署，也不是前端问题；公网注册邮箱验证码无法发送的直接原因是：公网 18084 数据库缺少 SMTP 运行态配置。

要恢复公网注册发信，需要把 SMTP 非敏感字段和已配置的 App Password 写入 `sub2api-candidate-postgres` 的 `settings`，或通过后台管理页重新配置 SMTP。密码明文不得进入文档、提交或日志。
