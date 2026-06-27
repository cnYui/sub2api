# 手机号用户 19814722044 真实邮箱更新计划

## 目标

将手机号迁移用户 `19814722044@phone.com` 的登录身份更新为真实邮箱 `varmons@proton.me`。

## 当前确认

- 当前用户为 `users.id=21`，`email/username=19814722044@phone.com`，角色为普通用户，状态为 `active`。
- 当前邮箱身份为 `auth_identities.id=21`，指向 `users.id=21`，`provider_subject=19814722044@phone.com`。
- 目标邮箱 `varmons@proton.me` 当前未被活跃用户占用，也未被现有邮箱身份占用。
- 该用户有一个 active API Key，属于 `codex-pool-19-usd`；本次不修改 API Key、订阅、余额、用量、密码 hash、角色或状态。

## 执行方案

1. 在 PostgreSQL 事务内锁定 `users.id=21`。
2. 前置校验用户仍是未删除活跃用户，且 `email/username` 仍为 `19814722044@phone.com`。
3. 前置校验目标邮箱 `varmons@proton.me` 未被活跃用户和邮箱身份占用。
4. 更新：
   - `users.email = 'varmons@proton.me'`
   - `users.username = 'varmons@proton.me'`
   - `auth_identities.provider_subject = 'varmons@proton.me'`
5. 清理该用户 active API Key 的 Redis auth cache。
6. 读回验证：
   - 活跃目标邮箱用户数量为 1，且为 `users.id=21`。
   - 活跃旧手机号邮箱用户数量为 0。
   - 邮箱 identity 指向 `users.id=21`。
   - 用户 API Key、订阅、余额、用量、密码 hash 未改动。

## 风险与回滚

- 风险点是目标邮箱可能在执行时被其他账号占用，因此事务内再次校验唯一性。
- 若任一前置校验失败，事务主动报错并回滚。
- 回滚可按本文件记录将 `users.email/users.username/auth_identities.provider_subject` 改回 `19814722044@phone.com`，但通常不应回滚真实邮箱迁移。
