# Sub2API 与 CLIProxyAPI 本地开发环境启动结果

## 结果

- 本地页面：`http://127.0.0.1:8080/`
- 本地登录页：`http://127.0.0.1:8080/login`
- CLIProxyAPI：`https://127.0.0.1:8317`
- PostgreSQL：`sub2api-postgres-dev`
- Redis：`sub2api-redis-dev`
- Sub2API：`sub2api-dev`
- CLIProxyAPI：`cliproxyapi-local-dev`

所有宿主机端口只绑定 `127.0.0.1`，未启动 Nginx、Cloudflare Tunnel 或公网入口。

## 数据恢复

- 附件四项 SHA256 校验全部通过。
- PostgreSQL dump 来自 PostgreSQL 18.4，共 991 个 TOC 条目。
- 恢复后核心数据：116 个用户、1 个上游账号、152 个 API Key、7 个套餐、204 条迁移记录。
- Redis 未恢复附件 RDB，从 0 个键启动；Sub2API 启动后自动重建缓存和调度键。
- 本地副本已关闭 SMTP、邮件验证、密码重置、在线支付、订阅到期通知、余额通知、账号限额通知、渠道监控和 ops 邮件通知。
- 原始 ZIP 与 dump 未修改，本地副本可随时重新恢复。

## CLIProxyAPI 边界

- 未读取、复制或挂载 `C:\Users\yui\.cli-proxy-api`。
- `D:\CodeWorkSpace\CLIProxyAPI-private\auths` 当前没有账号 JSON。
- CLIProxyAPI 静态 API Key 从本地恢复数据库的唯一 `pool_mode=true` 上游账号读取，只写入 Git 忽略的 `config.yaml`。
- 用户后续将账号 JSON 放入仓库 `auths/` 后，可由文件监听器加载；必要时重启 `cliproxyapi-local-dev`。

## 通信验证

- `GET http://127.0.0.1:8080/health` 返回 200。
- Sub2API 容器通过内置 CA 访问 `https://host.docker.internal:8317` 成功。
- CLIProxyAPI `/v1/models` 静态 Key 鉴权成功，空账号池返回 0 个模型。
- Sub2API `/v1/models` 使用恢复出的用户 Key 返回 13 个数据库配置模型。
- 最小 `gpt-5.6-sol` 聊天请求从 Sub2API 到达 CLIProxyAPI；本地账号配置保留 `pool_mode_retry_status_codes=[401,403,429]`，本次 502 由通用池模式重试链执行 3 次同账号重试，最终因 CLIProxyAPI 无认证账号返回 502。
- CLIProxyAPI 容器访问 Sub2API `/health` 返回 200。
- 正确 token/HMAC 的失败 usage event 回调返回 200、`skipped=true`，未新增 usage fact。
- 首页和登录页已在浏览器实际渲染；登录控件可用，控制台无前端错误。

## Redis Compose 修复

### 根因

原配置将 `redis-server` 和参数写成多条 shell 命令。第一条进程进入前台后，`--save`、`--appendonly`、`--appendfsync` 和 `--requirepass` 均未执行；同时空的 `REDISCLI_AUTH` 会让健康检查发送空 AUTH。

### 修改

- 四份 Compose 改为单条 `exec redis-server ...`，在容器内按需展开 `REDIS_PASSWORD`。
- Redis 容器显式接收 `REDIS_PASSWORD`。
- 健康检查在有密码时鉴权，无密码时取消 `REDISCLI_AUTH`。
- 新增 `deploy/verify-redis-compose-command.mjs`，验证四份 Compose 的真实渲染结果和空密码分支。

## 本地操作

启动或更新 Sub2API：

```powershell
cd D:\CodeWorkSpace\sub2api\deploy
docker compose --project-name sub2api-localdev --env-file .env.local-dev -f docker-compose.dev.yml -f docker-compose.override.yml up -d --build
```

停止 Sub2API 本地栈但保留数据：

```powershell
cd D:\CodeWorkSpace\sub2api\deploy
docker compose --project-name sub2api-localdev --env-file .env.local-dev -f docker-compose.dev.yml -f docker-compose.override.yml stop
```

启动或停止 CLIProxyAPI：

```powershell
docker start cliproxyapi-local-dev
docker stop cliproxyapi-local-dev
```

验证状态：

```powershell
docker ps --filter name=sub2api --filter name=cliproxyapi-local-dev
Invoke-WebRequest http://127.0.0.1:8080/health
```

## 未提交文件

- `deploy/.env.local-dev`、`deploy/docker-compose.override.yml`、`deploy/backups/` 为 Git 忽略的本地运行文件。
- CLIProxyAPI 的 `.env`、`config.yaml`、`auths/`、`logs/` 为 Git 忽略的本地运行文件。
- 本次未提交、未推送、未修改公网运行态。
