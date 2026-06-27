# Sub2API 候选预演成功结果

## 结论

本地候选环境已按“新镜像 + 生产 DB 克隆 + 独立 Redis + 本地端口”完整跑通。

- 候选镜像：`sub2api-candidate:20260626-211623-30e66c82580f`
- 候选镜像 ID：`sha256:8456aa3328e38c835b3aa127ed0016dcebbb77f11552f83fdb60c6add3080a6b`
- 候选端口：`http://127.0.0.1:18084`
- 候选 Postgres：`sub2api-candidate-postgres`
- 候选 Redis：`sub2api-candidate-redis`
- 公网容器未 promote、未重建，公网 health 仍为 `{"status":"ok"}`。

## 本次脚本修复

候选预演前先补了安全与启动修复：

- 移除候选脚本中的 `down --remove-orphans`。
- `--reset-db` 只删除明确的 `sub2api-candidate*` 容器。
- 增加公网容器 compose project label 检查，若公网容器 project 等于候选 project 立即退出。
- 增加 `pg_isready` 等候选 Postgres ready 后再 `pg_restore`。
- 将候选 `TOTP_ENCRYPTION_KEY` 示例值改为合法 64 位 hex，修复 `invalid totp encryption key` 启动失败。
- `deploy/test-candidate-rehearsal-scripts.sh` 覆盖以上 dry-run 安全边界。

## 执行结果

命令：

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
  deploy/rehearse-sub2api-candidate.sh --reset-db --candidate-port 18084
```

结果：

- Docker build 成功。
- 从公网 `sub2api-postgres` 生成生产 DB dump 成功。
- 候选 Postgres 从 dump 恢复成功。
- 候选 sanitize SQL 执行成功。
- 候选 app 启动成功。
- 候选 health 通过。

## 验证结果

候选容器：

- `sub2api-candidate`：healthy，`127.0.0.1:18084->8080`
- `sub2api-candidate-postgres`：healthy
- `sub2api-candidate-redis`：healthy

公网容器：

- `sub2api`：healthy，仍为 `127.0.0.1:18080->8080`
- `sub2api-postgres`：healthy
- `sub2api-redis`：healthy

候选 HTTP 验证：

- `GET /health` -> 200
- `GET /` -> 200
- `GET /dashboard` -> 200
- `GET /purchase` -> 200
- `GET /usage-guide` -> 200
- `GET /api/v1/settings/public` -> 200
- 管理员 `POST /api/v1/auth/login` -> 200
- `GET /api/v1/auth/me` -> 200
- `GET /api/v1/subscriptions/active` -> 200
- 无授权 `POST /v1/responses` -> 401（只验证本地候选路由，不触发上游）

候选 DB 验证：

- `schema_migrations` 总数：191
- `154_seed_codex_99_subscription_plan.sql` checksum：`1755c1d2ea1840ea907e3e91bd7be38be2de3820b43ba383c7f89ca5e43c8707`
- `155_seed_codex_subscription_plans_baseline.sql` checksum：`64c22df919959e4afef129f8aacd2785254215cee2888f2ec162f65c36d1921f`
- `users_total=32`
- `api_keys_total=22`
- `payment_provider_instances=0`

日志验证：

- 未发现 `checksum mismatch`。
- 未发现 `migration failed`。
- 未发现 `failed to initialize application`。
- 未发现 `invalid totp encryption key`。
- 未发现 panic/fatal/error 级启动失败。

## 当前边界

- 本次没有执行 `deploy/promote-sub2api-candidate.sh`。
- 本次没有重建公网 `sub2api`。
- 本次没有写公网 Postgres 或公网 Redis。
- 本次没有使用真实 API key 对候选 `/v1` 发起上游请求。

## 是否可直接上公网

候选环境证明：当前 main 镜像在生产 DB 克隆上可以完成 migration、启动和关键本地 API 验证。

真正上公网前仍建议补一个短发布门禁：

1. 提交候选脚本安全修复。
2. 再次执行 dry-run，确认只触碰 candidate 容器。
3. 若要发布，只重建公网 `sub2api` 应用容器，不重建公网 Postgres/Redis。
4. 切换后先验 `127.0.0.1:18080/health` 与 `https://api.aaccx.pw/health`，再做管理员登录和少量受控 API key smoke。
