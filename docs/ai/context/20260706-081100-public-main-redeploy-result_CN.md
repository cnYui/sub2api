# 公网 Sub2API main 最新代码发布结果

## 执行时间

2026-07-06 08:01 ~ 08:11 (Asia/Shanghai)

## 关键指标

| 项目 | 结果 |
|------|------|
| 发布前本地 HEAD | `9966c6f5b` (feat: finish automatic api key effective group) |
| 新镜像 | `sub2api-candidate:20260706-080023-9966c6f5b-local-main` |
| 新镜像 ID | `sha256:3273f560b6854d1a4887704c46c4131da4082f923bf1b45e0443562037d4315e` |
| 发布后 health | `18084/health=200`, `8080/health=200`, `api.aaccx.pw/health=200` |
| 执行回滚 | 否 |

## 备份文件

| 文件 | 权限 | 大小 |
|------|------|------|
| `deploy/backups/20260706-080023-sub2api-candidate-postgres-before-main-redeploy.dump` | 600 | 29MB |
| `deploy/backups/20260706-080023-sub2api-candidate-redis-before-main-redeploy.rdb` | 600 | 69KB |

## 数据库变更

### 发布前后对比

| 指标 | 发布前 | 发布后 |
|------|--------|--------|
| Active users | 60 | 60 |
| Active keys | 52 | 52 |
| Migrations | 194 (`158_enable_affiliate_default.sql`) | 195 (`159_auto_api_key_effective_group.sql`) |

### Migration 159 效果

- `traffic-pack-openai` 分组已创建：`id=10`, platform=`openai`, subscription_type=`standard`, status=`active`, is_exclusive=`true`, allow_image_generation=`true`
- 1 个未删除 OpenAI 上游账号已绑定到 `traffic-pack-openai`
- 52 个 API Keys 中，52 个 `group_id=NULL`（旧 OpenAI Key 全部迁移为自动 Key）

## Smoke Test 结果

| 端点 | 结果 |
|------|------|
| `https://aaccx.pw/dashboard` | 200 |
| `https://aaccx.pw/purchase` | 200 |
| `https://api.aaccx.pw/health` | 200 |
| 日志 checksum mismatch / migration failed / panic | 无 |

## 未修改项

- 未修改 nginx
- 未停止/重建 `sub2api-candidate-postgres` 和 `sub2api-candidate-redis`
- 未使用 18080 DB 覆盖公网 DB

## 运行态提醒

- 当前公网应用容器为 `sub2api-candidate`（新镜像）
- 新应用在公网 DB 上成功应用了 `159_auto_api_key_effective_group.sql`
- 所有用户 API Keys 已变为自动 Key（`group_id=NULL`），请求时会自动解析 effective_group
