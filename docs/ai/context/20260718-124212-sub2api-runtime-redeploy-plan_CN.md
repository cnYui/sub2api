# Sub2API 运行态重部署计划

## 背景

用户要求参考以下文档重新部署 `sub2api` 容器：

- `docs/ai/context/20260718-123656-deployment-runbook-disk-cleanup-plan_CN.md`
- `docs/SUB2API_CLIPROXYAPI_DEPLOYMENT_RUNBOOK_CN.md`
- `docs/ai/context/20260718-123907-deployment-runbook-disk-cleanup-result_CN.md`

## 已确认运行态

- 当前公网链路目标：`public_candidate_18084`。
- 当前应用容器：`sub2api-candidate`，端口 `127.0.0.1:18084->8080/tcp`。
- 当前数据库容器：`sub2api-candidate-postgres`。
- 当前 Redis 容器：`sub2api-candidate-redis`。
- `http://127.0.0.1:18084/health`、`http://127.0.0.1:8080/health`、`https://api.aaccx.pw/health` 均返回正常。
- Nginx 配置命中 `127.0.0.1:18084`。
- 根分区可用约 `9.9Gi`，Docker 存在可清理的已停止 `before-promote` 容器和 dangling 镜像。
- 当前 worktree 有既有未提交改动和未跟踪上下文文档，本次只执行运行态部署，不做提交。

## 执行计划

1. 按 Runbook 清理已停止旧容器和 dangling 镜像，保护当前生产三容器、volume 和业务备份。
2. 清理后再次确认生产三容器和 18084 health 正常。
3. 创建 Postgres 备份并用 `pg_restore --list` 验证可读。
4. 创建 Redis RDB 备份并用 `redis-check-rdb` 验证可读。
5. 只读核对 Sub2API 上游账号 `id=1`，确认 `base_url`、`pool_mode`、调度状态。
6. 核对 CLIProxyAPI 8317 TLS，以及 Sub2API 容器内到 `https://host.docker.internal:8317/v1/models` 的连通性。
7. 构建新的 `sub2api-candidate:<timestamp>-<sha>` 镜像。
8. 执行 `deploy/promote-sub2api-candidate.sh --dry-run --yes`，确认只替换应用容器。
9. 执行正式 promote，只替换 `sub2api-candidate`，不重建 Postgres、Redis、Nginx 或 Cloudflare Tunnel。
10. 发布后执行 health、容器状态、日志、DB/Redis 只读检查。
11. 如用户提供有效 Sub2API 用户 Key，再执行公网 `/v1/chat/completions` smoke test 和 `usage_facts` 验证；如果没有 Key，只做不含密钥的可用性验证。
12. 写入 result 上下文记录备份路径、镜像、清理项、部署和验证结果，不记录任何密钥。

## 回滚边界

- 应用发布失败时，只回滚应用容器到 `sub2api-candidate-before-promote-<timestamp>`。
- 不自动恢复 Postgres、Redis 或外部支付状态。
- 如果新版本运行过迁移且需要数据层回滚，必须另行评估和授权。

## 禁止事项

- 不执行 `docker volume prune`。
- 不执行 `docker system prune --volumes`。
- 不执行 `docker compose down -v`。
- 不删除 `deploy/candidate/postgres_data`、`deploy/candidate/redis_data`、`deploy/backups`。
- 不删除当前生产三容器。
- 不在文档、命令或聊天中记录完整密钥。
