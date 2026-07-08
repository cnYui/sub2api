# 公网 18084 强制正式 /v1/* API 路径发布结果

> 2026-07-08 20:43 JST 完成。

## 发布信息

- 发布时间：2026-07-08 20:34-20:43 JST
- 本地 HEAD：`cc845e468`（`docs: track project context updates`，含 `merge: enforce formal v1 api paths`）
- 新镜像：`sub2api-candidate:20260708-203429-cc845e468-formal-v1-api`
- image id：`sha256:c8f24c939a8064ce697883681578ae0ca22642b5eb6117ab0674d3038fd83de1`
- 旧容器：`sub2api-candidate-before-promote-20260708-203429`（保留）
- 上次备份（2026-07-08 09:25，Postgres dump 36MB + Redis RDB 130KB）仍有效

## 发布后验证

### 容器状态

- `sub2api-candidate`：healthy，新镜像运行中
- `sub2api-candidate-postgres`：healthy，未重建
- `sub2api-candidate-redis`：healthy，未重建

### Health

| endpoint | status |
|----------|--------|
| 18084/health | 200 |
| 8080/health | 200 |
| api.aaccx.pw/health | 200 |
| aaccx.pw/dashboard | 200 |
| aaccx.pw/purchase | 200 |

### 日志

- 无 checksum mismatch / migration failed / panic / 端口冲突

## 背景

- `codex/formal-v1-only-api` 分支的工作已通过 `merge: enforce formal v1 api paths` 合并进 main
- Nginx 已在公网 8080 入口拦截裸路径（`/models`、`/responses`、`/chat/completions` 等）
- 本次发布将后端应用也升级到包含 formal v1 API 强制路由的最新 main

## 未执行项

- 未修改 nginx / Cloudflare Tunnel（已在更早阶段完成）
- 未重建 Postgres / Redis
- 未推送 personal / origin

## 备份文件（沿用上次发布）

- Postgres dump：`deploy/backups/20260708-092542-sub2api-candidate-postgres-before-rmb-balance-affiliate.dump`（36MB，权限 600）
- Redis RDB：`deploy/backups/20260708-092542-sub2api-candidate-redis-before-rmb-balance-affiliate.rdb`（130KB，权限 600）

## 回滚信息

旧容器 `sub2api-candidate-before-promote-20260708-203429` 保留在 Docker 中，如需应用层回滚：

```bash
docker stop sub2api-candidate && docker rm sub2api-candidate && docker rename sub2api-candidate-before-promote-20260708-203429 sub2api-candidate && docker start sub2api-candidate
```
