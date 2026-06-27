# Sub2API 公网重启更新设计

## 目标

将当前本地 `sub2api` 分支最新代码更新到公网展示，同时保留当前 5174 端口作为本地预览验收参考。本文只写设计，不执行启动、重启、部署、推送或公网切换动作。

## 当前已确认状态

- 当前工作目录：`/Users/wujianxiang/CodeSpace/sub2api`
- 当前分支：`main`
- 当前本地最新提交：`c1ef7c2a merge: 记录本地前端分支合并`
- 当前本地分支状态：`main...origin/main [ahead 17, behind 80]`
- 当前 tracked 工作区：无未提交 tracked 改动。
- 当前仍存在未跟踪本地临时文件：`.tmp-*`、sqlite/dump 备份、`deploy/.env.scheme-a.runtime`，这些文件不进入部署包、不提交、不复制到公网。
- 5174 本地预览端口：`node` 进程正在监听，`GET/HEAD http://127.0.0.1:5174/purchase` 返回 200。
- 公网链路记忆：`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> Sub2API 127.0.0.1:18080 -> CLIProxyAPI 127.0.0.1:8317`。
- 本机实际监听：
  - `nginx` 监听 `*:8080`
  - Docker Desktop 端口转发监听 `127.0.0.1:18080`
- `deploy/.env.scheme-a.local` 的非敏感端口配置：
  - `BIND_HOST=127.0.0.1`
  - `SERVER_PORT=18080`
  - `SERVER_MODE=release`
  - `RUN_MODE=standard`
- 健康检查必须使用 `GET /health`，不能用 `HEAD /health`；`GET http://127.0.0.1:18080/health` 和 `GET http://127.0.0.1:8080/health` 均返回 `{"status":"ok"}`。
- 当前 shell 中 `docker` 命令不可用，执行前需要先确认 Docker CLI 可用路径，或改用当前机器已有的 Docker Desktop/Compose 操作入口。

## 关键判断

5174 是 Vite 本地预览服务，只适合确认前端页面效果，不应该直接暴露给公网，也不应该让 nginx/Cloudflare Tunnel 代理到 5174。公网应继续由 Sub2API 后端服务提供嵌入式前端资源，这样 `/api/*`、`/v1/*`、认证、支付、控制台路由和 CSP 都沿用现有生产链路。

当前公网 Sub2API 从 `127.0.0.1:18080` 提供服务，且该端口来自 Docker Desktop 转发。因此推荐更新方式是：基于当前本地 `main@c1ef7c2a` 构建新的 Sub2API Docker 镜像，保留数据库和 Redis 容器，只强制重建 `sub2api` 应用容器。

## 方案对比

### 方案 A：构建本地 Docker 镜像并只重建 app 容器（推荐）

流程：

1. 以 5174 页面作为更新前视觉验收参考。
2. 确认当前 `main` 是要发布的提交。
3. 给当前线上镜像打 rollback 标签。
4. 用当前代码构建 `weishaw/sub2api:latest` 本地镜像，Dockerfile 会先构建前端，再用 `go build -tags embed` 生成嵌入前端的后端二进制。
5. 使用 `deploy/.env.scheme-a.local` 和 `deploy/docker-compose.local.yml` 只重建 `sub2api` 服务。
6. 验证 `18080`、`8080` 和公网域名页面。

优点：

- 与当前 18080 Docker 运行形态一致。
- 不改 nginx、Cloudflare Tunnel、数据库、Redis。
- 前端资源被嵌入后端二进制，公网刷新后会使用新构建。
- 可以通过 rollback 镜像快速恢复。

风险：

- 当前 shell 中没有 `docker` 命令，需要先修复 Docker CLI 可用性。
- `weishaw/sub2api:latest` 本地标签会被覆盖，所以必须先打 rollback 标签。

### 方案 B：本机构建嵌入前端的二进制并替换 systemd 服务

流程：

1. `pnpm --dir frontend run build`
2. `cd backend && go build -tags embed -o bin/server ./cmd/server`
3. 替换 systemd 所用二进制并 `systemctl restart sub2api`

不推荐原因：

- 当前实际 18080 服务由 Docker Desktop 转发，不是 `deploy/sub2api.service` 描述的 systemd 8080 服务。
- 直接切到 systemd 会改变运行形态，可能绕过当前 Docker 数据卷、环境变量和端口绑定。

### 方案 C：让公网临时代理到 5174

不推荐，且不执行。

原因：

- 5174 是本地开发/预览服务，不是生产服务。
- 它不负责后端 API、生产 CSP、进程守护和容器化运行。
- 把公网指向 5174 会增加误暴露、稳定性和缓存问题。

## 推荐执行设计

### 1. 发布前确认

