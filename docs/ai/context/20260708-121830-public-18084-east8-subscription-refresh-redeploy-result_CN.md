# 公网 18084 东八区订阅用量刷新热修复发布结果

> 2026-07-08 12:18 JST 完成。

## 发布信息

- 发布时间：2026-07-08 11:59-12:18 JST
- 本地 HEAD：`83cf82584`（含 3 个新提交）
  - `feat: add east8 subscription usage refresh`
  - `fix: apply affiliate rebate to traffic packs`
  - `fix: display affiliate rebates in rmb`
- 新镜像：`sub2api-candidate:20260708-115902-83cf82584-east8-subscription-refresh`
- image id：`sha256:d436f7c0d8bcd3c7276d66064e261efd019ab3095c8d5a73d53ad2fd2c983b31`
- 旧容器：`sub2api-candidate-before-promote-20260708-115902`（保留）
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

### 日志

- 无 checksum mismatch / migration failed / panic / 端口冲突

## 发布过程中的测试 stub 修复

本次发布过程中发现 `backend/internal/server/api_contract_test.go` 的 `stubUserSubscriptionRepo` 缺少两个新方法，导致 `go test ./internal/server` 编译失败：

- `CalibrateActiveDailyUsageWindows`
- `CountStaleActiveDailyWindows`

已在本地修复并通过所有后端测试。修复文档见 `docs/ai/context/20260708-104500-test-stub-missing-methods-fix_CN.md`。

## 未执行项

- 未修改 nginx / Cloudflare Tunnel
- 未重建 Postgres / Redis
- 未推送 personal / origin

## 备份文件（沿用上次发布）

- Postgres dump：`deploy/backups/20260708-092542-sub2api-candidate-postgres-before-rmb-balance-affiliate.dump`（36MB，权限 600）
- Redis RDB：`deploy/backups/20260708-092542-sub2api-candidate-redis-before-rmb-balance-affiliate.rdb`（130KB，权限 600）

## 回滚信息

旧容器 `sub2api-candidate-before-promote-20260708-115902` 保留在 Docker 中，如需应用层回滚：

```bash
docker stop sub2api-candidate && docker rm sub2api-candidate && docker rename sub2api-candidate-before-promote-20260708-115902 sub2api-candidate && docker start sub2api-candidate
```
