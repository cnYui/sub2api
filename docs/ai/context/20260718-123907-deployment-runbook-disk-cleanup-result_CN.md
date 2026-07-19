# 部署 Runbook 增加磁盘空间清理结果

## 完成内容

- 更新 `docs/SUB2API_CLIPROXYAPI_DEPLOYMENT_RUNBOOK_CN.md`：
  - 在“部署红线”增加空间不足时先清旧容器和无用镜像，禁止删除 DB/Redis volume 或业务备份。
  - 在“部署前确认”中新增“清理旧容器和无用镜像释放空间”章节，并放在备份和构建前。
  - 增加磁盘占用检查、Docker 占用检查、旧容器候选列表、受保护生产容器、删除旧容器、清理 dangling image、清理 build cache、可选删除未引用旧镜像的步骤。
  - 明确禁止 `docker volume prune`、`docker system prune --volumes`、`docker compose down -v` 和删除 `deploy/candidate/postgres_data`、`deploy/candidate/redis_data`、`deploy/backups`。
  - 更新新人执行检查清单，要求重新部署前完成旧容器/无用镜像清理。
- 更新 `AGENTS.md`：
  - 长期记忆里补充“重新部署前空间不足要先按 Runbook 清理已停止旧容器和无用镜像，禁止删 DB/Redis volume”。
- 新增计划上下文：
  - `docs/ai/context/20260718-123656-deployment-runbook-disk-cleanup-plan_CN.md`

## 验证结果

- `git diff --check`：通过，无输出。
- 敏感信息粗扫：
  `rg -n "sk-|HMAC|secret|password|token|api key|API Key|Bearer [A-Za-z0-9_\\-]{20,}" ...`
  只命中通用说明和既有安全提醒，未发现完整真实密钥。

## 运行态影响

- 未执行 Docker 清理。
- 未删除容器、镜像、volume 或备份。
- 未构建镜像。
- 未部署。
- 未修改 PostgreSQL、Redis、Nginx、Cloudflare Tunnel 或 CLIProxyAPI 配置。
