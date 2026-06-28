# 2026-06-28 合并邀请返利分支与 main 分批提交计划

## 目标

- 将 worktree 分支 `codex/enable-affiliate-hide-register-promo` 的改动合并进本地 `main`。
- 将当前 `main` 工作区已有未提交内容按语义分批提交，避免混合运行态文档、79 元套餐修复和邀请返利功能。

## 当前事实

- 主工作区：`/Users/wujianxiang/CodeSpace/sub2api`，分支 `main`，当前状态为本地 ahead 60、behind 23。
- `main` 未提交内容包含：
  - 18080 main-preview 蓝绿测试与 18084 数据库克隆相关上下文文档，以及 `AGENTS.md` 长期记忆更新。
  - 79 元订阅池基础价修复：迁移、迁移 checksum 兼容、后端迁移测试、前端展示和下单测试、管理员套餐页测试。
- worktree 分支路径：`/Users/wujianxiang/CodeSpace/sub2api/.worktrees/codex-enable-affiliate-hide-register-promo`。
- worktree 分支未提交内容包含：
  - 注册页隐藏手填优惠码入口。
  - 后端邀请返利默认开启。
  - 管理端和前端 store 默认开启邀请返利。
  - 新增迁移和测试。
- 发现迁移号冲突：`main` 已有未提交的 `157_fix_codex_79_subscription_plan_base_price.sql`，worktree 分支也新增了 `157_enable_affiliate_default.sql`。

## 提交与合并策略

1. 先提交 `main` 里已有的 79 元套餐基础价修复，让 `157_fix_codex_79_subscription_plan_base_price.sql` 成为主线迁移。
2. 再提交 `main` 里已有的运行态上下文文档和 `AGENTS.md` 更新。
3. 在 worktree 分支中把邀请返利迁移顺延为 `158_enable_affiliate_default.sql`，并同步更新迁移回归测试。
4. 在 worktree 分支提交“隐藏注册页优惠码入口并默认开启邀请返利”。
5. 回到 `main` 合并 worktree 分支，处理可能的测试或文档冲突。
6. 合并后运行最小但覆盖关键路径的验证：
   - `git diff --check`
   - `go test -count=1 -tags=unit ./internal/service`
   - `go test -count=1 -tags=unit ./internal/repository`
   - `go test -count=1 ./migrations`
   - `npm test -- --run src/views/auth/__tests__/RegisterView.spec.ts src/stores/__tests__/app.spec.ts src/views/admin/__tests__/SettingsView.spec.ts src/components/payment/__tests__/SubscriptionPlanCard.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/admin/orders/__tests__/AdminPaymentPlansView.spec.ts`

## 风险与边界

- 不拉取、不重置、不变基远端 `origin/main`，因为当前用户要求处理的是本地 `main`。
- 不使用 `git add .`，只按文件清单暂存。
- 不提交 `deploy/backups/` 中的 dump 或任何运行态敏感文件。
- 不改写、删除历史 `docs/ai/context/` 文档，只新增本计划文档并提交已有未跟踪文档。
