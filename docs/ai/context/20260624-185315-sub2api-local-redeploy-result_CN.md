# Sub2API 本地镜像替换重部署记录

## 时间

- 2026-06-24 18:53 JST

## 操作

- 按 `sub2api-local-redeploy` 流程执行当前 Sub2API 重部署。
- 使用 main worktree：`/Users/wujianxiang/CodeSpace/sub2api`。
- 使用 HEAD：`c636ff10 feat: simplify available channels into model prices page`。
- 当前 main worktree 初始状态干净，但缺少技能要求的 `deploy/redeploy-sub2api-image.sh`。
- 从历史提交 `b65f07ff feat: add sub2api redeploy and restart scripts` 恢复 `deploy/redeploy-sub2api-image.sh`，不恢复任何 env 或密钥文件。
- 前台执行镜像替换脚本，构建 `weishaw/sub2api:latest` 并只重建 `sub2api` 容器。

## 验证

- 镜像替换脚本成功退出。
- `sub2api` 容器状态：`Up` 且 `healthy`，端口为 `127.0.0.1:18080->8080/tcp`。
- 本地健康检查 `http://127.0.0.1:18080/health` 返回 `{"status":"ok"}`。
- 公网健康检查 `https://api.aaccx.pw/health` 返回 `{"status":"ok"}`。
- 公网 Dashboard 当前资源：
  - JS：`/assets/app-index-Ssa87SNp.js`
  - CSS：`/assets/index-DQzRIYzN.css`
- 公网 CSS 检查：`index-DQzRIYzN.css` 包含 `z-index:35`，移动端遮罩修复仍存在。

## 注意

- 本次未读取、打印或记录 `deploy/.env.scheme-a.local` 内容。
- `deploy/redeploy-sub2api-image.sh` 当前为从历史提交恢复的新文件，后续如需长期保留应纳入正常提交。
