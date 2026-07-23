# 本地 main 合并、验证与镜像重建结果

时间：2026-07-23

## 完成事项

- 已将 `codex/28day-weekly-renewal-alignment` 合并到本地 `main`，合并提交为 `aa0e01d71`。
- 已确认 `codex/rolling-weekly-traffic-credit-fallback` 与 `main` 的对应改动 patch-id 等价，无需重复合并。
- 已归档并跟踪本地协作上下文与非敏感部署配置；数据库备份和真实 Tunnel 配置保持忽略，不纳入 Git。
- 已重新构建本地应用镜像：`sub2api-localdev-sub2api:latest`，镜像 ID 为 `sha256:12d30b9e1454a0efe57622f18116549cce4823f61ad3aa97ac335e6bc1a7cc59`。
- 本地应用容器 `sub2api-dev` 已以新镜像重建并恢复发布到 `127.0.0.1:18080`；本地 Nginx 继续监听 `127.0.0.1:8080` 并转发到该端口。

## 数据库保护与恢复验证

- 重建前已创建 PostgreSQL custom dump 和 Redis RDB 备份，位于被 Git 忽略的 `backups/` 目录。
- PostgreSQL dump SHA-256：`51EBA35884BF8F0DFE8D66ACC87CB5F0E45587DF33F44E141D19693CC0E39FC1`。
- Redis RDB SHA-256：`2F1CD834534FD96F60749FCD4ACAB199930812D97CFD60077399CAD06449ED42`。
- PostgreSQL/Redis 使用既有 bind mount 数据目录，镜像重建未删除或替换数据卷；新代码启动后，关键表计数与备份时一致：`users=124`、`payment_orders=446`、`user_subscriptions=103`、`subscription_entitlement_periods=112`、`usage_facts=19474`。
- 已将 PostgreSQL dump 实际恢复到隔离的 PostgreSQL 18 临时实例，以上五张关键表计数全部一致；临时容器和临时数据卷已清理。为避免覆盖新容器启动后可能产生的运行数据，未对仍完整保留的在线本地库执行重复覆盖恢复。

## 验证结果

- 通过：`go test ./...`。
- 通过：前端 `pnpm typecheck`、`pnpm lint:check`、`pnpm test:run`、`pnpm build`。
- 通过：脚本测试，共 8 项。
- 通过：定向的订阅周窗口数据库集成测试。
- 通过：`http://127.0.0.1:18080/health` 与 `http://127.0.0.1:8080/health` 均返回 200。
- 通过：未认证的 `GET /api/v1/payment/plans` 返回预期 401；应用启动后未见 FATAL、panic、迁移、TLS 或数据库错误日志。
- 当前可调度上游为 `sub2api-latest-openai-upstream`，指向 `http://host.docker.internal:18086/v1`；旧 CPA 上游账号仍保留但已不可调度。

## 已知验证边界

- `go test -tags=integration ./...` 仍因既有测试夹具污染和 PostgreSQL 微秒精度断言失败，未在本次发布范围内修改无关测试。
- 当前进程未注入临时 `SUB2API_PUBLIC_API_KEY`，因此未执行会产生真实请求和计费的公网 `/v1/models`、`/v1/responses`、`/v1/chat/completions`、图片 smoke；未读取、导出或记录任何用户 API Key。
