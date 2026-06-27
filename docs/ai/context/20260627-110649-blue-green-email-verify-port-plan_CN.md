# 2026-06-27 蓝绿测试邮箱验证码与端口规划

## 背景

用户要求继续做新版本蓝绿测试，并指出本地 main 分支新用户注册时无法发送邮箱验证码。按 `using-superpowers` 和 `systematic-debugging` 先做根因定位，不先改代码。

## 当前运行态证据

- `8080` 当前由 nginx 监听，反代到 `sub2api-candidate` 的 `127.0.0.1:18084`。
- `18084` 当前是公网候选链路后端，容器为 `sub2api-candidate`。
- `18082` 当前已有 `sub2api-main-preview`，看起来是旧的 main 预览实例。
- `3000`、`5174`、`5432`、`5433`、`6379`、`6380` 当前宿主机未监听。

## 邮箱验证码根因

### 18084 候选环境

公开配置显示：

- `registration_enabled=true`
- `email_verify_enabled=true`
- `turnstile_enabled=false`

数据库脱敏配置显示：

- `smtp_host` 为空
- `smtp_username` 为空
- `smtp_password` 未配置
- `smtp_from` 为空
- `smtp_port=587`
- `smtp_use_tls=false`

实际请求验证：

- `POST http://127.0.0.1:18084/api/v1/auth/send-verify-code` 返回 `200`，提示 `Verification code sent successfully`。
- 随后后台邮件队列日志报错：`EMAIL_NOT_CONFIGURED`。

结论：18084 的问题不是前端没有请求，也不是注册逻辑没有走到后端，而是 `SendVerifyCodeAsync` 只保证邮件任务入队成功；真正发送在异步 worker 中执行。当前候选库 SMTP 配置被清空，所以接口表面成功，实际邮件发送失败。

### 18082 main-preview

公开配置显示：

- `registration_enabled=false`
- `email_verify_enabled=false`
- `payment_enabled=false`

数据库 settings 查询关键项为空，说明它处在默认/未配置态。

实际请求验证：

- `POST http://127.0.0.1:18082/api/v1/auth/send-verify-code` 返回 `403 REGISTRATION_DISABLED`。

结论：18082 的问题是注册未开启，不是 SMTP 发送失败。

## 蓝绿测试端口规划

为避免污染当前公网链路，不要让本地 main 测试占用 `8080` 或 `18084`。

建议端口：

- 公网入口保留：`8080 -> 18084`，不要动。
- 当前候选后端保留：`18084`，不要动。
- 本地 main 后端：`18082` 可复用现有 `sub2api-main-preview`；若重建新 main 实例，继续使用 `127.0.0.1:18082 -> 8080`。
- 本地前端 dev：优先使用 `5174`，通过 `VITE_DEV_PROXY_TARGET=http://127.0.0.1:18082` 指向 main 后端。
- 本地 main PostgreSQL：容器内 `5432`，默认不暴露宿主机；如需调试，宿主机映射建议用 `127.0.0.1:15432 -> 5432`，不要用生产默认 `5432`。
- 本地 main Redis：容器内 `6379`，默认不暴露宿主机；如需调试，宿主机映射建议用 `127.0.0.1:16379 -> 6379`。
- 若另起新候选绿环境，建议使用 `18085`，避免覆盖当前 18084。

## 下一步建议

1. 若测试真实邮箱验证码，必须给对应测试库恢复 SMTP 配置；只记录“已配置/未配置”，不要把 SMTP 密码写入文档或日志。
2. 对 18084 候选，如果它要继续承接公网，不能把 `email_verify_enabled=true` 与空 SMTP 同时保留，否则新用户会卡在验证码页。
3. 对 18082 main-preview，如果它用于本地 main 蓝绿测试，先启用 `registration_enabled` 和必要的 `email_verify_enabled`，再配置 SMTP。
4. 中长期应考虑调整 `SendVerifyCodeAsync` 的语义：如果 SMTP 未配置，接口不应返回发送成功；至少应在入队前做 SMTP 配置可用性检查，避免前端误导用户。
