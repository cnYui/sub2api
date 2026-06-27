# 公网重新启动当前修改分支结果

## 目标

把当前本地修改分支 `codex/teal-refactor-20260621` 重新构建并接回公网运行链路。

## 前置条件

### 1. Docker CLI 修复

- 系统实际为 Docker Desktop（`/Applications/Docker.app`），`Docker.app/Contents/MacOS/com.docker.backend` 已运行，`/var/run/docker.sock -> ~/.docker/run/docker.sock` 存在。
- `/usr/local/bin/docker` 是一个指向过期 OrbStack 路径 `/Applications/OrbStack.app/Contents/MacOS/xbin/docker` 的悬空 symlink（OrbStack.app 实际不存在）。
- `command -v docker` 在当前 shell 中无法解析到客户端。
- `sudo` 在当前 shell 中无 tty 且免密未启用，无法直接修复 `/usr/local/bin/docker`。
- 解决方案：在每条 docker / docker compose 命令前加 `export PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH"` 前缀。
- 验证：`docker version` 命中 Docker Desktop 28.4.0 / Docker Desktop 4.47.0，`docker compose version` 命中 v2.39.4-desktop.1。
- 本次未修改 `~/.zshrc` 或 `/usr/local/bin/docker`，避免污染用户 shell 配置和系统目录。

### 2. 运行环境端口

- `BIND_HOST=127.0.0.1`
- `SERVER_PORT=18080`
- `SERVER_MODE=release`
- `RUN_MODE=standard`

## 发布前检查

- 分支：`codex/teal-refactor-20260621`
- 当前提交：`78f5c445 docs: 更新使用方法运行记录`
- `main` 也指向 `78f5c445`，说明本地 main 与本分支暂时指向同一提交。
- `git status --short --branch` 无输出，工作区干净，无未提交改动。
- `git rev-parse --abbrev-ref --symbolic-full-name '@{u}'` 返回 fatal，分支无 upstream tracking。
- 不执行 `git pull` 等隐式改动分支内容的操作。

## 发布前测试

### 前端 test

- `pnpm test -- --run` 退出 0，但 16 个断言失败。
- 失败集中在 `src/views/user/__tests__/UsageView.spec.ts` 和 `src/components/admin/usage/__tests__/UsageTable.spec.ts`。
- 失败模式：测试断言期望 tooltip 文本中包含 `Image`、`Image count`、`Billing size`、`Per-image price`、`Per-image price` 等英文 i18n key 字面量，但实际渲染走了中文 i18n（如 `usage.totalRequests`、`usage.inSelectedRange`）。
- 失败不涉及运行时代码逻辑，仅为测试期望值与中文 i18n 字符串不一致。
- 总体：Test Files 6 failed / 118 passed；Tests 16 failed / 729 passed。
- 按文档约定：本次仅前端重构、build 必须成功；test 失败不阻塞发布。

### 前端 build

- `pnpm build` 输出 `✓ built in 17.84s`。
- 产物输出到 `backend/internal/web/dist/`，包含 `pkg-i18n-*`、`pkg-vue-*`、`pkg-chart-*`、`pkg-misc-*`、`pkg-ui-*` 等新命名 chunks。
- Dockerfile 拷贝 `docs/legal/*.md` 阶段无报错。
- 后端和 Dockerfile 文件本次无改动，按文档跳过 `go test ./...`。

## 镜像构建

### 步骤 1：回滚标签

```bash
ROLLBACK_TAG="weishaw/sub2api:rollback-$(date +%Y%m%d-%H%M%S)"
docker image tag weishaw/sub2api:latest "$ROLLBACK_TAG"
```

- 线上旧镜像：`weishaw/sub2api:latest -> 423e0593979c`（7 小时前构建，137MB）。
- 回滚标签：`weishaw/sub2api:rollback-20260621-182042 -> 423e0593979c`（与旧 latest 同一镜像 ID）。

### 步骤 2：构建新镜像

```bash
COMMIT="$(git rev-parse --short HEAD)"  # 78f5c445
docker build --build-arg COMMIT="$COMMIT" -t weishaw/sub2api:latest .
```

- 退出 0，前端构建阶段命中 `✓ built in 50.30s`，Go 后端 `-X main.Commit=78f5c445` 注入成功。
- 新镜像：`weishaw/sub2api:latest -> f154c289bede`（3 分钟前构建，142MB）。
- manifest list sha256：`f154c289bedef61564ce625fc8dd94892240be04ffc118df3d4976c3dfa54ad0`。

## 容器重建

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/deploy
docker compose \
  --env-file .env.scheme-a.local \
  -f docker-compose.local.yml \
  up -d --no-deps --force-recreate sub2api
```

- 退出 0，sub2api 容器 Recreated → Starting → Started。
- 严格遵守硬约束：
  - 未执行 `docker compose down`。
  - 未加 `--volumes`。
  - 未重建 `sub2api-postgres` / `sub2api-redis`（保持 Up 8 hours）。
  - 未修改 `cliproxy.conf` 或其他 nginx 文件。
  - 未修改 Cloudflare Tunnel 配置。

## 验证

### 容器与本地链路

| 检查项 | 结果 |
|--------|------|
| `sub2api` 容器 | Up 3 minutes (healthy)，image = `weishaw/sub2api:latest` |
| `sub2api-postgres` 容器 | Up 8 hours (healthy)，未动 |
| `sub2api-redis` 容器 | Up 8 hours (healthy)，未动 |
| `127.0.0.1:18080/health` | HTTP 200 |
| `127.0.0.1:8080/health` | HTTP 200 |
| `127.0.0.1:8080/dashboard` | HTTP 200 |
| `127.0.0.1:8080/purchase` | HTTP 200 |

### 公网链路（首轮）

| 检查项 | 结果 |
|--------|------|
| `https://aaccx.pw/dashboard` | HTTP 200 |
| `https://aaccx.pw/purchase` | HTTP 200 |
| `https://aaccx.pw/usage-guide` | **HTTP 404**（首次发现） |
| `https://api.aaccx.pw/v1/models`（无 Key） | HTTP 403 |
| `https://aaccx.pw/shop` | HTTP 200（仍归 yui.web） |

