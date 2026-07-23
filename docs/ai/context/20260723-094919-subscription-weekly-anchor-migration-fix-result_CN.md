# 订阅周窗口锚点迁移修复结果

时间：2026-07-23

## 根因

历史迁移批次的订阅行已经正确保存统一锚点 `2026-07-22 00:00 +08`，并保留了当周 `weekly_usage_usd`。此前的续费对齐修复改为以当前权益段的原始 `starts_at` 计算周窗口；历史权益段的原始购买时刻与迁移午夜锚点不同，导致窗口起点不匹配。保护逻辑因此把已存周用量视为 0，`admin/subscriptions` 显示为空，后续计费路径也存在错误重置风险。

## 实施

- 新增 `subscription_entitlement_periods.quota_window_anchor_at`，将周刷新锚点变成每个权益段的不可变事实。
- 新增迁移 `177_subscription_entitlement_period_weekly_anchor.sql`：
  - 已有周窗口与订阅锚点一致、且锚点晚于权益段原始起点的历史记录，沿用其订阅锚点。
  - 其余记录使用权益段自身起点，确保后续订单按实际购买时刻刷新，续费权益段不继承旧锚点。
- 新订单、后台发放和后台续期均在创建周额度权益段时写入该权益段起点作为锚点。
- 管理端/用户端投影、Dashboard 聚合、Redis 预结算和 PostgreSQL 结算统一优先读取权益段锚点。
- 旧 Redis 订阅缓存缺少权益段锚点时会回源数据库并重建，避免继续使用旧缓存口径。

## 本地运行库结果

- 变更前已备份 PostgreSQL 和 Redis：
  - PostgreSQL：`20260723-094143-sub2api-postgres-dev-pre-weekly-anchor-fix.dump`，SHA-256 `5A0C5C786029D0BBD325E799AC5EDA816FE819D428B0041D4A98A61EB36803D1`。
  - Redis：`20260723-094143-sub2api-redis-dev-pre-weekly-anchor-fix.rdb`，SHA-256 `23FC0A8BC6B2FF8D1BECB8ADBD9C2BEF40F0B7B10A9599D29D828F0F6536EFB7`。
- 已先在隔离 PostgreSQL 18 恢复库实际执行迁移：110 条周权益段全部写入锚点，其中 101 条历史权益段保留统一午夜锚点。
- 新镜像已在本地 `sub2api-dev` 运行，运行库迁移后同样得到：周权益段 110、缺失锚点 0、历史午夜锚点 101、当前窗口与持久化窗口错位 0。
- 抽样核对显示历史用户已用周额度与剩余额度可保留并按统一午夜窗口展示；后续订单仍使用各自购买时刻作为锚点。

## 验证

- 通过：`go test ./...`。
- 通过：历史迁移锚点保留周用量的定向单元测试。
- 通过：隔离完整备份恢复后执行 SQL 迁移的实际验证。
- 通过：Docker 无缓存构建，前端 `vue-tsc` 与 Vite production build 完成。
- 通过：新 `sub2api-dev` 容器健康，`http://127.0.0.1:18080/health` 和 `http://127.0.0.1:8080/health` 均返回 200；启动后未见迁移或数据库错误。
