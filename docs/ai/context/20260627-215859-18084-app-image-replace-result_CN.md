# 18084 应用容器镜像替换结果

时间：2026-06-27 21:58 JST

## 目标

按用户确认的方案执行：只替换公网链路中的 `sub2api-candidate` 应用容器镜像，保留 18084 数据层和 nginx 指向。

公网拓扑保持：

```text
Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317
```

## 执行摘要

- 本地 `main` HEAD：`e4704061d fix: 统一认证中间件计费准入`
- 源镜像：`sub2api-smtp-test:20260627-214036`
- 新候选镜像：`sub2api-candidate:20260627-214036-e4704061d`
- 镜像 digest：`sha256:00f926480c5f01e59a06796a1f8e9bfe569598d40e2a236a266880702d7bb4bc`
- 旧候选镜像：`sub2api-candidate:20260626-220602-payment-template-30e66c82580f`
- 替换对象：仅 `sub2api-candidate` 应用容器
- 保留对象：
  - `sub2api-candidate-postgres`
  - `sub2api-candidate-redis`
  - nginx 配置
  - 18085 测试栈

## 备份与迁移状态

- 替换前已对 `sub2api-candidate-postgres` 做逻辑备份：
  - `deploy/backups/20260627-215617-sub2api-candidate-before-app-image-replace.dump`
  - 大小约 `14M`
- 替换前 `schema_migrations` 行数：`191`
- 替换后 `schema_migrations` 行数：`191`
- 本次未停止、删除或重建数据库和 Redis。

## 验证结果

容器状态：

- `sub2api-candidate`：`sub2api-candidate:20260627-214036-e4704061d`，`127.0.0.1:18084->8080/tcp`，healthy
- `sub2api-candidate-postgres`：仍运行，healthy
- `sub2api-candidate-redis`：仍运行
- `sub2api-smtp-test`：仍运行在 `127.0.0.1:18085->8080/tcp`，healthy

健康检查：

- `http://127.0.0.1:18084/health` 返回 `{"status":"ok"}`
- `http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`
- `Host: api.aaccx.pw http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`
- `https://api.aaccx.pw/health` 返回 `{"status":"ok"}`
- `http://127.0.0.1:18085/health` 返回 `{"status":"ok"}`

公网和本机路由：

- 本机 nginx `Host: aaccx.pw`：
  - `/`：200
  - `/dashboard`：200
  - `/purchase`：200
  - `/subscriptions`：200
  - `/api/v1/settings/public`：200
- 公网 `https://aaccx.pw`：
  - `/dashboard`：200
  - `/purchase`：200
  - `/api/v1/settings/public`：200
- 无效 Key 请求 `https://api.aaccx.pw/v1/models` 返回 `401 INVALID_API_KEY`，确认仍落在 Sub2API 认证路径。

18084 公开设置非敏感状态：

- `registration_enabled=true`
- `email_verify_enabled=true`
- `password_reset_enabled=true`
- `payment_enabled=true`
- `purchase_subscription_enabled=false`
- `site_name=天才程序员小站`
- `version=0.1.138`

前端资源：

- `assets/app-index-CXmPznNo.js`
- `assets/index-nffSQZgD.css`
- `assets/pkg-i18n-CRLwLFIo.js`
- `assets/pkg-misc-CjRx2-Hi.js`
- `assets/pkg-misc-DB0Q8XAf.css`
- `assets/pkg-vue-BqGtxt06.js`

日志摘要：

- 新 `sub2api-candidate` 日志出现 `Server started on 0.0.0.0:8080`。
- 未见 `panic`、`fatal` 或启动失败。
- 存在 `trusted_proxies` 和 CORS 配置警告，不影响本次容器健康检查和路由验证。

## 结论

18084 公网候选应用容器已替换为 18085 验证过的新前后端镜像；18084 数据库、Redis、nginx 指向和 18085 测试栈均保留。公网入口和本机入口验证正常。
