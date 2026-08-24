# 退款真实支付准入发布

## Git

- 本地 `main` 提交 `0df1bd25f`：`fix: 限制仅真实支付余额套餐可退款`。
- 已推送到私有 GitHub `fork/main`。
- 发布后新增本记录并同步到 `fork/main`。

## 验证

- `pnpm build` 通过。
- `go test ./internal/service` 通过。
- `go build ./cmd/server` 通过。
- 前端此前全量测试为 201 个测试文件、1397 个测试通过。

## Docker 发布

- 基于当前 `main` 构建 `deploy-sub2api:latest` 成功，镜像摘要：`sha256:a56e65317eedbc8ab95da7f59c545952309a2b6c68bd7f8d3e5d035f6085aa17`。
- 使用 `docker-compose.dev.yml` 与 `docker-compose.18082.yml`，仅执行 `sub2api` 服务 `--no-deps --force-recreate`。
- `sub2api-official-18082` 当前 `running healthy`；运行时 `BILLING_FINAL_MULTIPLIER=18`，ZPay 直连域名仍在 `NO_PROXY`/`no_proxy` 中。
- PostgreSQL 和 Redis 未重建，均保持 `running healthy`；凭证密钥挂载保持不变。
- `http://127.0.0.1:18082/health`、`http://127.0.0.1:8080/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 和 `https://aaccx.pw/usage-guide` 均返回 HTTP 200。
