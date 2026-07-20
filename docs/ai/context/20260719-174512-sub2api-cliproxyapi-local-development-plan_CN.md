# Sub2API 与 CLIProxyAPI 本地开发环境实施计划

> **执行要求：** 按步骤恢复隔离的本地运行环境；任何敏感值只进入 Git 忽略文件，验证输出必须脱敏。

**目标：** 使用两个仓库的当前源码和附件 PostgreSQL 备份启动完整的本地开发链路。

**架构：** PostgreSQL、Redis、Sub2API 和 CLIProxyAPI 均运行在 Docker Desktop。Sub2API 通过宿主机映射的 HTTPS 8317 访问 CLIProxyAPI；CLIProxyAPI 通过宿主机映射的 8080 回传 usage event。

**技术栈：** Docker Compose、PostgreSQL 18、Redis 8、Go 1.26、Vue 3、CLIProxyAPI。

---

### 任务一：准备隔离的本地运行文件

**文件：**

- 新建：`deploy/.env.local-dev`
- 新建：`deploy/docker-compose.override.yml`
- 新建：`D:\CodeWorkSpace\CLIProxyAPI-private\config.yaml`
- 新建：`D:\CodeWorkSpace\CLIProxyAPI-private\.env`
- 新建目录：`D:\CodeWorkSpace\CLIProxyAPI-private\auths`

- [ ] 检查目标端口和现有 Docker 容器，避免覆盖其他本地服务。
- [ ] 生成本地数据库密码、JWT secret、TOTP key、usage event token 和 HMAC secret。
- [ ] 创建仅绑定 `127.0.0.1:8080` 的 Sub2API Compose 覆盖配置。
- [ ] 创建启用 TLS、使用仓库空账号池的 CLIProxyAPI 配置。

### 任务二：启动 Docker Desktop 并校验备份

**输入：**

- `D:\xwechat_files\wxid_4lkns2swsaad22_1df8\msg\file\2026-07\sub2api-runtime-backups.zip`

- [ ] 启动 Docker Desktop，等待 Docker Engine 可用。
- [ ] 将备份解压到 `deploy/backups/local-runtime-20260719`。
- [ ] 按附件 `SHA256SUMS` 校验 PostgreSQL、Redis 和恢复说明文件。
- [ ] 使用 PostgreSQL 18 `pg_restore -l` 验证 dump TOC 可读。

### 任务三：恢复 PostgreSQL，保持 Redis 为空

- [ ] 仅启动本地 PostgreSQL 容器。
- [ ] 将 custom dump 复制进容器并恢复到空的 `sub2api` 数据库。
- [ ] 检查核心表、迁移记录、管理员、套餐和上游账号数量。
- [ ] 在本地副本关闭主动邮件外发开关。
- [ ] 启动空 Redis，并确认没有载入附件 RDB。

### 任务四：生成 CLIProxyAPI 本地配置

- [ ] 从恢复数据库定位 `pool_mode=true` 的 CLIProxyAPI OpenAI 上游账号。
- [ ] 读取其 HTTPS base URL 和 API Key，敏感值不打印到终端摘要。
- [ ] 将 API Key 写入 Git 忽略的 CLIProxyAPI `config.yaml`。
- [ ] 配置 `8317` TLS、仓库内 `auths/` 和 usage event 回调环境变量。

### 任务五：构建并启动两个项目

- [ ] 从 `CLIProxyAPI-private` 当前源码构建 Go 1.26 Docker 镜像并启动容器。
- [ ] 从 `sub2api` 当前源码构建前端与后端镜像并启动容器。
- [ ] 检查 PostgreSQL、Redis、CLIProxyAPI 和 Sub2API 日志，不输出 secret 或完整 API Key。

### 任务六：验证完整通信链路

- [ ] 请求 Sub2API `/health` 并确认成功。
- [ ] 请求 CLIProxyAPI HTTPS 入口并确认 TLS 与 API Key 鉴权可用。
- [ ] 检查恢复数据库中的 CLIProxyAPI 上游配置、池模式和重试状态码。
- [ ] 通过 Sub2API 发起最小模型请求；空账号池时确认失败来自 `auth_unavailable`，而非连接、TLS 或鉴权错误。
- [ ] 检查 CLIProxyAPI usage event 回调未出现连接失败或签名失败。

### 任务七：记录结果

- [ ] 新增本地启动结果文档，记录容器名、端口、验证结论和停止/重启方式。
- [ ] 在根 `AGENTS.md` 写入一条压缩记忆，不记录敏感值。
- [ ] 检查 `docs/ai/context` 未跟踪文档和整个工作区差异。
