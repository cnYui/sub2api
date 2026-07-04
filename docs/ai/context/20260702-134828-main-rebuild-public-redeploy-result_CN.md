# 本地 main 重建并替换公网 Sub2API 结果

## 执行目标

- 按用户要求使用当前本地 `main` 分支代码重新构建 Sub2API 镜像。
- 将公网入口中的 `sub2api` 应用容器替换为新镜像，使公网前端静态资源来自当前本地 main 构建产物。

## 构建来源

- main worktree：`/Users/wujianxiang/CodeSpace/sub2api`
- 分支：`main`
- HEAD：`6ef887a8d`
- 构建 context：`/Users/wujianxiang/CodeSpace/sub2api`
- Dockerfile：`/Users/wujianxiang/CodeSpace/sub2api/Dockerfile`
- 发布脚本：`deploy/redeploy-sub2api-image.sh --yes --foreground`

## 替换范围

- 已重新构建镜像：`weishaw/sub2api:latest`
- 新镜像 ID：`sha256:b195fcb03a925a0adda2026d75f6be42111162d2f7a023a74f4d44b80512a287`
- 镜像创建时间：`2026-07-02T04:47:11.320333591Z`
- 已重建容器：`sub2api`
- 新容器启动时间：`2026-07-02T04:47:16.047638468Z`
- 未重启/未重建：`sub2api-postgres`、`sub2api-redis`。

## 前端资源变化

- 发布前公网资源：
  - `assets/app-index-DKtOnsbF.js`
  - `assets/index-BMta9z_W.css`
- 发布后公网资源：
  - `assets/app-index-DBKABD7p.js`
  - `assets/index-B--asykV.css`
  - `assets/pkg-i18n-CRLwLFIo.js`
  - `assets/pkg-misc-CjRx2-Hi.js`
  - `assets/pkg-misc-DB0Q8XAf.css`
  - `assets/pkg-vue-BqGtxt06.js`
- `https://aaccx.pw/purchase` 也返回同一组新资源。
- `https://aaccx.pw/assets/index-B--asykV.css` 校验：
  - 长度：`203083`
  - 包含 `z-index:35`
  - 包含 `z-index:40`

## 验证结果

- `sub2api`：`Up ... (healthy)`，端口 `127.0.0.1:18080->8080/tcp`。
- `sub2api-postgres`：仍 healthy。
- `sub2api-redis`：仍 healthy。
- `http://127.0.0.1:18080/health`：200。
- `http://127.0.0.1:8080/health`：200。
- `https://api.aaccx.pw/health`：200。
- 发布后日志显示服务正常启动并监听 `0.0.0.0:8080`。

## 注意

- 本次没有修改源码。
- 当前工作树仍有未提交上下文文档和 `AGENTS.md` 记忆更新。
- 本次未打印或记录 `deploy/.env.scheme-a.local` 内容。
