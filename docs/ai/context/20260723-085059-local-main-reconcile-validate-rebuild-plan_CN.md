# 本地 main 整理、全量验证与 Docker 重建计划

## 目标

- 整理本地 `main` 的已合并分支、未提交上下文和可公开部署配置。
- 完整验证合并后的代码。
- 备份并验证当前本地 PostgreSQL/Redis 数据，重建 `sub2api-dev` 应用镜像并启动新代码。

## Git 决策

- `codex/rolling-weekly-traffic-credit-fallback` 的补丁与 `main` 提交 `4cefdda10` 的 patch-id 相同，已等价落入 `main`，不重复合并。
- 跟踪根工作区中已修改的 `AGENTS.md`、项目上下文文档，以及不含凭据的 Nginx/Cloudflared 示例和启动配置。
- 不跟踪 `backups/`、真实 Cloudflared 配置或任何扫描到凭据的文件；为其补充忽略规则，避免后续误提交。

## 验证

- 后端：`go test ./...` 与带 integration 标签的仓储/迁移验证。
- 前端：typecheck、lint、全量 Vitest、生产构建。
- Git：暂存内容敏感信息扫描、`git diff --check`、提交后工作树核验。

## 备份与恢复验证

- 在重启前对 `sub2api-postgres-dev` 生成 custom-format `pg_dump`，并用 `pg_restore --list` 验证可读。
- 同步导出 `sub2api-redis-dev` 的 RDB 快照。
- 将 PostgreSQL 备份实际恢复到临时隔离 PostgreSQL 容器，并与在线库核对核心表行数；临时容器随后删除。
- `docker-compose.dev.yml` 只重建 `sub2api` 服务，保留 `deploy/postgres_data` 和 `deploy/redis_data`。数据库目录不被替换，因此新代码启动后继续使用已验证的原数据。
- 不把旧备份直接覆盖正在运行的在线数据库；那会丢弃切换期间新增的订单、用量与支付回调。若新代码健康失败，使用保留的旧应用镜像回滚，数据库无需回滚。

## 启动与验收

- 在重建前记录旧镜像 ID 并创建本地回滚标签。
- 使用 Compose 仅更新 `sub2api`，不执行 `down`、不重建 PostgreSQL/Redis。
- 验证 `18080`、Nginx `8080` 和公网 health；使用不含密钥的健康检查、容器健康状态、迁移日志和数据库表计数确认。
- 如健康失败，停止继续切换并用旧镜像恢复应用容器。
