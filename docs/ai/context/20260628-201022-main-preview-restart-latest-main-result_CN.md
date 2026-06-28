# 2026-06-28 main-preview 18080 重启到最新 main 结果

## 执行目标

把本地 18080 main-preview 环境更新到当前本机 `main` 最新代码，启动 Sub2API 前后端，并保持 18080 preview 数据层和 18084 公网候选栈隔离。

## 使用代码

- 工作树：`/Users/wujianxiang/CodeSpace/sub2api`
- 分支：`main`
- HEAD：`befa8f138 docs: 记录邀请返利合并结果`

本次启动前先备份 18080 preview DB：

- `deploy/backups/20260628-194157-18080-preview-before-main-restart.dump`
- 大小约 15MB
- 权限 `600`
- 位于 ignored 的 `deploy/backups/`，不要提交

## 构建与启动

第一次构建镜像：

- `sub2api-main-preview:20260628-194157-befa8f138`

该镜像启动失败，原因是 `156_seed_codex_79_subscription_plan.sql` checksum mismatch：

```text
db=35caceaf96260c67814cd35514271c21095f79bd708f60758d7104e56d0ea1b7
file=bfd204584e5b768e03ebacd4ab6ba86562f3e73d5e6b9001807c0b1c53c10b49
```

根因是提交 `2f0ac55c7 fix: 修正 79 元订阅池基础价` 已新增 `157_fix_codex_79_subscription_plan_base_price.sql`，但同时修改了已经应用过的 `156_seed_codex_79_subscription_plan.sql`，违反迁移不可变约束。正确修复是：

- 恢复 `156` 原始内容，保持原 seed 价 `79.79`。
- 保留 `157` 作为补偿迁移，将 79 套餐基础价修为 `79.00`。
- 修正 `156` checksum 兼容白名单为迁移器实际使用的 trim checksum，兼容已短暂应用过误改版 `156` 的环境。

已跑回归测试：

```text
go test ./migrations -run TestMigration156SeedsCodex79SubscriptionPlanWithoutAccountBinding -count=1
go test ./internal/repository -run 'TestIsMigrationChecksumCompatible/156误改checksum可兼容回滚后的原始79套餐seed' -count=1
go test ./migrations ./internal/repository -run 'TestMigration15(6|7)|TestIsMigrationChecksumCompatible' -count=1
```

最终构建并启动镜像：

- `sub2api-main-preview:20260628-195911-befa8f138-migrationfix`
- `sub2api-main-preview:codex-main`

## 当前状态

```text
sub2api-main-preview            sub2api-main-preview:20260628-195911-befa8f138-migrationfix   Up (healthy)   127.0.0.1:18080->8080/tcp
sub2api-main-preview-postgres   postgres:18-alpine                                            Up             5432/tcp
sub2api-main-preview-redis      redis:8-alpine                                                Up             6379/tcp
```

18084 公网候选栈保持不变：

```text
sub2api-candidate               sub2api-candidate:20260627-221441-traffic-card-fix            Up (healthy)   127.0.0.1:18084->8080/tcp
sub2api-candidate-postgres      postgres:18-alpine                                            Up (healthy)   5432/tcp
sub2api-candidate-redis         redis:8-alpine                                                Up (healthy)   6379/tcp
```

`sub2api-candidate` 启动时间仍为 `2026-06-27T13:25:13Z`，本次未停止、未重建、未写入 `sub2api-candidate*`，也未修改 nginx 或 `8080 -> 18084`。

## 验证

- `curl -fsS http://127.0.0.1:18080/health` 返回 `{"status":"ok"}`。
- `curl -fsS http://127.0.0.1:18084/health` 返回 `{"status":"ok"}`。
- `curl -fsS http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`。
- 18080 前端资源：
  - `assets/index-DU7TEMtY.js`
  - `assets/index-nffSQZgD.css`
  - `assets/pkg-i18n-CRLwLFIo.js`
  - `assets/pkg-misc-CjRx2-Hi.js`
  - `assets/pkg-vue-BqGtxt06.js`
- 18080 preview DB：
  - `users=49`
  - `api_keys=41`
  - `schema_migrations=194`
- 新应用迁移：
  - `156_seed_codex_79_subscription_plan.sql` checksum 为 `35caceaf96260c67814cd35514271c21095f79bd708f60758d7104e56d0ea1b7`
  - `157_fix_codex_79_subscription_plan_base_price.sql` 已应用
  - `158_enable_affiliate_default.sql` 已应用
- 79 套餐当前 preview DB 状态：
  - `group=codex-pool-69-usd`
  - `plan=79 元订阅池`
  - `price=79.00`
  - `for_sale=true`

## 后续提醒

当前工作树有代码修复和上下文文档改动，尚未提交。`deploy/backups/20260628-194157-18080-preview-before-main-restart.dump` 是敏感备份文件，已被 `.gitignore` 忽略，不要提交。
