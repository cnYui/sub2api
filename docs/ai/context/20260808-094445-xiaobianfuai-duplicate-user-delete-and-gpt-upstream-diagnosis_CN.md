# xiaobianfuai 重复用户清理与 GPT 不可用诊断

## ID 461 硬删除

- `xiaobianfuai@gmail.com` 曾同时存在两条用户记录：`448` 为有效账户，`461` 为 `2026-06-20` 已软删除的旧记录。
- 按管理员要求，精确执行 `DELETE FROM users WHERE id=461 AND email='xiaobianfuai@gmail.com' AND deleted_at IS NOT NULL`。删除预演和正式事务均只命中 ID `461`。
- 删除级联移除了旧 API Key `id=33`（名称 `ceshi`）；该旧记录没有用量、余额套餐或流量卡。
- 清理旧 Key 的 Redis 认证缓存并广播失效事件后，使用该 Key 请求 `/v1/models` 返回 `401 INVALID_API_KEY`。有效用户 `448` 仍存在，余额保持 `99930.87224958 USD`。
- 保留 `audit_logs.id=787` 的历史系统安全审计。该记录不受外键约束，保留它可追溯当时的安全暂停操作，不再关联到可登录用户。

## GPT API 实测与根因

- 有效用户 `448` 的 GPT Key `id=227` 为 `active`、无配额或有效期限制，分组为 `9`（Codex 日常）；公网 `GET https://api.aaccx.pw/v1/models` 返回 `200`，模型列表为 20 个。
- 用同一 Key 通过公网调用 `POST /v1/responses`，模型为 `gpt-5.6-luna`，返回 `502`；请求前后余额与用量记录数均未变化。
- `ops_error_logs.id=9583` 记录该失败来自账号 `1128` 的上游 HTTP `502`，非用户认证、余额或流量卡问题。该分组只有账号 `1128`，失败后没有备用账号可切换。
- 同日用户的 `gpt-5.6-terra` 流式请求也由账号 `1128` 产生上游错误“Your account is not active, please check your billing details on our website.”；上游错误已写入响应，客户端即使看到 HTTP `200` 也会收到失败内容。
- 本地账号 `1128` 当前仍标记为 `active/schedulable`，但上游凭证或上游账户已失效。修复必须续费/恢复该上游账户，或为 GPT 分组补充并启用备用账号；不能靠给用户余额、重建容器或修改用户 Key 解决。
