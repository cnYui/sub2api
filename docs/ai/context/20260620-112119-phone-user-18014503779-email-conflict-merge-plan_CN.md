# 手机号迁移用户真实邮箱冲突合并计划

## 目标

- 将 `18014503779@phone.com` 对应的有权益用户改为真实邮箱 `1915474749@qq.com`。
- 保留原手机号迁移用户的 API Key、订阅、用量和余额。

## 冲突

目标邮箱 `1915474749@qq.com` 已经存在一个 active 用户：

- 有权益主账号：`users.id=16`
  - `email=18014503779@phone.com`
  - active API Key 数量为 1
  - 有订阅和用量记录
- 真实邮箱空壳账号：`users.id=25`
  - `email=1915474749@qq.com`
  - active API Key 数量为 0
  - 无订阅、无用量、无支付订单
  - 有 1 条 email identity
  - 有 1 条 user affiliate 记录
  - 有 4 条默认 user platform quota 记录

直接把 `id=16` 改成 `1915474749@qq.com` 会撞 active users 邮箱唯一约束。

## 合并策略

在单个数据库事务中：

1. 锁定 `users.id=16` 和 `users.id=25`。
2. 校验 `id=16` 是旧手机号邮箱 active 用户。
3. 校验 `id=25` 是目标真实邮箱 active 用户。
4. 校验 `id=25` 无 API Key、订阅、用量、支付订单。
5. 将 `id=25` 的 `user_affiliates` 记录迁到 `id=16`。
6. 将 `id=25` 的 `user_platform_quotas` 记录迁到 `id=16`。
7. 将 `id=25` 的真实邮箱 `auth_identities` 迁到 `id=16`。
8. 删除 `id=16` 的旧 `18014503779@phone.com` email identity。
9. 软删除 `users.id=25`。
10. 将 `users.id=16` 的 `email` 和 `username` 更新为 `1915474749@qq.com`。
11. 清理 `id=16` 的 API Key auth cache。

## 不做的事

- 不复制 `id=25` 的密码 hash 到 `id=16`。
- 不修改 `id=16` 的 API Key、订阅、余额、用量、角色、状态。
- 若用户不知道 `id=16` 的原密码，应通过邮箱忘记密码重置。