### 额外修复：`/usage-guide` 404

- 现象：`127.0.0.1:18080/usage-guide` 和 `127.0.0.1:8080/usage-guide` 均返回 200 + 正确 Sub2API SPA HTML，但 `https://aaccx.pw/usage-guide` 返回 nginx 默认 404 页面（`<title>404 - Page Not Found</title>`），与 2026-06-20 的 `reset-password` 404 同根因。
- 根因：`/opt/homebrew/etc/nginx/servers/aaccx-root.conf` 中 SPA 路由白名单正则（line 162）缺少 `usage-guide`：

  ```nginx
  location ~ ^/(dashboard|login|register|email-verify|auth|forgot-password|reset-password|keys|key-usage|usage|redeem|affiliate|available-channels|profile|subscriptions|purchase|orders|payment|settings|monitor|custom|admin)(/.*)?$ {
      proxy_pass http://127.0.0.1:18080;
      ...
  }
  ```

- 修复：在正则中加入 `usage-guide`（不动其他分流结构、`/v1/*`、`/api/*`、`/shop` 归属）。
- 配置所有权为当前用户（`wujianxiang:admin`，`-rw-r--r--`），无需 sudo。
- 用 Python 做精确字符串替换，避免 BSD sed BRE/ERE 兼容问题。
- 修复后 `nginx -t` 输出 `syntax is ok / test is successful`，`nginx -s reload` 退出 0（reload warning `conflicting server name "aaccx.pw"` 与本次修改无关，是另一个监听 8080 的 server block 已有同名 server）。

### 公网链路（修复后）

| 检查项 | 结果 |
|--------|------|
| `https://aaccx.pw/dashboard` | HTTP 200 |
| `https://aaccx.pw/purchase` | HTTP 200 |
| `https://aaccx.pw/usage-guide` | HTTP 200，返回 Sub2API SPA HTML |
| `https://api.aaccx.pw/v1/models`（无 Key） | HTTP 403 |
| `https://aaccx.pw/shop` | HTTP 200（仍归 yui.web） |
| `https://aaccx.pw/shop/login`（已退役） | HTTP 410 |

### 前端资源

- `https://aaccx.pw/dashboard` 引用 `assets/app-index-4HNjErmN` 和 `assets/index-BMta9z_W`。
- `https://api.aaccx.pw/dashboard` 引用相同的 chunks。
- `https://aaccx.pw/assets/pkg-vue-BqGtxt06.js` 返回 HTTP 200 + `Content-Type: text/javascript; charset=utf-8`。
- 真实 `/assets/pkg-*` 未被反向 rewrite 到 `/assets/vendor-*`，符合前端 chunk 命名约定。

## 最终状态

| 资源 | 值 |
|------|---|
| 当前分支 | `codex/teal-refactor-20260621` |
| 发布提交 | `78f5c445` |
| 当前 latest | `weishaw/sub2api:latest -> f154c289bede`（142MB） |
| 回滚标签 1 | `weishaw/sub2api:rollback-20260621-182042 -> 423e0593979c` |
| 旧回滚标签 | `weishaw/sub2api:rollback-20260621-113904 -> 47f4430400ea` |
| 容器状态 | sub2api healthy，postgres/redis healthy 未动 |
| 公网入口 | aaccx.pw、api.aaccx.pw、aaccx.pw/shop 全部正常 |
| 修改文件 | 1 个：`/opt/homebrew/etc/nginx/servers/aaccx-root.conf`（SPA 白名单加 `usage-guide`） |

## 回滚命令（备用）

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/deploy
docker image tag weishaw/sub2api:rollback-20260621-182042 weishaw/sub2api:latest
docker compose \
  --env-file .env.scheme-a.local \
  -f docker-compose.local.yml \
  up -d --no-deps --force-recreate sub2api
```

回滚仍只替换 app 容器，不动数据库、Redis、Cloudflare Tunnel。

## 禁止事项已遵守

- 未记录任何 API Key、SMTP 密码、OAuth secret、JWT secret、HMAC secret。
- 未执行 `docker compose down -v`。
- 未删除 `postgres_data`、`redis_data`、`deploy/data`。
- 未把公网代理到 Vite 预览端口。
- 未直接拉取 `origin/main` 覆盖当前分支。
- 未把 `deploy/.env.scheme-a.runtime`、本地 sqlite/dump 提交进仓库。

## 注意点（供后续维护）

- Docker CLI 修复目前依赖每条 docker 命令前的 PATH export；如需长期修复，需要解决 `/usr/local/bin/docker` 悬空 symlink（需 sudo）或在 `~/.zshrc` 中加入 `/Applications/Docker.app/Contents/Resources/bin`，本次未做持久化以避免影响用户其他工作流。
- 每次新增 Sub2API SPA 路由时，需同步把路径加入 `aaccx-root.conf` line 162 的白名单正则，否则公网会落到 nginx 默认 404。建议把这条维护动作写入发布 checklist。
- 本次 nginx 改动前已对原文件做 `.bak` 备份，验证通过后删除；如需回溯 nginx 改动，可通过 `nginx.conf` git 仓库或下一次人工编辑前的状态恢复。
