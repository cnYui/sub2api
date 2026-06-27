# 最新 DB Volume 候选预演纠偏计划

## 背景

当前发现公网容器 `sub2api-postgres` 挂载的是 bind 目录：

- `/Users/wujianxiang/CodeSpace/sub2api/deploy/postgres_data`

该目录内容较旧：

- `users=32`
- `api_keys=22`
- `schema_migrations=188`
- `xiaobianfuai@gmail.com` 是 `users.id=26` 普通用户，未删除。

而 Docker named volume `deploy_postgres_data` 中保留了更新的生产数据：

- `users=47`
- `api_keys=40`
- `schema_migrations=191`
- `xiaobianfuai@gmail.com` 是 `users.id=13` 管理员。
- `users.id=26` 同邮箱普通测试账号已软删除。

## 纠偏目标

重新本地跑通候选环境，但数据源必须改为最新 named volume `deploy_postgres_data`，不是当前错误挂载的公网 bind 目录。

## 执行边界

- 不停止公网 `sub2api`。
- 不停止公网 `sub2api-postgres`。
- 不停止公网 `sub2api-redis`。
- 不写当前公网 bind DB。
- 不写 `deploy_postgres_data` 原始 named volume；只复制/导出。
- 不输出密码、API key、JWT、支付密钥。

## 执行步骤

1. 从 `deploy_postgres_data` 克隆到临时 volume。
2. 启动临时 Postgres 容器，只用于 `pg_dump`。
3. 将 dump 恢复到 `sub2api-candidate-postgres`。
4. 使用当前 main 构建好的候选镜像 `sub2api-candidate:20260626-211623-30e66c82580f` 继续运行本地候选。
5. 重启候选 app，不触碰公网容器。
6. 验证：
   - 候选 DB 中 `xiaobianfuai@gmail.com` 是 admin。
   - 候选 migration 为 191。
   - 候选 health 为 200。
   - 候选登录、`/auth/me`、`/subscriptions/active` 成功。
   - 关键页面和只读 API 成功。

## 后续判断

如果候选用最新 volume 数据跑通，说明之前失败的根因是候选数据源选错；之后上公网的正确修复应是把公网 Postgres 重新挂回 `deploy_postgres_data` 或先从该 volume 的 dump 恢复到公网目标数据目录，而不是继续使用当前旧 bind 目录。
