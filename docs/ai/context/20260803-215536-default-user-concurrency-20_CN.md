# 新用户默认并发设为 20

## 目标

后续注册的每个用户账户总并发为 20。该限制存储在 `users.concurrency`，对同一用户的所有 API Key 合并生效，不按 API Key 分配。

## 实施

- 18082 运行实例的 `default.user_concurrency` 设为 20。
- `default_concurrency` 及邮箱、LinuxDO、OIDC、微信、GitHub、Google、钉钉的注册来源并发默认值均设为 20。
- 新迁移将 `users.concurrency` 的数据库默认值改为 20；升级时仅将遗留值 5 改为 20，保留其他人工配置。
- 配置样例、初始化默认值和 Ent 用户架构同步设为 20。

## 已有用户

澄清前按原请求已将 18082 中 142 位活动用户从 200 统一为 20，并对 153 个有效 API Key 完成认证缓存失效。未修改已删除用户、余额、套餐或 API Key。
