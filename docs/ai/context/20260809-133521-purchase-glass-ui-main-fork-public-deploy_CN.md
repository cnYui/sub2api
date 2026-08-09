# 购买页玻璃卡片 main 同步与公网发布

## 代码同步

- 本地 `main` 已快进合并购买页改动提交 `16864f66d`。
- `cnYui/sub2api` 已存在并确认是 `Wei-Shaw/sub2api` 的 Fork。
- Fork 原有 `main` 与本地 `main` 历史不连续，先保留为 `backup/main-before-local-main-20260809-1202`，再使用 `--force-with-lease` 将 Fork 的 `main` 更新为 `16864f66d`。

## 发布范围

- 从干净工作树的 `16864f66d` 构建 `deploy-sub2api:latest`，避免工作区其它未提交改动进入镜像。
- 仅使用 Compose 强制替换 `sub2api-official-18082` 应用容器。
- PostgreSQL、Redis、持久化数据目录、Nginx 与 Cloudflare Tunnel 均未重建；Nginx 仅在语法检查通过后执行平滑重载。

## 验证

- 新应用容器使用新构建镜像并达到 `running healthy`。
- `BILLING_FINAL_MULTIPLIER=16`、最小余额预检和账户凭证 Secret 挂载均保持原有运行配置。
- `http://127.0.0.1:18082/health`、本地 Nginx、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 200。
- `https://aaccx.pw/purchase` 返回 200，支付结算信息接口在发布后返回 200。
