# 2026-08-20 main 本地重启与公网替换

## 发布前检查

- 当前分支为 `main`，工作区干净，没有未跟踪文件。
- 本地具名分支均已是 `main` 的祖先，没有未合并分支。
- 未执行退款重试，20 笔已手动处理的历史失败订单保持原状态。

## 执行

- 重启既有 `sub2api-official-18082` 应用容器，恢复后为 `healthy`。
- 基于当前 `main` 构建 `deploy-sub2api:latest`，镜像 manifest 为 `sha256:b2a892da1f52985e411cd31951e659c6685dbbe939c0f9c7723c155ef3f76132`。
- 使用 `docker compose ... up -d --no-deps --force-recreate sub2api` 仅替换公网应用容器。PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷未重建。
- 运行容器保留 `BILLING_FINAL_MULTIPLIER=18`、原凭证 secret 挂载，以及 `zpayz.cn` 的 `NO_PROXY`/`no_proxy` 直连配置。

## 验证

- `127.0.0.1:18082/health`：200。
- 本地 Nginx `127.0.0.1:8080/health`：200。
- `aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 的 `/health`：均 200。
- 三个公网域名的 `/usage-guide`：均 200。
- 应用、PostgreSQL、Redis、Nginx 运行状态正常；应用启动日志显示服务监听 `0.0.0.0:8080`。

## 推送

当前 `main` 提交 `fc4baed43` 已准备推送到 `fork/main`（`cnYui/sub2api`），本次不推送到 `upstream`。
