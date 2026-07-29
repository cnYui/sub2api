# OpenAI 原子计费授权公网部署结果

## 部署结果

已将本地 `main` 上完成重构与验证的外层服务恢复至公网入口。公网 Nginx `sub2api-public-nginx-local` 已重新创建并处于 `running` 状态。

## 重建与保护措施

- 外层服务继续使用已完成无缓存构建的 `sub2api-localdev-sub2api:latest`；运行镜像已在此前与构建产物核对一致。
- Nginx 重新拉取并使用 `nginx:1.27-alpine`，镜像摘要为 `sha256:65645c7bb6a0661892a8b03b89d0743208a18dd2f3f17a54ef4b76fb8e2f2a10`。
- 在替换前使用临时容器执行 `nginx -t`，配置语法检查通过。
- 新 Nginx 保持原运行边界：`127.0.0.1:8080:8080` 回环绑定、只读挂载 `deploy/nginx-public-local-18080.conf`、`unless-stopped` 重启策略，以及到 `host.docker.internal:18080` 的上游代理。
- 未重建或清空外层、内层 PostgreSQL/Redis 容器和 volume；既有数据库完整备份与外层回滚镜像保持可用。

## 验证证据

- `127.0.0.1:18080/health`、`127.0.0.1:18086/health` 和 `127.0.0.1:8080/health` 均为 200。
- `https://aaccx.pw/health` 与 `https://api.aaccx.pw/health` 均为 200。
- 本地 Nginx 与根域名的无认证 `/v1/models` 均为应用层预期 401。
- `api.aaccx.pw` 的完全无认证 `/v1/models` 返回 403；回环 Nginx 使用相同 `Host` 返回 401，且公网请求携带无效 Bearer 后同样返回 401。由此确认 403 来自 Cloudflare 边缘的无 Authorization 请求策略，不是本次应用、Nginx 配置或镜像重建造成，也不阻断带 Authorization 的请求到达应用层。
- 全部验证仅使用健康检查、无认证请求和无效占位 Bearer；未使用有效 API Key，未发起模型调用或产生上游计费。

## 回滚边界

如需临时下线公网入口，可停止 `sub2api-public-nginx-local`，外层和内层服务不受影响。若需回滚外层版本，可使用 `sub2api-localdev-sub2api:rollback-20260729-093220`，并结合 `deploy/backups/public-billing-rollout-20260729-093220` 的已验证备份执行。
