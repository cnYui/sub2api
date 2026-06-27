# 本地 main 端口 5174 使用公网数据库启动计划

## 目标

- 在本机 `http://127.0.0.1:5174` 重新启动本地 `main` worktree 的最新代码，供浏览器测试。
- 登录已有账号必须使用当前公网运行态的数据源，避免创建隔离测试库导致账号不存在。

## 当前判断

- `main` worktree：`/Users/wujianxiang/CodeSpace/sub2api`
- 当前 HEAD：`0df3031508f04b73bf6c92aafd4d37b34ad73f48`
- 本地 `main` 与 `origin/main` 分叉：本地 ahead 48、behind 47。为避免破坏本地历史，本次不自动 `pull/rebase/merge`。
- 前端 Vite 端口由 `VITE_DEV_PORT` 控制；API 代理目标由 `VITE_DEV_PROXY_TARGET` 控制。
- 当前公网链路的 Sub2API 后端本机入口是 `http://127.0.0.1:18080`，它连接当前公网使用的数据源。

## 执行方案

1. 确认 `5174` 是否已有进程占用；如有，优先停止旧的本地 Vite 进程。
2. 确认 `http://127.0.0.1:18080/health` 可用，证明公网后端链路仍在。
3. 在 `frontend/` 启动：

   ```bash
   VITE_DEV_PORT=5174 VITE_DEV_PROXY_TARGET=http://127.0.0.1:18080 pnpm run dev -- --host 0.0.0.0
   ```

4. 验证 `http://127.0.0.1:5174` 可访问，并确认 `/api` 请求由 Vite 代理到 `127.0.0.1:18080`。

## 风险与取舍

- 该方案只重启本地前端开发服务器，不替换公网 Docker 容器，不中断公网用户请求。
- 如果要测试的是后端 Go 代码变更，则仅启动 Vite 不够，需要另起一个连接同一数据库的后端实例；这会涉及端口、Redis、定时任务和写入并发风险，需要单独设计。
- 本次不会打印或记录数据库密码、JWT secret、内部 token、API Key。
