# 公网 18084 发布人民币余额支付与邀请返利重构结果

> 2026-07-08 09:35 JST 完成。

## 发布信息

- 发布时间：2026-07-08 09:25-09:35 JST
- 本地 HEAD：`6f00a311a`（`docs: record rmb balance payment rebuild`）
- 新镜像：`sub2api-candidate:20260708-092542-6f00a311a-rmb-balance-affiliate`
- image id：`sha256:57ac11217ddf73283d9e7798085793010cf6a7bd8b67d431e4b87abeb52a3039`
- 旧容器：`sub2api-candidate-before-promote-20260708-092542`（保留，待业务验收后删除）

## 发布前基线

- 69 active users / 68 active keys / 52 active subscriptions
- 195 migrations（最新 `159_auto_api_key_effective_group.sql`）
- `160_rmb_balance_payment_affiliate_defaults.sql` 未应用
- 返利 settings：仅 `affiliate_enabled=true`，其他不存在
- `payment_type=balance` 订单数：0

## 发布后验证

### 容器状态

- `sub2api-candidate`：healthy，新镜像运行中
- `sub2api-candidate-postgres`：healthy，未重建
- `sub2api-candidate-redis`：healthy，未重建

### Migration

- 196 migrations（`160_rmb_balance_payment_affiliate_defaults.sql` 已应用）

### 返利默认值

| key | value |
|-----|-------|
| affiliate_rebate_rate | 8 |
| affiliate_rebate_freeze_hours | 24 |
| affiliate_rebate_duration_days | 365 |
| affiliate_rebate_per_invitee_cap | 100 |

### Health

| endpoint | status |
|----------|--------|
| 18084/health | 200 |
| 8080/health | 200 |
| api.aaccx.pw/health | 200 |
| aaccx.pw/dashboard | 200 |
| aaccx.pw/purchase | 200 |

### Smoke test

- `/v1/models` 未鉴权：403（受控错误）
- `/v1/responses` 未鉴权：403（受控错误）
- `/purchase` 返回新前端资源 `app-index-CrRjitF5.js`
- 日志无 migration failed / checksum mismatch / panic / 端口冲突

## 未执行项

- 未执行真实余额支付人工验收（需用户浏览器登录测试账号）
- 未修改 nginx / Cloudflare Tunnel
- 未重建 Postgres / Redis
- 未推送 personal / origin

## 备份文件

- Postgres dump：`deploy/backups/20260708-092542-sub2api-candidate-postgres-before-rmb-balance-affiliate.dump`（36MB，权限 600）
- Redis RDB：`deploy/backups/20260708-092542-sub2api-candidate-redis-before-rmb-balance-affiliate.rdb`（130KB，权限 600）

## 回滚信息

旧容器 `sub2api-candidate-before-promote-20260708-092542` 保留在 Docker 中，如需应用层回滚：

```bash
docker stop sub2api-candidate && docker rm sub2api-candidate && docker rename sub2api-candidate-before-promote-20260708-092542 sub2api-candidate && docker start sub2api-candidate
```
