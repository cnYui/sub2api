# 当前修改合并、GitHub 同步与 Docker 重启发布

## Git

- 当前工作区原本就在本地 `main`，已将现有修改统一提交为 `fb821052e`（`chore: 汇总当前模型价格与流量卡发布改动`）。
- 提交内容包含当前工作区的模型广场倍率测试修订、210 号流量卡迁移、上下文文档和 `AGENTS.md` 记忆更新。
- 已推送到私有 GitHub `fork/main`；本地 `main` 与远端 `fork/main` 均指向 `fb821052eaea86517f274be5496083102edbccd4`。
- 推送前端定向测试 14/14 通过，生产构建通过；`go test ./internal/service` 全量测试仍有既有外部模拟/并发相关失败，未发现由本次改动直接引起的失败。

## Docker

- 已执行 Docker Desktop 引擎重启。PostgreSQL、Redis、Nginx 和应用容器均自动恢复，随后仅强制替换 `sub2api-official-18082` 应用容器。
- 新应用镜像 manifest：`sha256:368cdaec0987b33521f585d7ec6c7ca032ef4331549b19163735f35b6aa8e6bd`。
- 应用运行态 `BILLING_FINAL_MULTIPLIER=17`，凭证 Secret 仍挂载到 `/run/secrets/account_credentials_encryption_key`。
- PostgreSQL、Redis、Nginx 未在应用替换阶段重建；Docker 重启导致它们统一恢复启动，但数据卷未重建。

## 验证

- 应用容器状态为 `running healthy`。
- 本地应用 `127.0.0.1:18082/health`：200。
- 本地 Nginx `127.0.0.1:8080/health`：200。
- `aaccx.pw/health`、`www.aaccx.pw/health`、`api.aaccx.pw/health`：均为 200。
- `https://api.aaccx.pw/usage-guide` 跟随跳转后返回 200。
- 数据库已登记 `210_add_traffic_pack_30usd_7cny.sql`，商品 `traffic_30usd_7cny` 字段为 `7.00 CNY / 30.0000000000 USD / 28 天 / all / for_sale=true`。
