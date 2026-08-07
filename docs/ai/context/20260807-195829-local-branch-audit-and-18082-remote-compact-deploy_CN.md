# 本地分支审计与 18082 remote compact 修复发布

## 本地 Git 审计

- 本地 `main` 在发布前工作树干净，HEAD 为 `1c46ee5559be8a44e329ebe3858b8fa254bdda67`（`fix: 兼容 Codex remote compact 输出别名`）。
- `codex/fix-traffic-pack-card-layout`、`codex/official-upstream-migration`、`codex/update-codex-guide-images` 均已被 `main` 包含，不存在待合并提交。
- `D:\CodeWorkSpace\sub2api-billing-loss-fix-20260805` 是 detached 工作树：其中 `usage_billing_repo.go` 的未提交差异已与 `main` 提交 `c90b9bee4` 完全一致，且主分支已有带 `unit` 标签的回归测试，未重复提交。
- 该 detached 工作树中的未跟踪 `deploy/docker-compose.production-billing-fix.yml` 固定 `BILLING_FINAL_MULTIPLIER=15`，与当前生产 `18` 不一致且重复现有发布配置，因此保留在原工作树、未纳入 `main`、未删除。

## 构建与替换

- 基于当前本地 `main` 执行：
  - `docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml build sub2api`
  - `docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --no-deps --force-recreate sub2api`
- 新镜像为 `deploy-sub2api:latest`，镜像 ID 为 `sha256:46a1d3267e9233c99b635f30a87e8fad6d868e1f0904a62c459f01ea34cef779`。
- 应用容器由 `a83787dc014f5a7b447bab1d0fded919dab2246448129a5da1f7060ddf3b7f98` 替换为 `6f38f3b803f4b7e1cd1996bb90d8eed5866f75040f10d256f72a1c51a7649850`，新容器状态为 `running healthy`。
- PostgreSQL 容器 ID 保持 `d94d74cddbcb30fd0481c1f20b81cda63a1ea65d5ed6e4c92811c72ce846d7cf`，Redis 容器 ID 保持 `d6ea60b580181b4d084fef022192b623e5db3fa44caa567b186cceda4e00cd66`；未重建数据库、Redis、数据卷、Nginx 或 Cloudflare Tunnel。
- 合并 Compose 配置确认服务端最终倍率仍为 `BILLING_FINAL_MULTIPLIER=18`。

## 验证

- `go test -tags unit ./internal/repository -run '^(TestApplyUsageBillingEffects_FlagsBalanceOverdraft|TestApplyUsageBillingEffectsRecordsDebtForUncoveredTrafficPackCost)$' -count=1` 通过。
- `go test ./internal/service -run 'Compact|Compaction|OpenAIGatewayServiceForward.*RemoteCompact' -count=1` 通过。
- `go test ./internal/handler -run 'Compact|Compaction|OpenAI' -count=1` 通过。
- 构建完成；保留项目既有的 Browserslist 数据、动态导入分包和大 chunk 提示，未影响构建成功。
- `http://127.0.0.1:18082/health`、`http://127.0.0.1:8080/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200。
- `docker exec sub2api-public-nginx-local nginx -t` 通过。未对用户 API Key 发起真实模型请求，避免额外扣费；remote compact 的协议映射由上述服务与处理器回归测试覆盖。
