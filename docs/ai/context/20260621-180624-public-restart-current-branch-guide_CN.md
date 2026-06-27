# 公网重新启动当前修改分支说明

## 目标

把当前本地修改分支 `codex/teal-refactor-20260621` 重新构建并接回公网运行链路，让公网继续通过现有域名访问 Sub2API 新产物。

本文只写操作说明，不在本文创建时执行重启、构建、推送或公网切换。

## 当前分支状态

- 工作目录：`/Users/wujianxiang/CodeSpace/sub2api`
- 当前分支：`codex/teal-refactor-20260621`
- 当前提交：`78f5c445 docs: 更新使用方法运行记录`
- 当前 tracked 工作区：干净，无未提交改动。
- 当前分支没有 upstream tracking 信息；发布前不要用 `git pull` 隐式改动分支内容。
- `main` 也包含当前提交，说明当前修改分支和本地 `main` 暂时指向同一个提交。

## 公网运行链路

当前公网链路仍按既有架构运行：

```text
Cloudflare Tunnel
  -> nginx 127.0.0.1:8080
  -> Sub2API 127.0.0.1:18080
  -> CLIProxyAPI 127.0.0.1:8317
```

本次重启只应该替换 Sub2API app 容器产物，不应该改动：

- PostgreSQL 数据容器和数据目录；
- Redis 数据容器和数据目录；
- nginx 公网分流配置；
- Cloudflare Tunnel 配置；
- CLIProxyAPI 上游账号池配置；
- yui.web `/shop` 路由归属。

## 前置条件

### 1. 确认 Docker CLI 可用

当前 shell 中执行 `docker` 返回 `command not found`。真正执行公网重启前，必须先让 Docker CLI 可用：

```bash
command -v docker
docker version
docker compose version
```

如果仍无输出或报错，先打开 Docker Desktop，或修复当前 shell 的 Docker CLI PATH。这个条件不满足时不要继续执行后续步骤。

### 2. 确认运行环境端口

生产用本地 env 文件为 `deploy/.env.scheme-a.local`。只检查非敏感项：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
sed -n '/^BIND_HOST=/p;/^SERVER_PORT=/p;/^SERVER_MODE=/p;/^RUN_MODE=/p' deploy/.env.scheme-a.local
```

预期：

```text
BIND_HOST=127.0.0.1
SERVER_PORT=18080
SERVER_MODE=release
RUN_MODE=standard
```

不要打印或记录 `.env` 中的密码、JWT、SMTP、OAuth secret、内部 token。

## 发布前检查

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
git status --short --branch --untracked-files=all
git rev-parse --abbrev-ref HEAD
git rev-parse --short HEAD
git log --oneline --decorate -8
```

通过标准：

- 分支是 `codex/teal-refactor-20260621`，或用户明确确认的新分支。
- `git status --short` 没有需要处理的 tracked 改动。
- 最新提交是要发布到公网的提交。

如分支上后续继续做了重构提交，以实际 `git rev-parse --short HEAD` 为准，不要固定使用本文里的 `78f5c445`。

## 发布前测试

至少执行与本次前端重构相关的验证：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/frontend
pnpm test -- --run
pnpm build
```

如果只想先跑重点测试，可按改动范围追加具体测试文件；但最终接公网前必须至少保证 `pnpm build` 成功。

如后端或 Dockerfile 相关文件有改动，再补：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
go test ./...
```

## 推荐重启方案

采用“本地构建镜像，只重建 app 容器”的方式。

原因：

- 公网真实服务来自 `127.0.0.1:18080` 的 Docker 映射；
- 根目录 `Dockerfile` 会构建前端并嵌入 Go 后端；
- 只重建 `sub2api` 容器即可刷新前端和后端产物；
- 数据库、Redis、nginx、Cloudflare Tunnel 都保持原样，回滚面最小。

不要把公网临时代理到 Vite 预览端口。Vite 只适合本地看页面，不负责生产 API、认证、CSP、进程守护和公网缓存行为。

## 执行步骤

