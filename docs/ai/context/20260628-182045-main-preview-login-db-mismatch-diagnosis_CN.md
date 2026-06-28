# 2026-06-28 main-preview 登录账号不可用诊断

## 现象

用户在 `http://127.0.0.1:18080` 的 main-preview 蓝绿测试环境中使用原有账号密码登录失败。

## 只读排查

18080 应用容器实际环境：

- `DATABASE_HOST=postgres`
- `DATABASE_DBNAME=sub2api`
- `DATABASE_USER=sub2api`
- `REDIS_HOST=redis`
- `ADMIN_EMAIL=admin@sub2api.local`
- `AUTO_SETUP=true`

这些 `postgres`/`redis` 是 `sub2api-main-preview-net` 内的别名，指向：

- `sub2api-main-preview-postgres`
- `sub2api-main-preview-redis`

不是公网 18084 的：

- `sub2api-candidate-postgres`
- `sub2api-candidate-redis`

## 数据对比

18080 main-preview 库：

```text
preview_users|1
preview_active_users|1
preview_password_hash_nonempty|1
preview_auth_identities|0
preview_api_keys|0
preview_migrations|192
```

18084 candidate 公网候选库：

```text
candidate_users|49
candidate_active_users|46
candidate_password_hash_nonempty|46
candidate_auth_identities|47
candidate_api_keys|41
candidate_migrations|191
```

抽查常用账号：

- 18080 只有 `admin@sub2api.local`。
- 18084 包含 `1038686518@qq.com`、`2799523972@qq.com`、`cnfoxian@gmail.com`、`xiaobianfuai@gmail.com` 等公网候选库用户。

18080 日志确认 `POST /api/v1/auth/login` 返回 401。

## 根因

18080 蓝绿测试环境虽然启动了 PostgreSQL 和 Redis，但启动的是隔离的 main-preview 数据层，且该数据层不是当前 18084 公网候选库的克隆。

因此公网账号密码无法在 18080 使用，不是 Redis/PostgreSQL 没启动，也不是前后端没有跑起来，而是账号数据不在这套库里。

## 可选修复方向

1. 克隆 18084 candidate 数据库到 18080 main-preview 数据库，再让本地 main 代码自动跑新迁移。优点是能用公网候选账号做最接近真实的蓝绿验证；风险是会覆盖当前 18080 预览库，必须只写 main-preview 数据层，且对 18084 只能做只读 dump。
2. 只在 18080 main-preview 中创建或重置一个测试管理员/测试用户。优点是最小影响；缺点是不能验证真实公网用户、Key、订单、支付配置。
3. 不建议让 18080 直接连接 18084 数据库。当前本地 main 会应用新迁移，直接连公网候选库会修改 18084 数据层，违反本轮隔离目标。
