# 邀请返利线上实例生效验证

## 验证对象

- 实例：`sub2api-official-18082`
- 地址：`http://127.0.0.1:18082`
- 数据库：`sub2api-official-18082-postgres`

## 执行结果

- 使用本地源码重建应用镜像并重建 18082 应用容器，保留原 PostgreSQL 和 Redis 容器及数据卷。
- 应用启动时自动执行 `197_affiliate_auto_balance_rebate.sql`，`schema_migrations` 已记录该文件。
- 数据库设置已确认：
  - `affiliate_enabled = true`
  - `affiliate_rebate_rate = 8`
  - `affiliate_rebate_freeze_hours = 24`
  - `affiliate_auto_balance_migrated_v1 = true`
- 旧返利归集后 `user_affiliates.aff_quota` 总额为 `0`，`aff_frozen_quota` 保留冻结快照；目标库余额与返利台账查询正常。
- `GET /health` 返回 `{"status":"ok"}`；公开设置接口返回 `affiliate_enabled: true`。

## 代码验证

- 前端生产构建通过（Docker build 内执行 `pnpm run build`）。
- 后端返利履约与余额扣费定向测试通过。
- `git diff --check` 通过。

## 部署注意

- 当前 Compose 工程存在旧的 18082 依赖容器和网络；本次只替换应用容器，并将其接入原有网络，未重建或清理 PostgreSQL/Redis 数据。
