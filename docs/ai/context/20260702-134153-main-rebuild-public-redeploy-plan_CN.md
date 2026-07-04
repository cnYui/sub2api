# 本地 main 重建并替换公网 Sub2API 计划

## 目标

- 使用当前本地 `main` worktree 的代码重新构建 `weishaw/sub2api:latest`。
- 将公网入口使用的 `sub2api` 应用容器替换为新镜像，确保前端静态资源来自当前本地 `main` 构建产物。

## 当前状态

- main worktree：`/Users/wujianxiang/CodeSpace/sub2api`
- 当前分支：`main`
- 当前 HEAD：`6ef887a8d`
- 当前 Docker 容器：`sub2api`、`sub2api-postgres`、`sub2api-redis` 三个运行中容器。
- 当前公网链路：`api.aaccx.pw/aaccx.pw -> nginx 127.0.0.1:8080 -> sub2api 127.0.0.1:18080`。
- 当前公网资源：
  - `assets/app-index-DKtOnsbF.js`
  - `assets/index-BMta9z_W.css`
  - `assets/pkg-i18n-CRLwLFIo.js`
  - `assets/pkg-misc-CjRx2-Hi.js`
  - `assets/pkg-misc-DB0Q8XAf.css`
  - `assets/pkg-vue-BqGtxt06.js`
- 当前工作树存在上一轮上下文记录相关改动：`AGENTS.md` 已修改，`docs/ai/context/` 有未跟踪文档；这些不会改变前端业务构建逻辑。

## 执行方式

- 使用项目发布脚本：`deploy/redeploy-sub2api-image.sh`
- 构建 context：`/Users/wujianxiang/CodeSpace/sub2api`
- Dockerfile：`/Users/wujianxiang/CodeSpace/sub2api/Dockerfile`
- Compose 文件：`deploy/docker-compose.local.yml`
- Compose env：`deploy/.env.scheme-a.local`，只传路径，不打印内容。
- 只重建 `sub2api` 应用容器：`docker compose up -d --no-deps --force-recreate sub2api`
- 不重启、不删除、不重建 `sub2api-postgres` 与 `sub2api-redis`。

## 验证

- 发布脚本退出码为 0。
- `sub2api` 容器为 healthy。
- `http://127.0.0.1:18080/health` 返回 200。
- `http://127.0.0.1:8080/health` 返回 200。
- `https://api.aaccx.pw/health` 返回 200。
- `https://aaccx.pw/dashboard` 中前端 JS/CSS 资源 hash 发生更新。

## 回滚

- 如新容器异常，优先使用 Docker 镜像历史或前一镜像 ID 回滚 `sub2api` 应用容器。
- 数据层不在本次操作范围内；本次不修改 PostgreSQL/Redis 数据。
