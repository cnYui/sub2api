# Sub2API 运行态重部署结果

## 执行结论

已按 `docs/SUB2API_CLIPROXYAPI_DEPLOYMENT_RUNBOOK_CN.md` 完成本次 `sub2api` 运行态重部署。当前公网链路仍由 `sub2api-candidate` 承接，数据库和 Redis 未重建，Nginx 与 Cloudflare Tunnel 未改动。

## 已执行内容

### 1. 运行态检查

- `git status --short --branch`：当前 worktree 存在既有未提交改动和未跟踪上下文文档。
- `docker ps`：确认当前生产三容器存在。
- `http://127.0.0.1:18084/health`、`http://127.0.0.1:8080/health`、`https://api.aaccx.pw/health`：均返回 `{"status":"ok"}`。
- `nginx -T`：确认实际指向 `127.0.0.1:18084`。
- `df -h /`：清理前根分区可用约 `9.9Gi`。
- `docker system df -v`：存在已停止的 `before-promote` 容器与 dangling 镜像，可安全清理。

### 2. 计划上下文

- 新建运行态重部署计划上下文：`docs/ai/context/20260718-124212-sub2api-runtime-redeploy-plan_CN.md`

### 3. 安全清理与备份

- 删除已停止旧容器：`sub2api-candidate-before-promote-20260718-103605`
- 清理 dangling 镜像和旧 build cache
- `df -h /` 清理后可用空间提升到约 `16Gi`
- 创建并验证 Postgres 备份：`deploy/backups/20260718-124323-sub2api-candidate-postgres-before-deploy.dump`
- 创建并验证 Redis 备份：`deploy/backups/20260718-124323-sub2api-candidate-redis-before-deploy.rdb`
- `pg_restore --list` 成功
- `redis-check-rdb` 成功

### 4. 只读配置核对

- `accounts.id=1`：`cliproxy-local-openai`
- `base_url=https://host.docker.internal:8317/v1`
- `status=active`
- `schedulable=true`
- `pool_mode=true`
- `temp_unschedulable_until` 为空
- CLIProxyAPI 8317 证书检查正常，且从 Sub2API 容器内访问返回 `401 Unauthorized`，说明 TLS 与连通性正常

### 5. 构建与发布

- 构建候选镜像：`sub2api-candidate:20260718-124408-e16a67a58cd3`
- `deploy/promote-sub2api-candidate.sh --dry-run --yes` 通过，确认只替换应用容器
- `deploy/promote-sub2api-candidate.sh --yes` 成功
- 新 `sub2api-candidate` 启动并通过 `/health`

### 6. 发布后验收

- `http://127.0.0.1:18084/health`：通过
- `http://127.0.0.1:8080/health`：通过
- `https://api.aaccx.pw/health`：通过
- `docker logs --since 10m sub2api-candidate`：未见 panic、migration failed、x509、invalid url scheme、account_select_failed、auth_unavailable 等新异常
- `usage_facts` 最新记录仍为 `settled`

### 7. 代码校验

- `go test ./...`：通过
- `pnpm --dir frontend run lint:check`：通过
- `pnpm --dir frontend run typecheck`：通过
- `golangci-lint run ./...`：本机未安装 `golangci-lint`，因此该命令未能执行

## 当前运行态

- 当前应用容器：`sub2api-candidate`
- 当前数据库容器：`sub2api-candidate-postgres`
- 当前 Redis 容器：`sub2api-candidate-redis`
- 当前公网宿主端口：`127.0.0.1:18084 -> 8080`

## 回滚边界

- 若新版本需要回滚，只回滚应用容器到 `sub2api-candidate-before-promote-20260718-125251`
- 不回滚数据库、Redis、支付状态或其他外部状态

## 备注

- 本次没有导出或记录任何完整密钥
- 本次没有修改 Nginx、Cloudflare Tunnel、Postgres volume 或 Redis volume
