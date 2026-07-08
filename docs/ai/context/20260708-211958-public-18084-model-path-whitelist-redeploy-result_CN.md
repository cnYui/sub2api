# 公网 18084 模型路径白名单与裸路径拦截发布结果

> 2026-07-08 21:19 JST 完成。

## 发布信息

- 发布时间：2026-07-08 21:10-21:19 JST
- 本地 HEAD：`4bf902234`（`fix: 补齐模型路径白名单与裸路径拦截`）
- 新镜像：`sub2api-candidate:20260708-211028-4bf902234-model-path-whitelist`
- image id：`sha256:c437fe2492f9a404ac369e84563ba5870d751fcf9d501b0f4e4582e0dc3e7189`
- 旧容器：`sub2api-candidate-before-promote-20260708-211028`（保留）
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

## 未执行项

- 未修改 nginx / Cloudflare Tunnel
- 未重建 Postgres / Redis
- 未推送 personal / origin

## 备份文件（沿用上次发布）

- Postgres dump：`deploy/backups/20260708-092542-sub2api-candidate-postgres-before-rmb-balance-affiliate.dump`（36MB，权限 600）
- Redis RDB：`deploy/backups/20260708-092542-sub2api-candidate-redis-before-rmb-balance-affiliate.rdb`（130KB，权限 600）

## 回滚信息

旧容器 `sub2api-candidate-before-promote-20260708-211028` 保留在 Docker 中，如需应用层回滚：

```bash
docker stop sub2api-candidate && docker rm sub2api-candidate && docker rename sub2api-candidate-before-promote-20260708-211028 sub2api-candidate && docker start sub2api-candidate
```
