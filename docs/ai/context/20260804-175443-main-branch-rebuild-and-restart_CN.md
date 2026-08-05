# 主分支重建与 18082 重启

## 背景

- 管理员要求检查本地未合并分支，将其合并到主分支后，以最新代码重建并重启 18082 服务。
- 当前主分支含有未提交的支付、余额套餐、流量卡、迁移和使用方法相关改动；发布构建以该工作区作为最新代码来源，不覆盖或丢弃这些改动。

## 分支核验

- `codex/fix-traffic-pack-card-layout` 已是 `main` 的祖先。
- `codex/official-upstream-migration` 已是 `main` 的祖先。
- 两个本地分支均无需要再次合并的独有提交。

## 构建前修复

- `backend/internal/repository/usage_billing_repo.go` 中 `sql.Tx.ExecContext` 的两个返回值曾只接收一个，导致后端测试和 Docker 构建无法编译。
- 修复为显式忽略 `sql.Result` 并处理 `error`，不改变余额套餐后续到账撤销逻辑。

## 发布步骤

1. 运行相关后端与前端测试。
2. 停止当前映射 `127.0.0.1:18082` 的应用容器。
3. 从当前主分支工作区构建镜像。
4. 使用 `deploy/docker-compose.dev.yml` 和 `deploy/docker-compose.18082.yml` 仅重建 `sub2api` 服务，保留 PostgreSQL、Redis 与持久卷。
5. 验证 18082 健康检查、本地 Nginx 链路与公网入口。
