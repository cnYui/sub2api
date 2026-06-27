# 本地 main 端口 5174 使用公网数据库启动结果

## 结果

- 已在本机启动本地 `main` worktree 的前端开发服务。
- 访问地址：`http://127.0.0.1:5174/`
- 运行会话：`screen` detached 会话 `sub2api-frontend-5174`
- 监听进程：`node` 监听 `*:5174`
- API 代理目标：`http://127.0.0.1:18080`
- 当前后端数据源：沿用公网运行态 Sub2API 后端的数据源，因此可登录已有账号测试。

## 使用的代码

- worktree：`/Users/wujianxiang/CodeSpace/sub2api`
- 分支：`main`
- HEAD：`0df3031508f04b73bf6c92aafd4d37b34ad73f48`
- 注意：本地 `main` 与 `origin/main` 仍是分叉状态，本次没有执行 `pull`、`rebase`、`merge` 或推送。

## 启动命令

```bash
screen -dmS sub2api-frontend-5174 bash -lc 'cd /Users/wujianxiang/CodeSpace/sub2api/frontend && VITE_DEV_PORT=5174 VITE_DEV_PROXY_TARGET=http://127.0.0.1:18080 /opt/homebrew/bin/node ./node_modules/vite/bin/vite.js --host 0.0.0.0 --port 5174'
```

## 验证

- `screen -ls` 显示 `71485.sub2api-frontend-5174 (Detached)`。
- `lsof -nP -iTCP:5174 -sTCP:LISTEN` 显示 `node` 正在监听 `*:5174`。
- `curl -I http://127.0.0.1:5174` 返回 `HTTP/1.1 200 OK`。
- `curl http://127.0.0.1:5174/api/v1/settings/public` 返回 `code=0`、`message=success`。
- `curl http://127.0.0.1:18080/health` 返回 `{"status":"ok"}`。

## 过程记录

- 旧的 `5174` Vite 进程代理目标不正确，已先停止。
- 直接用 `pnpm run dev` 会触发 pnpm 依赖检查，并因非 TTY 下拒绝清理 `node_modules` 失败；为避免误删依赖，改用已安装的本地 Vite 二进制入口。
- 普通 detached/nohup 方式下 Vite 会很快退出，因此最终使用 `screen` 提供稳定伪终端。
- 本次没有打印或记录任何数据库密码、JWT secret、内部 token 或 API Key。