### 1. 给当前线上镜像打回滚标签

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
ROLLBACK_TAG="weishaw/sub2api:rollback-$(date +%Y%m%d-%H%M%S)"
docker image tag weishaw/sub2api:latest "$ROLLBACK_TAG"
printf '%s\n' "$ROLLBACK_TAG"
```

如果 `weishaw/sub2api:latest` 不存在，先停止发布，查清当前 `sub2api` 容器实际使用的镜像：

```bash
docker inspect sub2api --format '{{.Config.Image}}'
```

### 2. 用当前分支构建新镜像

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
COMMIT="$(git rev-parse --short HEAD)"
docker build \
  --build-arg COMMIT="$COMMIT" \
  -t weishaw/sub2api:latest \
  .
```

说明：

- 不需要单独发布 `frontend/dist`。
- 不需要启动 5174。
- 构建失败时不要继续重建容器。

### 3. 只重建 Sub2API app 容器

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/deploy
docker compose \
  --env-file .env.scheme-a.local \
  -f docker-compose.local.yml \
  up -d --no-deps --force-recreate sub2api
```

硬约束：

- 不执行 `docker compose down`。
- 不加 `--volumes`。
- 不重建 `postgres` 和 `redis`。
- 不修改 nginx。
- 不修改 Cloudflare Tunnel。

## 更新后验证

### 1. 容器与本地链路

```bash
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Ports}}\t{{.Status}}' | rg 'sub2api|postgres|redis'
curl -sS http://127.0.0.1:18080/health
curl -sS http://127.0.0.1:8080/health
curl -I http://127.0.0.1:8080/dashboard
curl -I http://127.0.0.1:8080/purchase
```

预期：

- `sub2api` 容器为 running/healthy。
- `GET /health` 返回 200。
- `8080` 通过 nginx 能访问 Sub2API 页面。

### 2. 公网链路

```bash
curl -I https://aaccx.pw/dashboard
curl -I https://aaccx.pw/purchase
curl -I https://aaccx.pw/usage-guide
curl -I https://api.aaccx.pw/v1/models
```

预期：

- `dashboard`、`purchase`、`usage-guide` 返回 200 或前端 SPA 正常入口。
- `/v1/models` 无 Key 时应该返回 Sub2API 风格认证错误；使用有效 Key 时应返回 200。
- `aaccx.pw/shop` 仍归 yui.web，不应被本次重启改变。

### 3. 前端资源验证

```bash
curl -sSL https://aaccx.pw/dashboard | rg '/assets/(app-index|pkg-)'
curl -sSL https://api.aaccx.pw/dashboard | rg '/assets/(app-index|pkg-)'
```

注意：

- 真实 `/assets/pkg-*` 不要反向 rewrite 到 `/assets/vendor-*`。
- 如出现 MIME 错误或白屏，优先检查 nginx 旧 rewrite 和 Cloudflare 缓存。

## 回滚步骤

如果新版本异常，用第 1 步输出的 `$ROLLBACK_TAG` 回滚 app 镜像：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/deploy
docker image tag "$ROLLBACK_TAG" weishaw/sub2api:latest
docker compose \
  --env-file .env.scheme-a.local \
  -f docker-compose.local.yml \
  up -d --no-deps --force-recreate sub2api
curl -sS http://127.0.0.1:18080/health
curl -I http://127.0.0.1:8080/dashboard
```

回滚仍然只替换 app 容器，不动数据库、Redis、nginx、Cloudflare Tunnel。

## 禁止事项

- 不记录完整 API Key、SMTP 密码、OAuth secret、JWT secret、HMAC secret。
- 不执行 `docker compose down -v`。
- 不删除 `postgres_data`、`redis_data`、`deploy/data`。
- 不把公网代理到 Vite 预览端口。
- 不直接拉取 `origin/main` 覆盖当前修改分支。
- 不把 `deploy/.env.scheme-a.runtime`、sqlite/dump、本地临时文件提交进仓库。

## 成功标准

- 当前分支构建出的镜像已被 `sub2api` app 容器使用。
- `127.0.0.1:18080/health` 和 `127.0.0.1:8080/health` 均可用。
- `https://aaccx.pw/dashboard`、`https://aaccx.pw/purchase`、`https://aaccx.pw/usage-guide` 可访问。
- `https://api.aaccx.pw/v1/models` 保持 Sub2API API 行为。
- `aaccx.pw/shop` 仍归 yui.web。
