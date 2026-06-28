# 18084 替换为 18085 最新代码的切换方案讨论稿

时间：2026-06-27 21:51 JST

## 当前拓扑

公网链路：

```text
Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317
```

本地测试链路：

```text
127.0.0.1:18085 -> sub2api-smtp-test -> sub2api-smtp-test-postgres / sub2api-smtp-test-redis
```

## 已核实事实

- `sub2api-candidate` 当前镜像：`sub2api-candidate:20260626-220602-payment-template-30e66c82580f`
- `sub2api-candidate` 端口：`127.0.0.1:18084->8080/tcp`
- `sub2api-candidate` 数据层：
  - `DATABASE_HOST=sub2api-candidate-postgres`
  - `REDIS_HOST=sub2api-candidate-redis`
- `sub2api-candidate` 运行态包含公网候选需要的 URL allowlist：
  - `SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=true`
  - `SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=true`
- `sub2api-smtp-test` 当前镜像：`sub2api-smtp-test:20260627-214036`
- `sub2api-smtp-test` 端口：`127.0.0.1:18085->8080/tcp`
- `sub2api-smtp-test` 数据层：
  - `DATABASE_HOST=sub2api-smtp-test-postgres`
  - `REDIS_HOST=sub2api-smtp-test-redis`
- `18084` 和 `18085` 前端资源 hash 当前相同：
  - `assets/index-CXmPznNo.js`
  - `assets/index-nffSQZgD.css`
  - `assets/pkg-i18n-CRLwLFIo.js`
  - `assets/pkg-misc-CjRx2-Hi.js`
  - `assets/pkg-misc-DB0Q8XAf.css`
  - `assets/pkg-vue-BqGtxt06.js`
- `18084/api/v1/settings/public`：
  - `registration_enabled=true`
  - `email_verify_enabled=true`
  - `password_reset_enabled=true`
  - `payment_enabled=true`
  - `purchase_subscription_enabled=false`
- `18085/api/v1/settings/public`：
  - `registration_enabled=true`
  - `email_verify_enabled=true`
  - `password_reset_enabled=true`
  - `payment_enabled=false`
  - `purchase_subscription_enabled=false`

## 不推荐方案

不建议把 nginx 从 `127.0.0.1:18084` 直接切到 `127.0.0.1:18085`。

原因：

- 18085 是测试栈，连接独立测试数据库和 Redis，不是当前公网用户、API Key、订单、支付配置的事实源。
- 18085 当前 `payment_enabled=false`，直接切流会改变公网购买入口表现。
- 18085 `RUN_MODE=standard`、`SERVER_MODE=debug`，与 18084 当前候选运行态不一致。

## 推荐方案

推荐只替换 `sub2api-candidate` 应用容器镜像，保留 `18084` 端口、容器名、Docker 网络、候选 Postgres 和候选 Redis。

执行思路：

1. 将当前已验证镜像 `sub2api-smtp-test:20260627-214036` 重新打 tag 为新的候选镜像，例如：
   - `sub2api-candidate:20260627-214036-e4704061d`
2. 从旧 `sub2api-candidate` 提取环境变量到临时文件，不打印敏感值。
3. 停止旧 `sub2api-candidate`，先重命名为临时备份容器。
4. 用新候选镜像启动新的 `sub2api-candidate`：
   - 容器名仍为 `sub2api-candidate`
   - 网络仍为 `sub2api-candidate-network`
   - 端口仍为 `127.0.0.1:18084:8080`
   - env 仍复用旧 18084 运行态
5. 等新容器 healthy 后删除备份容器。
6. 如果新容器 unhealthy 或超时，删除新容器并把备份容器改回 `sub2api-candidate` 后启动。
7. 验证：
   - `curl -fsS http://127.0.0.1:18084/health`
   - `curl -fsS http://127.0.0.1:8080/health`
   - `curl -fsS -H 'Host: api.aaccx.pw' http://127.0.0.1:8080/health`
   - `curl -fsS http://127.0.0.1:18084/api/v1/settings/public` 只检查非敏感开关
   - 用无效 Key 请求 `/v1/models` 或 `/v1/chat/completions`，确认仍由 Sub2API 返回认证错误而不是 nginx/yui.web

## 预期影响

- 会有一次很短的 `18084` 应用容器重启窗口。
- nginx 配置不变，Cloudflare Tunnel 不变。
- `sub2api-candidate-postgres` 和 `sub2api-candidate-redis` 不动。
- 18085 测试栈不动，仍可作为回看环境。

## 待用户确认

是否按“只替换 18084 应用容器镜像，保留 18084 数据层和 nginx 指向”的方案执行。
