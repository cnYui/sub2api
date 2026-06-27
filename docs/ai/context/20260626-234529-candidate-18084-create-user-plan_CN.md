# 18084 候选环境新建登录用户计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `127.0.0.1:18084` 对应的候选环境数据库中创建 `1038686518@qq.com` 普通用户，使其可通过登录页面使用用户指定的默认密码登录。

**Architecture:** `18084` 映射到容器 `sub2api-candidate`，该容器通过 `DATABASE_HOST=sub2api-candidate-postgres` 连接候选 PostgreSQL。用户创建需要同时维护 `users` 和 `auth_identities`，与后端 `userRepository.Create` 的行为保持一致，避免只插入 `users` 导致 email 登录身份缺失。

**Tech Stack:** Docker Desktop CLI、PostgreSQL `psql`、Go bcrypt 兼容哈希、Sub2API 后端用户表和 email auth identity 表。

---

## 边界

- 只修改 `sub2api-candidate-postgres` 候选数据库。
- 不修改生产库 `sub2api-postgres`。
- 不创建 API Key、不绑定套餐、不调整余额、不修改支付配置。
- 不在文档中记录候选环境内部运行密钥。

## 已确认事实

- `sub2api-candidate` 端口映射为 `127.0.0.1:18084 -> 8080/tcp`。
- `sub2api-candidate` 连接 `sub2api-candidate-postgres`。
- 候选库当前未删除用户数为 44。
- `1038686518@qq.com` 在候选库 `users` 中不存在。
- `1038686518@qq.com` 在候选库 `auth_identities` 中不存在。
- `users.email` 有 `WHERE deleted_at IS NULL` 的唯一索引。
- `auth_identities` 对 `(provider_type, provider_key, provider_subject)` 有唯一索引。

## 执行步骤

- [ ] 生成与后端一致的 bcrypt 密码哈希。

```bash
go run /tmp/sub2api-bcrypt-hash.go
```

期望：输出一个以 `$2a$` 或 `$2b$` 开头的 bcrypt 哈希。

- [ ] 使用事务插入候选库用户和 email 身份。

```sql
BEGIN;

WITH new_user AS (
  INSERT INTO users (
    email,
    username,
    password_hash,
    role,
    balance,
    concurrency,
    status,
    signup_source,
    notes,
    created_at,
    updated_at
  )
  VALUES (
    '1038686518@qq.com',
    '1038686518@qq.com',
    '<bcrypt_hash>',
    'user',
    0,
    5,
    'active',
    'email',
    '',
    NOW(),
    NOW()
  )
  RETURNING id
)
INSERT INTO auth_identities (
  user_id,
  provider_type,
  provider_key,
  provider_subject,
  verified_at,
  metadata,
  created_at,
  updated_at
)
SELECT
  id,
  'email',
  'email',
  '1038686518@qq.com',
  NOW(),
  '{"source":"manual_candidate_18084_create_user"}'::jsonb,
  NOW(),
  NOW()
FROM new_user;

COMMIT;
```

期望：事务成功提交，新增一个 `users` 行和一个 `auth_identities` 行。

- [ ] 验证数据库记录。

```sql
SELECT id, email, username, role, status, signup_source, deleted_at
FROM users
WHERE email = '1038686518@qq.com';

SELECT ai.user_id, ai.provider_type, ai.provider_key, ai.provider_subject, ai.verified_at
FROM auth_identities ai
JOIN users u ON u.id = ai.user_id
WHERE u.email = '1038686518@qq.com';
```

期望：用户为 `role=user`、`status=active`、`signup_source=email`、`deleted_at` 为空；身份记录为 `provider_type=email`、`provider_key=email`。

- [ ] 通过 18084 登录接口验证。

```bash
curl -sS -X POST http://127.0.0.1:18084/api/auth/login \
  -H 'Content-Type: application/json' \
  --data '{"email":"1038686518@qq.com","password":"<default_password>"}'
```

期望：返回成功响应并包含登录 token 或用户信息；如果真实路径不同，按当前后端路由调整为候选环境实际登录接口。

- [ ] 写入结果上下文并更新 `AGENTS.md`。

结果文档需记录目标环境、创建结果和验证结果；`AGENTS.md` 只追加一条长期记忆，不记录运行密钥。
