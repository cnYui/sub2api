# CLIProxyAPI 管理登录与公网链路一次更新设计

## 背景

- 用户在 `https://127.0.0.1:8317/management.html#/login` 输入管理密码后登录失败。
- 页面错误为 `HTTP 404: 未找到 Management API`。
- 当前 `cliproxyapi-local-dev`、`sub2api-dev`、`sub2api-public-nginx-local` 和 Windows `cloudflared` 均已运行。
- `https://api.aaccx.pw/health` 当前返回 200，说明公网入口已经能到 Windows 本地 `127.0.0.1:8080`。

## 根因

- CPA 静态管理页 `/management.html` 已经返回 200。
- CPA 管理 API `/v0/management/*` 仍返回 404。
- 源码注册条件为：`remote-management.secret-key`、`MANAGEMENT_PASSWORD` 或本地 `-password` 任一非空才注册管理 API 路由。
- 当前 `D:\CodeWorkSpace\CLIProxyAPI-private\config.yaml` 中 `remote-management.secret-key` 实际为空，所以后端未注册 `/v0/management/*`。
- 因此这不是密码错误，也不是浏览器缓存问题，而是管理 API 没有启用。

## 目标

一次更新完成三件事：

1. 启用 CPA 管理 API，使 `123123` 能登录本地管理页。
2. 保持 CPA 只绑定 `127.0.0.1:8317`，不直接暴露公网。
3. 验证公网 Sub2API 入口和 Sub2API 到 CPA 的上游链路仍正常。

## 更新方案

- 在 `D:\CodeWorkSpace\CLIProxyAPI-private\config.yaml` 的 `remote-management.secret-key` 写入 `123123` 对应的 bcrypt 校验值，不保存明文密码。
- 保持：
  - `remote-management.allow-remote: true`
  - `remote-management.disable-control-panel: false`
  - `ports: 127.0.0.1:${CLI_PROXY_PORT:-8317}:8317`
- 重启 `cliproxyapi-local-dev`，不重建 Sub2API，不改 DB/Redis/Nginx/Cloudflare。

## 验证标准

1. `cliproxyapi-local-dev` 为 running/healthy。
2. `https://127.0.0.1:8317/healthz` 返回 200。
3. `https://127.0.0.1:8317/management.html` 返回 200。
4. 未带管理密钥访问 `/v0/management/config` 返回 401，而不是 404。
5. 使用用户提供的管理密码访问 `/v0/management/config` 返回 200。
6. 浏览器管理页能登录。
7. `sub2api-dev` 容器内访问 `https://cliproxyapi:8317/v1/models` 未带 CPA key 返回 401，表示网络和 TLS 可达。
8. `https://api.aaccx.pw/health` 返回 200。
9. 公网 `/v1/models` 未带用户 key 返回 403/401 属于预期鉴权结果。

## 风险与回滚

- 风险：CPA 重启期间公网模型请求可能短暂失败。
- 回滚：恢复 `D:\CodeWorkSpace\CLIProxyAPI-private\backups\20260722-122604-before-local-management-panel\config.yaml`，然后重启 `cliproxyapi-local-dev`。
- 不触碰 Sub2API 数据库、Redis、用户余额、套餐、usage facts、Nginx 或 Cloudflare Tunnel。

## 后续

- 如果管理页登录后显示 auth 状态异常，再进入 CPA 账号池排查。
- 如果公网模型请求仍失败，再按 `usage_facts`、Sub2API account 1、Redis 调度快照和 CPA 错误契约分段定位。
