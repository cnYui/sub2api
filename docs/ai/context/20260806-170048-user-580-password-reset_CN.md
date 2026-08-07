# 用户 580 密码重置记录

## 背景

- 管理员要求重置用户 `935649226@qq.com` 的密码。
- 操作目标为生产实例 `sub2api-official-18082`，用户 ID 为 `580`，操作前状态为 `active`。

## 执行

- 使用项目依赖 `golang.org/x/crypto/bcrypt` 的默认成本生成新密码哈希。
- 在 PostgreSQL 事务中仅更新该用户的 `password_hash` 与 `updated_at`。
- 删除 Redis 中该用户的 2 个 refresh token 及 `user_refresh_tokens:580` 会话索引，确保既有登录会话无法续期。
- 密码哈希变化会改变服务端令牌版本指纹，因此既有访问令牌同时失效。

## 核验

- 数据库更新行数为 1，用户 ID、邮箱和 `active` 状态保持不变。
- 新密码的 bcrypt 比对通过。
- Redis 中用户会话索引不存在，未发现仍指向该用户的 refresh token。

## 安全说明

- 本记录不保存密码明文、完整密码哈希或会话令牌。
