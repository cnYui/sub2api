# 最新 DB Volume 候选预演纠偏结果

## 结论

之前本地候选预演用错了 DB 源。

正确的最新生产 DB 数据在 Docker named volume `deploy_postgres_data`；当前公网容器误挂载的 bind 目录 `deploy/postgres_data` 是旧库。已重新用 `deploy_postgres_data` 克隆数据，并用本地最新 main 修复后的候选镜像跑通本地候选环境。

## 根因

存在两套不同的生产数据存储：

1. 最新 named volume：`deploy_postgres_data`
   - `users=47`
   - `api_keys=40`
   - `schema_migrations=191`
   - `xiaobianfuai@gmail.com` 为 `users.id=13`，`role=admin`，`deleted=false`
   - `users.id=26` 同邮箱普通测试账号已软删除

2. 旧 bind 目录：`/Users/wujianxiang/CodeSpace/sub2api/deploy/postgres_data`
   - `users=32`
   - `api_keys=22`
   - `schema_migrations=188`
   - `xiaobianfuai@gmail.com` 为 `users.id=26`，`role=user`，`deleted=false`

此前候选预演从当前错误运行的 bind DB 克隆，因此拿到的是旧库，导致误判 `xiaobianfuai@gmail.com` 不是管理员。

## 进一步暴露的问题

用最新 DB 启动当前 main 镜像时，应用失败在 migration 155 checksum：

- DB checksum：`0e2d20c620783bf91cbf5ffb524edb46730e998a10799f66dd03e988a32b0b8f`
- 当前文件 checksum：`64c22df919959e4afef129f8aacd2785254215cee2888f2ec162f65c36d1921f`

这说明最新 DB 曾应用过旧版本 `155_seed_codex_subscription_plans_baseline.sql`，而当前 main 中该迁移文件内容已变化。已按项目现有模式补充 checksum 兼容白名单，不直接改生产 DB checksum。

## 本次代码修复

修改文件：

- `backend/internal/repository/migrations_runner.go`
- `backend/internal/repository/migrations_runner_checksum_test.go`

新增行为：

- `155_seed_codex_subscription_plans_baseline.sql` 允许 DB 中已应用的历史 checksum `0e2d20...` 与当前文件 checksum `64c22...` 兼容。
- 兼容规则仍只针对迁移名和已知 checksum 集合，不放宽全局迁移校验。

测试：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/backend
go test ./internal/repository -run TestIsMigrationChecksumCompatible/155 -count=1
go test ./internal/repository -run TestIsMigrationChecksumCompatible -count=1
```

结果均为 `ok`。

## 最新候选运行态

- 候选镜像：`sub2api-candidate:20260626-214312-compat155-30e66c82580f`
- 候选端口：`http://127.0.0.1:18084`
- 候选容器：`sub2api-candidate`
- 候选 Postgres：`sub2api-candidate-postgres`
- 候选 Redis：`sub2api-candidate-redis`
- 候选 app 状态：running + healthy
- 公网 health 仍为 200，未重建公网容器。

## 候选 DB 验证

- `candidate_users=47`
- `candidate_api_keys=40`
- `candidate_migrations=191`
- `candidate_155=0e2d20c620783bf91cbf5ffb524edb46730e998a10799f66dd03e988a32b0b8f`
- `candidate_xiaobian=13:xiaobianfuai@gmail.com:admin:status=active:deleted=false`
- `candidate_xiaobian=26:xiaobianfuai@gmail.com:user:status=active:deleted=true`

## 候选 HTTP/API 验证

- `GET /health` -> 200
- `GET /` -> 200
- `GET /dashboard` -> 200
- `GET /purchase` -> 200
- `GET /usage-guide` -> 200
- `GET /api/v1/settings/public` -> 200
- 未授权 `GET /v1/responses` -> 401

使用 `xiaobianfuai@gmail.com` 管理员账号验证候选：

- `POST /api/v1/auth/login` -> 200
- 登录响应包含 access token
- `GET /api/v1/auth/me` -> 200
- `GET /api/v1/subscriptions/active` -> 200

未输出密码、token、API key 或支付/SMTP 密钥。

## 当前边界

- 未 promote 候选镜像。
- 未重建公网 `sub2api`。
- 未重建公网 Postgres/Redis。
- 未写 `deploy_postgres_data` 原始 named volume。
- 未写当前公网 bind DB。

## 后续修复公网建议

公网当前仍连接旧 bind DB，所以用户侧仍会看到旧库状态。要修复公网，应在维护窗口内执行二选一：

1. 将公网 compose 切回 named volume `deploy_postgres_data`，并只重建公网 Postgres/Sub2API；这是恢复原最新数据源。
2. 或先从 `deploy_postgres_data` 导出 dump，再恢复到公网目标 bind 目录，然后继续使用 local compose。

两种方式都必须先备份当前 bind DB 和 named volume；切换时会影响公网 API，不能在未确认窗口时执行。
