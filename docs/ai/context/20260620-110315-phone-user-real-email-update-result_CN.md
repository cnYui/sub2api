# 手机号迁移用户真实邮箱更新结果

## 结果

- 已将 `users.id=3` 的身份字段从 `13052071067@phone.com` 更新为 `milesyang987@gmail.com`。
- 已同步更新：
  - `users.email`
  - `users.username`
  - `auth_identities.provider_subject`
- 已清理该用户 active API Key 对应的 Redis API Key auth cache。

## 未改动内容

- 用户 id 未变：`3`
- 用户角色未变：`user`
- 用户状态未变：`active`
- 注册来源未变：`email`
- API Key 绑定未变：仍有 1 个 active API Key 绑定 `user_id=3`
- 套餐、订阅、余额、用量、密码 hash 均未修改。

## 验证

- 旧邮箱 active 用户数：`0`
- 新邮箱 active 用户数：`1`
- 新邮箱 email 身份数：`1`
- `users.id=3` 当前：
  - `email=milesyang987@gmail.com`
  - `username=milesyang987@gmail.com`
  - `role=user`
  - `status=active`
- `auth_identities.id=3` 当前：
  - `user_id=3`
  - `provider_type=email`
  - `provider_key=email`
  - `provider_subject=milesyang987@gmail.com`
  - `verified=true`
