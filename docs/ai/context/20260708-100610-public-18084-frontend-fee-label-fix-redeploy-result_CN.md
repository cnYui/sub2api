# 公网 18084 前端手续费标签隐藏热修复发布结果

> 2026-07-08 10:06 JST 完成。

## 发布信息

- 发布时间：2026-07-08 09:49-10:06 JST
- 本地 HEAD：`a93548db1`（`fix: hide balance payment fee label`）
- 新镜像：`sub2api-candidate:20260708-094943-a93548db1-frontend-fee-label-fix`
- image id：`sha256:fd481ca9d2d71ae272e0a8a3f9c863d3c196f1151e0d7c8fc0ed47e889b3e648`
- 旧容器：`sub2api-candidate-before-promote-20260708-094943`（保留）
- 上次发布（09:35）备份仍然有效：Postgres dump 36MB + Redis RDB 130KB

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

### 前端资源

- `/purchase` 返回新前端资源 `app-index-CXRS0hdS.js`（前次 `app-index-CrRjitF5.js`）

## 未执行项

- 未修改 nginx / Cloudflare Tunnel
- 未重建 Postgres / Redis
- 未推送 personal / origin

## 备份文件（沿用上次发布）

- Postgres dump：`deploy/backups/20260708-092542-sub2api-candidate-postgres-before-rmb-balance-affiliate.dump`（36MB，权限 600）
- Redis RDB：`deploy/backups/20260708-092542-sub2api-candidate-redis-before-rmb-balance-affiliate.rdb`（130KB，权限 600）

## 回滚信息

旧容器 `sub2api-candidate-before-promote-20260708-094943` 保留在 Docker 中，如需应用层回滚：

```bash
docker stop sub2api-candidate && docker rm sub2api-candidate && docker rename sub2api-candidate-before-promote-20260708-094943 sub2api-candidate && docker start sub2api-candidate
```
