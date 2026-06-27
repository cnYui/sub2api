# 管理员账号邮箱替换与测试账号删除计划

## 目标

将当前 `15951875192@phone.com` 对应的管理员账号改为 `xiaobianfuai@gmail.com`，并删除占用该邮箱的普通测试账号。

## 当前确认

- 管理员账号为 `users.id=13`，当前邮箱和用户名为 `15951875192@phone.com`，角色为 `admin`，状态为 `active`。
- 管理员账号持有本机无限额 API Key，属于 `codex-pool-local-unlimited`，本次不修改该 Key、订阅、余额、用量、密码 hash、角色或状态。
- 普通测试账号为 `users.id=26`，邮箱和用户名为 `xiaobianfuai@gmail.com`，角色为 `user`，状态为 `active`。
- 测试账号持有一个测试 API Key，属于 `codex-pool-49-usd`，删除测试账号时需写入 `deleted_api_key_audits`，再软删除该 Key。
- `auth_identities` 对 `(provider_type, provider_key, provider_subject)` 有唯一约束，需要先释放测试账号的邮箱身份，再让管理员占用该邮箱身份。

## 执行方案

1. 在 PostgreSQL 事务内锁定 `users.id IN (13, 26)`。
2. 前置校验：
   - `id=13` 必须是未删除管理员，邮箱为 `15951875192@phone.com`。
   - `id=26` 必须是未删除普通用户，邮箱为 `xiaobianfuai@gmail.com`。
3. 对测试账号 `id=26`：
   - 将其未删除 API Key 写入 `deleted_api_key_audits`。
   - 用 tombstone 覆盖其 API Key 明文值，并设置 `deleted_at/updated_at`。
   - 删除其 `auth_identities`，释放 `xiaobianfuai@gmail.com` 身份占用。
   - 软删除该用户，并在 `notes` 标记为被管理员邮箱替换流程删除。
4. 对管理员 `id=13`：
   - 将 `users.email` 和 `users.username` 更新为 `xiaobianfuai@gmail.com`。
   - 将旧邮箱身份 `15951875192@phone.com` 更新为 `xiaobianfuai@gmail.com`，保持邮箱 provider。
   - 保持 `role='admin'`。
5. 清理 Redis 中管理员 Key 和测试 Key 的鉴权缓存。
6. 读回验证：
   - 活跃 `xiaobianfuai@gmail.com` 用户只有 `id=13`，且仍是 `admin`。
   - 活跃 `15951875192@phone.com` 用户为 0。
   - `users.id=26` 已软删除。
   - `auth_identities` 中 `xiaobianfuai@gmail.com` 指向 `user_id=13`。
   - 管理员 API Key 仍未删除；测试 API Key 已软删除且有审计。

## 风险与回滚

- 风险点是目标邮箱已被测试账号占用，因此必须在同一事务中先删除测试身份再更新管理员身份。
- 若任一前置校验不通过，事务会主动失败并回滚。
- 若事务执行后需要恢复，可用 `deleted_at`、`deleted_api_key_audits` 和变更文档定位本次修改，但不应恢复测试账号占用管理员邮箱。
