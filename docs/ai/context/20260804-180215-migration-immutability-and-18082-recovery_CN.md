# 199 迁移不可变恢复与 18082 发布

## 背景

当前工作区的 `backend/migrations/199_normalize_balance_package_lifecycle.sql` 曾被追加订单快照和套餐计划同步逻辑。生产数据库已执行的 199 记录校验和为 `ebf3270d791be46096d4a92c0531da7ec5520de769ab196e65b0df211b8f8f8b`，新镜像启动时因此被迁移校验保护拒绝。

## 决策

- 将 199 恢复为生产实际执行的不可变内容，恢复后 SHA-256 与 `schema_migrations` 一致。
- 保留的订单快照、套餐字段同步逻辑迁移到新增的 `201_normalize_balance_package_plan_snapshots.sql`。
- 不修改生产数据库中的迁移校验和，不关闭全局迁移校验。
- 200 迁移继续按当前工作区顺序执行。

## 发布范围

只停止并重建 `sub2api-official-18082` 应用容器，PostgreSQL、Redis、Cloudflare Tunnel 和公网 Nginx 保持不变。启动前已禁用旧应用容器的重启循环，待新镜像启动成功后由 Compose 恢复 `unless-stopped`。

## 验证

- 199 恢复后校验和：`ebf3270d791be46096d4a92c0531da7ec5520de769ab196e65b0df211b8f8f8b`。
- 新镜像需通过前端构建、后端 `go build`，并确认应用、8080 反向代理和 `aaccx.pw` 健康检查。
- 数据库需确认 199、200、201 的迁移记录及校验和。
