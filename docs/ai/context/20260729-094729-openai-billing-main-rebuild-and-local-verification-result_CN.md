# OpenAI 原子计费授权 main 重构与本地服务验证结果

## 范围

- 本地 `main` 已包含合并提交 `fe1b4b513`。
- 外层服务为 `127.0.0.1:18080`，内层服务为 `127.0.0.1:18086`。
- 公网入口 `sub2api-public-nginx-local` 在整个操作期间保持停止，未恢复公网访问。

## 变更前保护与回滚边界

- 重建前已完整备份外层与内层 PostgreSQL 数据库，备份目录为 `deploy/backups/public-billing-rollout-20260729-093220`。
- `outer-sub2api.dump` 与 `inner-sub2api.dump` 均通过 `pg_restore --list` 可读性校验；全局对象备份同时保留。
- 未删除或重建 PostgreSQL、Redis 容器及其 volume。
- 旧外层镜像保留为 `sub2api-localdev-sub2api:rollback-20260729-093220`，可与数据库备份配合回滚。

## 部署操作

- 使用 `sub2api-localdev` Compose 项目名无缓存构建外层 `sub2api` 镜像，避免错误复用默认 `deploy` 项目名生成的镜像标签。
- 仅强制重建外层 `sub2api-dev` 容器，未重建其依赖服务。
- 容器启动后执行迁移；运行中的外层容器镜像 ID 已与新构建镜像 ID 一致。

## 验证证据

- `http://127.0.0.1:18080/health` 返回 200，外层容器健康状态为 `healthy`。
- `http://127.0.0.1:18086/health` 返回 200。
- 无认证访问 `http://127.0.0.1:18080/v1/models` 返回 401，未发送可能产生上游计费的模型请求。
- 外层数据库已记录 `180_openai_billing_authorizations.sql` 迁移，且存在 `public.billing_authorizations` 表。
- 公网 Nginx 容器状态为 `exited`。

## 结论与后续边界

本地 main 的 OpenAI 原子计费授权代码已完成镜像重构、外层服务替换与非计费验证。公网仍保持下线；如需恢复公网入口，应在另行确认后启动 Nginx 并重新检查 `127.0.0.1:8080/health`。由于未提供专用测试 API Key，本次未执行真实模型调用。
