# 18084 下架与重建结果

## 结论

已完成当前公网上 18084 服务的下架、Sub2API 与 CLIProxyAPI 的 Docker 重建、共享网络恢复、数据回灌与备份再封存。

## 实际结果

- Docker 运行环境已恢复，磁盘空间已释放。
- `sub2api-candidate` 已重新启动并通过健康检查。
- `sub2api-candidate-postgres`、`sub2api-candidate-redis` 均为健康状态。
- `cliproxyapi-local-dev` 已在 Docker 中运行，并加入共享网络 `sub2api-cliproxy-local`。
- `sub2api-candidate` 已加入 `sub2api-candidate-network` 与 `sub2api-cliproxy-local`。
- 公网 `https://aaccx.pw/dashboard`、`https://api.aaccx.pw/health`、`http://127.0.0.1:18084/health` 均返回 200。
- `https://aaccx.pw/api/v1/settings/public` 返回 200。
- 未登录访问 `https://aaccx.pw/api/v1/auth/me` 与 `https://aaccx.pw/api/v1/usage/dashboard/quota` 返回 401，符合预期。
- CLIProxyAPI TLS 连接可建立，证书链仍是本地自签链，未导入本机信任库时会显示校验告警，但握手成功。
- PostgreSQL 与 Redis 的原始候选数据已恢复，恢复后账号、API Key、订阅等数据可见。
- 新备份已生成并验证：Postgres custom dump 可被 `pg_restore --list` 读取，Redis RDB 可被 `redis-check-rdb` 校验。

## 关键观察

- `yui.web` 本身是启动状态，监听 `4173`；此前浏览器 502 的直接原因是 Nginx 路由仍指向正在下架重建的 `18084`，不是 `yui.web` 未启动。
- 账号登录错误不是 PostgreSQL / Redis 断连，而更像是认证信息本身不匹配或未登录状态访问。

## 风险与备注

- 这次恢复过程中曾短暂把 Sub2API candidate 指到了空数据目录，随后已恢复到原始数据。
- `sub2api-candidate` 镜像本身已重建成功，当前容器状态正常。
- 未修改线上 DNS、未推送代码。
