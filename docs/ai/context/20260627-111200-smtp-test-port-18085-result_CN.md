# 2026-06-27 SMTP 测试端口 18085 启动与文档缺失排查结果

## 18085 本地测试实例

已另起隔离实例：

- 应用容器：`sub2api-smtp-test`
- 镜像：`sub2api-smtp-test:20260627-111200`
- 访问地址：`http://127.0.0.1:18085`
- 端口映射：`127.0.0.1:18085 -> container:8080`
- 数据库容器：`sub2api-smtp-test-postgres`
- Redis 容器：`sub2api-smtp-test-redis`
- Docker 网络：`sub2api-smtp-test-net`

不影响当前公网链路：

- `8080 -> 18084` 保持不动。
- `sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis` 未重建、未停止。

## 启动修正

第一次应用容器启动失败，日志为：

- `invalid totp encryption key: encoding/hex`

原因是测试环境变量 `TOTP_ENCRYPTION_KEY` 使用了非 hex 字符串。已只重建 `sub2api-smtp-test` 应用容器，改用合法 hex key 后启动成功。

## 当前 18085 配置状态

已在测试库打开：

- `registration_enabled=true`
- `email_verify_enabled=true`
- `password_reset_enabled=true`
- `turnstile_enabled=false`

SMTP 仍未配置真实账号：

- `smtp_host` 未配置
- `smtp_username` 未配置
- `smtp_password` 未配置
- `smtp_from` 未配置
- `smtp_port=587`
- `smtp_use_tls=true`
- `smtp_from_name=Sub2API SMTP Test`

验证：

- `GET http://127.0.0.1:18085/health` 返回 `{"status":"ok"}`。
- `GET http://127.0.0.1:18085/api/v1/settings/public` 返回注册、邮箱验证、密码重置均为 true。
- `POST /api/v1/auth/send-verify-code` 当前返回 200，但 worker 仍报 `EMAIL_NOT_CONFIGURED`，符合“入队成功但 SMTP 未配置”的已知行为。

## SMTP 文档与分支调查

存在两个关键文档：

- `docs/ai/context/20260619-093831-email-smtp-password-reset-current-state_CN.md`
  - 被 git 跟踪。
  - 记录当时运行态尚未配置 SMTP。
- `docs/ai/context/20260619-220357-gmail-smtp-enabled-result_CN.md`
  - 本地文件存在。
  - 被 `.gitignore: docs/*` 忽略，未被 git 跟踪。
  - 未出现在任何本地/远端 git ref 中。
  - 当前 HEAD 的 `AGENTS.md` 曾引用它，但该文件本身没有进分支。

分支调查结论：

- 多个本地分支的 `AGENTS.md` 记录过“SMTP 已配置为 Gmail 发信”。
- 未发现把 Gmail SMTP 运行态配置固化成源码、SQL 迁移或部署脚本的分支。
- SMTP 配置本质是数据库 `settings` 运行态数据，不会因为代码分支合并自动出现。
- 18084 候选环境缺 SMTP 更可能来自候选库清洗或新库未恢复运行态 settings，而不是业务代码缺少 SMTP 功能。

## 关键判断

当前“最新版本没有 SMTP 修改”的准确表述应是：

1. SMTP 功能代码在当前 main 已存在。
2. Gmail SMTP 的具体配置曾经写入某个运行态数据库。
3. 记录“已配置 Gmail SMTP”的结果文档没有被 git 跟踪，属于本地 ignored 文档。
4. 当前 18084 候选库的 SMTP settings 为空，因此新用户验证码发不出。
5. 如果要恢复真实发信，需要重新把 SMTP 配置写入目标运行态数据库，并建议先更换曾暴露过的 Gmail 应用专用密码。
