# 公网 18084 下架与 Sub2API/CLIProxyAPI Docker 重部署计划

## 背景

用户要求阅读 `docs/SUB2API_CLIPROXYAPI_DEPLOYMENT_RUNBOOK_CN.md` 后，将当前公网上部署的 18084 端口服务下架，并按部署代码重新进行 Sub2API、CLIProxyAPI 的 Docker 部署与 Docker 网络构建。

当前已知运行态约束：

- 公网链路当前为 `Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`。
- 当前公网事实源容器为 `sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis`。
- 任何修改 DB、Redis、容器、Nginx 或公网链路前，必须先备份并验证备份可读。
- 本次用户明确授权停掉当前 18084 服务并封存数据库，但仍不得删除数据库/Redis volume 或数据目录。

## 目标

1. 只读确认当前运行态：代码状态、容器、Compose project、网络、挂载、Nginx 指向、健康检查、CLIProxyAPI 容器与端口。
2. 备份当前 `sub2api-candidate-postgres` 与 `sub2api-candidate-redis`，并验证备份可读。
3. 停止当前 18084 相关公网服务容器，数据库以停止状态封存，保留数据目录、volume 与备份文件。
4. 创建或确认 Sub2API 与 CLIProxyAPI 的专用 Docker bridge 网络。
5. 按仓库部署代码重新构建并启动 CLIProxyAPI 与 Sub2API Docker 服务。
6. 验证新服务的容器健康、端口、Docker 网络、TLS、Sub2API 到 CLIProxyAPI 连通性、公网 health。
7. 记录实施结果。

## 非目标

- 不删除数据库、Redis volume 或 `deploy/candidate/*` 数据目录。
- 不执行 `docker compose down -v`、`docker volume prune`、`docker system prune --volumes`。
- 不把完整 API Key、内部转发密钥、HMAC secret、SMTP 密码、支付密钥写入文档或终端输出。
- 不擅自恢复整库；如需数据回滚必须另行授权。
- 不提交、不推送代码，除非用户另行要求。

## 执行步骤

### 1. 只读确认

- 检查 Git 工作区与最近提交。
- 检查 `docs/ai/context` 未跟踪文件。
- 检查 Docker 容器、Compose project、端口、挂载与网络。
- 检查 Nginx 当前是否仍指向 `127.0.0.1:18084`。
- 检查 18084、8080、公网 health。
- 检查 CLIProxyAPI 当前容器、配置路径、认证目录、TLS 证书目录、端口和网络。
- 检查磁盘空间，避免备份或构建中途失败。

### 2. 备份与验证

- 使用 `pg_dump --format=custom --no-owner --no-privileges` 备份当前 Postgres。
- 使用 `pg_restore --list` 验证 dump 可读。
- 使用 Redis `SAVE`、复制 RDB，并用 `redis-check-rdb` 验证可读。
- 只记录备份路径与大小，不记录备份内容。

### 3. 下架与封存

- 停止当前 `sub2api-candidate` 应用容器。
- 停止 `sub2api-candidate-postgres` 与 `sub2api-candidate-redis`，以停止状态封存。
- 停止旧 CLIProxyAPI 容器，但保留其配置、认证目录、日志和证书目录。
- 不删除旧数据容器、数据目录和备份。

### 4. Docker 网络构建

- 创建或确认共享网络 `sub2api-cliproxy-local`。
- 保持 Sub2API 数据网络与 CLIProxyAPI 共享网络分离：CLIProxyAPI 只加入共享网络，不加入 Sub2API PostgreSQL/Redis 网络。

### 5. 重新部署

- 在 CLIProxyAPI 仓库使用 `docker-compose.sub2api-local.yml` 构建/启动 `cliproxyapi-local-dev`，复用现有配置、认证目录、日志目录和 TLS 目录。
- 在 Sub2API 仓库构建新镜像。
- 基于候选 Compose 或发布脚本启动 Sub2API、Postgres、Redis，并确保 Sub2API 同时能连接数据网络和共享网络。
- 如果保留原公网入口，应用继续绑定 `127.0.0.1:18084`，使 Nginx 无需变更即可恢复公网。

## 验证

- `docker ps` 中新 Sub2API、Postgres、Redis、CLIProxyAPI 状态正常。
- `curl http://127.0.0.1:18084/health`、`curl http://127.0.0.1:8080/health`、`curl https://api.aaccx.pw/health` 返回 200。
- CLIProxyAPI 8317 为 HTTPS/TLS，未带内部 key 的 `/v1/models` 返回 401 可接受。
- Sub2API 容器内访问 `https://cliproxyapi:8317/v1/models` 能完成 TLS 并到达 CLIProxyAPI。
- Sub2API 上游账号 `base_url` 需要切换到 Docker 网络服务名时，应为 `https://cliproxyapi:8317/v1`，且 `pool_mode=true`、账号可调度。
- 日志中无 panic、migration failed、DB/Redis 初始化失败、x509、invalid url scheme、account_select_failed、auth_unavailable 等异常。

## 回滚边界

- 应用层失败：停止新应用容器，保留备份与旧数据，按旧容器/镜像恢复或重新启动封存前容器。
- 数据层失败：不自动恢复数据库；先保留现场并等待用户授权。
- CLIProxyAPI 失败：回退到旧容器或旧配置目录，保持 Sub2API 数据容器不变。
