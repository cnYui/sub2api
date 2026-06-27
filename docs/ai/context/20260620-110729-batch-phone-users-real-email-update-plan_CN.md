# 批量手机号迁移用户真实邮箱更新计划

## 目标

将以下手机号迁移用户从假邮箱 `<手机号>@phone.com` 更新为真实邮箱：

- `18367290091@phone.com` -> `changjunwang123@gmail.com`
- `13052071067@phone.com` -> `milesyang987@gmail.com`
- `19520434236@phone.com` -> `xunskyler@gmail.com`
- `18405650929@phone.com` -> `xwh1124wcw@163.com`
- `13584052801@phone.com` -> `897858381@qq.com`
- `15995436627@phone.com` -> `15995436627@163.com`

## 当前检查

- 6 个映射都能定位到唯一 active 用户。
- 每个用户当前都有 1 个 active API Key。
- `13052071067` 已在上一轮更新为 `milesyang987@gmail.com`，本次事务保持幂等。
- 其余 5 个用户仍使用 `<手机号>@phone.com`。
- 每个用户都有一条 `provider_type=email`、`provider_key=email` 的 `auth_identities` 记录。

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

## 风险控制

- 如果某个映射无法定位唯一 active 用户，事务失败。
- 如果目标邮箱被其他 active 用户占用，事务失败。
- 如果目标邮箱被其他 email identity 占用，事务失败。
- 更新后清理这些用户 active API Key 对应的 Redis auth cache，避免认证快照短时间保留旧邮箱。
