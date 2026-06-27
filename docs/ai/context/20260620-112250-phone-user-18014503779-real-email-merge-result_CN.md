# 手机号迁移用户真实邮箱合并结果

## 结果

- 已将有权益主账号 `users.id=16` 从 `18014503779@phone.com` 更新为 `1915474749@qq.com`。
- 目标真实邮箱原本已存在空壳账号 `users.id=25`，该账号已软删除并合并到 `users.id=16`。

## 合并前

- `users.id=16`
  - `email=18014503779@phone.com`
  - 有 1 个 active API Key
  - 有订阅和用量记录
- `users.id=25`
  - `email=1915474749@qq.com`
  - 无 API Key、无订阅、无用量、无支付订单
  - 有 email identity
  - 有 1 条 user affiliate 记录
  - 有 4 条 user platform quota 默认记录

## 合并后

- `users.id=16`
  - `email=1915474749@qq.com`
  - `username=1915474749@qq.com`
  - `role=user`
  - `status=active`
  - active API Key 数量仍为 1
  - 订阅和用量仍绑定 `user_id=16`
  - email identity 已为 `1915474749@qq.com`
  - user affiliate 记录已迁到 `user_id=16`
  - user platform quota 记录已迁到 `user_id=16`
- `users.id=25`
  - 已软删除
  - notes 记录 `merged_into_user_id=16 for real_email_ownership_20260620`

## 未改动内容

- `users.id=16` 的 API Key 未改动。
- `users.id=16` 的订阅、余额、用量、密码 hash、角色、状态未改动。
- 未复制 `users.id=25` 的密码 hash。

## 验证

- `users.id=16` 当前：
  - `email=1915474749@qq.com`
  - `username=1915474749@qq.com`
  - `active API Key=1`
- `auth_identities.id=27` 当前：
  - `user_id=16`
  - `provider_type=email`
  - `provider_key=email`
  - `provider_subject=1915474749@qq.com`
  - `verified=true`
- `users.id=25` 当前 `deleted=true`。
- `user_affiliates` 当前归属 `user_id=16`。
- `user_platform_quotas` 当前有 4 条 active 记录归属 `user_id=16`。
- 已清理 `user_id=16` active API Key 对应的 Redis API Key auth cache。