只读确认：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
git status --short --branch --untracked-files=all
git log --oneline --decorate -8
curl -sS http://127.0.0.1:5174/purchase >/tmp/sub2api-5174-purchase.html
curl -sS http://127.0.0.1:18080/health
curl -sS http://127.0.0.1:8080/health
```

通过标准：

- 当前分支为 `main`，最新提交为 `c1ef7c2a` 或用户再次确认的新提交。
- 除 `.tmp-*`、备份文件和本地 env 外没有新的未处理改动。
- 5174 `/purchase` 可访问。
- `GET /health` 返回 `{"status":"ok"}`。

### 2. 发布前测试

执行前应重新跑一次与本次前端更新直接相关的测试：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/frontend
pnpm vitest run src/components/payment/__tests__/SubscriptionPlanCard.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/__tests__/HomeView.spec.ts src/__tests__/visualThemeSource.spec.ts
pnpm build
```

通过标准：

- 4 个测试文件、15 个测试通过。
- `pnpm build` 成功，只允许出现既有 Vite 动态导入、大 chunk、Browserslist 数据过期警告。

### 3. Docker CLI 可用性确认

当前 shell 中 `docker` 不在 PATH。实际执行前先确认 Docker CLI：

```bash
command -v docker
docker version
docker compose version
```

如果仍不可用，需要先打开 Docker Desktop 并确认 CLI 安装路径，或使用已有部署环境中的 Docker/Compose 命令入口。这个前置条件未满足时不进入公网更新。

### 4. 备份 rollback 镜像标签

在 Docker CLI 可用后执行：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
ROLLBACK_TAG="weishaw/sub2api:rollback-$(date +%Y%m%d-%H%M%S)"
docker image tag weishaw/sub2api:latest "$ROLLBACK_TAG"
printf '%s\n' "$ROLLBACK_TAG"
```

说明：

- 这只保留当前本地镜像引用，不触碰数据库。
- 后续如果新镜像异常，可把该 rollback tag 重新标回 `weishaw/sub2api:latest`。

### 5. 构建当前代码镜像

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
COMMIT="$(git rev-parse --short HEAD)"
docker build \
  --build-arg COMMIT="$COMMIT" \
  -t weishaw/sub2api:latest \
  .
```

说明：

- 根目录 `Dockerfile` 会构建前端并复制到后端 `internal/web/dist`，再用 `-tags embed` 构建后端。
- 不需要单独把 5174 的 dev server 暴露给公网。

### 6. 只重建 Sub2API app 容器

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/deploy
docker compose \
  --env-file .env.scheme-a.local \
  -f docker-compose.local.yml \
  up -d --no-deps --force-recreate sub2api
```

约束：

- 不执行 `docker compose down`。
- 不重建 `postgres` 和 `redis`。
- 不修改 nginx 配置。
- 不修改 Cloudflare Tunnel。

### 7. 更新后验证

本地链路：

```bash
curl -sS http://127.0.0.1:18080/health
curl -sS http://127.0.0.1:8080/health
curl -I http://127.0.0.1:8080/purchase
curl -I http://127.0.0.1:8080/home
```

公网链路：

```bash
curl -I https://aaccx.pw/purchase
curl -I https://aaccx.pw/home
curl -I https://api.aaccx.pw/v1/models
```

通过标准：

- `18080` 和 `8080` 的 `GET /health` 返回 200。
- `/purchase`、`/home` 返回 200。
- 公网域名返回 200 或符合未认证接口预期的状态码。
- 浏览器中 `/purchase` 可看到：
  - 默认展示订阅页签；
  - 订阅在左、充值在右；
  - 套餐卡片右侧显示 `¥29元`、`¥39元`、`¥59元`；
  - 卡片描述为「月度订阅-时间 30天，日限额 x刀，24点刷新」。

### 8. 回滚设计

如果更新后公网页面异常、接口异常或容器无法稳定运行，使用第 4 步输出的 rollback tag：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/deploy
docker image tag "$ROLLBACK_TAG" weishaw/sub2api:latest
docker compose \
  --env-file .env.scheme-a.local \
  -f docker-compose.local.yml \
  up -d --no-deps --force-recreate sub2api
curl -sS http://127.0.0.1:18080/health
curl -I http://127.0.0.1:8080/purchase
```

回滚同样只替换 app 容器，不动数据库、Redis、nginx 和 Cloudflare Tunnel。

## 不执行事项

- 不启动新的 5174 服务。
- 不停止当前 5174 服务。
- 不把公网代理切到 5174。
- 不运行 `docker build`、`docker compose up`、`docker compose restart`、`systemctl restart`。
- 不推送远端。
- 不拉取远端 `origin/main`。
- 不提交或复制 `.tmp-*`、sqlite/dump 备份、`deploy/.env.scheme-a.runtime`。

## 执行前需要用户确认

执行公网更新前至少确认两点：

1. 是否接受使用当前本地 `main@c1ef7c2a` 作为发布版本，而不是先处理 `origin/main` 的 `behind 80`。
2. 是否已经具备可用 Docker CLI；如果当前 shell 仍无 `docker` 命令，需要先补齐 Docker CLI 环境。
