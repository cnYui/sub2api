# 批量手机号迁移用户真实邮箱更新结果

## 结果

已完成 6 个手机号迁移用户的真实邮箱更新：

- `18367290091@phone.com` -> `changjunwang123@gmail.com`
- `13052071067@phone.com` -> `milesyang987@gmail.com`
- `19520434236@phone.com` -> `xunskyler@gmail.com`
- `18405650929@phone.com` -> `xwh1124wcw@163.com`
- `13584052801@phone.com` -> `897858381@qq.com`
- `15995436627@phone.com` -> `15995436627@163.com`

## 更新字段

- `users.email`
- `users.username`
- `auth_identities.provider_subject`
- 对应用户 active API Key 的 Redis auth cache

## 未改动内容

- 用户 id
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

- 6 个真实邮箱都能查到对应 active 用户。
- 6 个用户的 `users.email` 与 `users.username` 均已同步为真实邮箱。
- 6 个用户的 `auth_identities` email 身份均已同步为真实邮箱，且 `verified=true`。
- 6 个旧 `<手机号>@phone.com` active 用户记录数为 `0`。
- 6 个用户各自仍有 1 个 active API Key，均继续绑定 `codex-pool-19-usd`。

## 用户 id 对照

- `milesyang987@gmail.com` -> `users.id=3`
- `897858381@qq.com` -> `users.id=5`
- `15995436627@163.com` -> `users.id=14`
- `changjunwang123@gmail.com` -> `users.id=17`
- `xwh1124wcw@163.com` -> `users.id=18`
- `xunskyler@gmail.com` -> `users.id=19`
