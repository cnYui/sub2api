# Sub2API 本地重部署计划

## 背景

用户要求执行 `sub2api-local-redeploy`，目标是把本地 `main` 最新代码构建为 `weishaw/sub2api:latest`，并替换当前公网入口使用的本地 `sub2api` 容器。

## 必须事项

- 使用当前签出 `main` 的 worktree 作为 Docker build context。
- 只重建 `sub2api` 服务，不重启 Postgres、Redis、CLIProxyAPI、nginx、Cloudflare Tunnel。
- 不打印、记录或提交 `deploy/.env.scheme-a.local` 内容。
- 等待发布脚本退出，不能留下仍需本轮处理的后台会话。
- 发布后验证本地 `http://127.0.0.1:18080/health` 和公网 `https://api.aaccx.pw/health`。

## 风险

- `docker build` 阶段旧容器仍可服务。
- `docker compose up -d --no-deps --force-recreate sub2api` 会短暂重建容器，期间 `https://api.aaccx.pw/v1/*` 可能出现连接中断、502 或流式请求断开。
- API Key 和用户数据不应因本次操作改变；风险集中在短暂可用性中断。

## 执行步骤

1. 定位 `main` worktree，检查分支、HEAD 和工作区状态。
2. 确认 `deploy/redeploy-sub2api-image.sh` 可执行。
3. 前台执行重部署脚本，显式指定 Docker CLI、build context、Dockerfile、Compose 文件和 env 文件路径。
4. 验证 `sub2api` 容器状态、本地健康检查、公网健康检查。
5. 检查公网 dashboard 当前 JS/CSS 资源 hash；如拿到 CSS 路径，确认移动端遮罩相关 `z-index:35` 仍存在。
