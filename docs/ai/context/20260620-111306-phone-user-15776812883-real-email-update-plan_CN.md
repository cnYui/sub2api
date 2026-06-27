# 手机号迁移用户真实邮箱更新计划

## 目标

- 将 `15776812883@phone.com` 对应的 Sub2API 用户改为真实邮箱 `liyutong2883@gmail.com`。
- 只改身份字段，不改变 API Key、订阅、余额、套餐、用量、角色、状态和密码。

## 当前确认

- 目标用户存在：
  - `users.id=12`
  - `users.email=15776812883@phone.com`
  - `users.username=15776812883@phone.com`
  - `role=user`
  - `status=active`
  - active API Key 数量为 1
- 目标真实邮箱当前未被 active 用户占用。
- 目标真实邮箱当前未被 email provider 身份占用。
- 用户存在一条 `auth_identities` email 身份：
  - `provider_type=email`
  - `provider_key=email`
  - `provider_subject=15776812883@phone.com`

## 变更范围

在单个数据库事务中更新：

- `users.email`
- `users.username`
- `users.updated_at`
- `auth_identities.provider_subject`
- `auth_identities.updated_at`

不更新：

- API Key
- 订阅
- group 绑定
- usage logs
- 余额
- 并发
- 密码 hash
- 用户状态
- 角色

## 验证

- 查询旧邮箱不再存在于 active users。
- 查询新邮箱存在且用户 id 不变。
- 查询 `auth_identities` 的 email 身份已同步为新邮箱。
- 查询 API Key 仍绑定原用户 id。
