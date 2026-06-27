# 手机号用户 15706598243 真实邮箱更新结果

## 结果

- 已将手机号迁移用户 `users.id=11` 从 `15706598243@phone.com` 更新为 `15706598243@163.com`。
- 已同步更新：
  - `users.email`
  - `users.username`
  - `auth_identities.provider_subject`
- 用户仍保持 `role=user`、`status=active`，未删除。
- 原 API Key 仍保持 active，仍绑定 `codex-pool-19-usd`，掩码为 `sk-yui-z...JQfmTr`。
- 未修改该用户的 API Key、订阅、余额、用量、密码 hash、角色或状态。

## 验证

数据库断言全部通过：

- 活跃目标邮箱 `15706598243@163.com` 用户数量为 1，且为 `users.id=11`。
- 活跃旧手机号邮箱 `15706598243@phone.com` 用户数量为 0。
- 邮箱 identity `15706598243@163.com` 指向 `users.id=11`。
- 旧手机号邮箱 identity 数量为 0。
- 该用户 active API Key 数量为 1。

缓存处理：

- 已对该用户 active API Key 执行 Redis auth cache 删除并广播 L1 失效。
- Redis 删除返回 0，表示当时没有该 Key 的 L2 auth cache 可删；失效广播已发送。
