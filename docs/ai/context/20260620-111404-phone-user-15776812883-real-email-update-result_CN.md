# 手机号迁移用户真实邮箱更新结果

## 结果

- 已将 `users.id=12` 的身份字段从 `15776812883@phone.com` 更新为 `liyutong2883@gmail.com`。
- 已同步更新：
  - `users.email`
  - `users.username`
  - `auth_identities.provider_subject`
- 已清理该用户 active API Key 对应的 Redis API Key auth cache。

## 未改动内容

- 用户 id 未变：`12`
- 用户角色未变：`user`
- 用户状态未变：`active`
- API Key 绑定未变：仍有 1 个 active API Key 绑定 `user_id=12`
- 套餐、订阅、余额、用量、密码 hash 均未修改。

## 验证

- 旧邮箱 active 用户数：`0`
- 新邮箱 active 用户数：`1`
- 新邮箱 email 身份数：`1`
- `users.id=12` 当前：
  - `email=liyutong2883@gmail.com`
  - `username=liyutong2883@gmail.com`
  - `role=user`
  - `status=active`
- `auth_identities.id=12` 当前：
  - `user_id=12`
  - `provider_type=email`
  - `provider_key=email`
  - `provider_subject=liyutong2883@gmail.com`
  - `verified=true`
