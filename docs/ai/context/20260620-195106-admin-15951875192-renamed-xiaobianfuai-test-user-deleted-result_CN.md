# 管理员账号邮箱替换与测试账号删除结果

## 结果

- 已将管理员账号 `users.id=13` 从 `15951875192@phone.com` 更新为 `xiaobianfuai@gmail.com`。
- 管理员账号仍保持 `role=admin`、`status=active`，未删除。
- 管理员本机无限额 API Key `api_keys.id=32` 仍未删除，归属用户仍为 `users.id=13`，分组仍为 `codex-pool-local-unlimited`。
- 已软删除普通测试账号 `users.id=26`，该账号原邮箱为 `xiaobianfuai@gmail.com`。
- 已删除测试账号的邮箱身份，`auth_identities` 中 `xiaobianfuai@gmail.com` 现在只指向管理员 `users.id=13`。
- 已将测试账号 API Key `api_keys.id=33` 写入 `deleted_api_key_audits`，随后用 tombstone 覆盖 key 字段并软删除。
- 已删除测试账号 `user_refresh_tokens:26`，并清理测试 Key 的 Redis auth cache。

## 验证

数据库断言全部通过：

- 活跃 `xiaobianfuai@gmail.com` 用户数量为 1，且为 `users.id=13`。
- 活跃 `15951875192@phone.com` 用户数量为 0。
- 管理员邮箱身份存在且指向 `users.id=13`。
- 旧手机号邮箱身份数量为 0。
- 测试账号身份数量为 0。
- 管理员 API Key 仍 active 且未软删除。
- 测试 API Key 已软删除，并存在删除审计。

缓存验证：

- 测试 Key 的 Redis auth cache 已不存在。
- 管理员本机 Key 已执行 Redis L2 删除并广播 L1 失效；因该 Key 是本机运行态使用的活跃 Key，验证时 Redis 中可被服务重新写回。

## 注意

- 本次没有迁移测试账号的密码 hash 到管理员账号；`xiaobianfuai@gmail.com` 现在对应的是原管理员账号的密码和权限。
- 本次没有修改管理员余额、订阅、用量、密码 hash、角色、状态、API Key 或分组。
