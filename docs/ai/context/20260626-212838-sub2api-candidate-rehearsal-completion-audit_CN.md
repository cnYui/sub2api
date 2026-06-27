# Sub2API 候选预演完成审计

## 目标

本地完整跑通“新镜像 + 生产 DB 克隆 + 独立 Redis + 本地端口”，用于后续上公网前的蓝绿候选验证。

## 审计时间

- 2026-06-26 21:28 JST

## 当前候选运行态

- 候选镜像：`sub2api-candidate:20260626-211623-30e66c82580f`
- 候选镜像 ID：`sha256:8456aa3328e38c835b3aa127ed0016dcebbb77f11552f83fdb60c6add3080a6b`
- main HEAD：`30e66c82580f`
- 候选端口：`http://127.0.0.1:18084`
- 候选容器：
  - `sub2api-candidate`：healthy，`127.0.0.1:18084->8080`
  - `sub2api-candidate-postgres`：healthy
  - `sub2api-candidate-redis`：healthy
- 候选 compose project：`sub2api-candidate-rehearsal`
- 公网 compose project：`deploy`

## 隔离证据

- 公网容器仍为：
  - `sub2api`：healthy，`127.0.0.1:18080->8080`
  - `sub2api-postgres`：healthy
  - `sub2api-redis`：healthy
- 公网 health：`http://127.0.0.1:18080/health` -> 200，`{"status":"ok"}`
- 候选 health：`http://127.0.0.1:18084/health` -> 200，`{"status":"ok"}`
- 候选容器挂载目录位于 worktree 下的 `deploy/candidate/`，不共用公网 `deploy/postgres_data` 或 `deploy/redis_data`。
- 候选 Redis 独立：
  - candidate Redis：`PONG`，`DBSIZE=77`
  - public Redis：`PONG`，`DBSIZE=386`

## 脚本安全验证

- `deploy/test-candidate-rehearsal-scripts.sh`：PASS。
- `deploy/rehearse-sub2api-candidate.sh --dry-run --reset-db --candidate-port 18084` 已验证：
  - 显式使用 `-p sub2api-candidate-rehearsal`。
  - 会检查公网容器 compose project label。
  - `--reset-db` 只删除 `sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis`。
  - dry-run 输出不包含 `down --remove-orphans`。
  - dry-run 输出不包含 `up -d --no-deps --force-recreate sub2api`。
  - dry-run 输出不访问 `https://api.aaccx.pw/v1`。

## 生产 DB 克隆与迁移验证

公网 DB 当前：

- `public_users=32`
- `public_api_keys=22`
- `public_migrations=188`
- `public_payment_provider_instances=0`
- `public_status=active:32`

候选 DB 当前：

- `candidate_users=32`
- `candidate_api_keys=22`
- `candidate_migrations=191`
- `candidate_payment_provider_instances=0`
- `candidate_status=active:32`
- `candidate_password_hash_nonempty=32`
- `candidate_auth_identities=31`
- `154_seed_codex_99_subscription_plan.sql=1755c1d2ea1840ea907e3e91bd7be38be2de3820b43ba383c7f89ca5e43c8707`
- `155_seed_codex_subscription_plans_baseline.sql=64c22df919959e4afef129f8aacd2785254215cee2888f2ec162f65c36d1921f`

判断：候选 DB 是从生产 DB 克隆后在候选容器内完成新迁移；生产 DB 没有被写入候选迁移。

## HTTP 与关键 API 验证

候选本地端口验证：

- `GET /health` -> 200
- `GET /` -> 200
- `GET /dashboard` -> 200
- `GET /purchase` -> 200
- `GET /usage-guide` -> 200
- `GET /api/v1/settings/public` -> 200
- 无授权 `GET /v1/responses` -> 401

管理员登录 smoke：

- 使用公网运行配置中的管理员 env 值，仅用于本地候选请求，未输出凭据或 token。
- `POST /api/v1/auth/login` -> 200
- 登录响应包含 access token。
- `GET /api/v1/auth/me` -> 200
- `GET /api/v1/subscriptions/active` -> 200

补充：用户指定的 `xiaobianfuai@gmail.com` 在当前候选 DB 和公网 DB 中存在、active、有密码哈希，但 role 为 `user`；使用用户本轮提供的密码对候选端口登录返回 401。该结果说明这组凭据不适合本次管理员 smoke，不影响候选环境已跑通的结论。

## 日志验证

候选容器最近日志未发现：

- `checksum mismatch`
- `migration failed`
- `failed to initialize application`
- `invalid totp`
- `panic`
- `fatal`

## 当前未做事项

- 未 promote 候选镜像。
- 未重建公网 `sub2api`。
- 未重建公网 Postgres 或 Redis。
- 未向公网 `/v1` 发起验证请求。
- 未输出或记录完整 API key、JWT、支付密钥、SMTP 密码、管理员密码。

## 结论

本地候选预演目标已经满足：新镜像、生产 DB 克隆、独立 Redis、独立本地端口和关键 API 均已通过验证。后续如要上公网，必须只替换公网 `sub2api` 应用容器，不能重建公网 Postgres/Redis。
