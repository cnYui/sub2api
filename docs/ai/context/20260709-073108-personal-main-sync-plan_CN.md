# personal/main 同步计划

## 目标

把当前本地 `main` 上的 149/199 元订阅套餐改动提交，并同步到远端 `personal/main`。

## 当前状态

- 当前分支：`main`。
- 已执行 `git fetch personal main`。
- `git rev-list --left-right --count personal/main...main` 返回 `0 0`。
- 说明远端 `personal/main` 没有本地缺失提交，本地也还没有新提交；可以在本地提交后常规推送。

## 提交范围

- `AGENTS.md`
- `backend/migrations/161_seed_codex_149_199_subscription_plans.sql`
- `backend/migrations/auth_identity_payment_migrations_regression_test.go`
- `frontend/src/views/user/__tests__/PaymentView.spec.ts`
- `docs/ai/context/20260708-214551-codex-plan-f-198-subscription-design_CN.md`
- `docs/ai/context/20260708-214551-codex-plan-f-198-subscription-implementation-plan_CN.md`
- `docs/ai/context/20260708-215138-codex-plan-f-198-subscription-result_CN.md`
- `docs/ai/context/20260709-072356-codex-149-199-subscription-design_CN.md`
- `docs/ai/context/20260709-072356-codex-149-199-subscription-implementation-plan_CN.md`
- `docs/ai/context/20260709-072356-codex-149-199-subscription-result_CN.md`
- `docs/ai/context/20260709-073108-personal-main-sync-plan_CN.md`

## 验证

提交前重新执行：

- `cd backend && go test -count=1 ./migrations`
- `cd frontend && pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts`
- `git diff --check`

## 同步方式

- 使用普通提交，不改写历史。
- 使用 `git push personal main`。
- 不使用 force push。

## 发布边界

本次只同步 Git 远端代码，不构建镜像、不部署 18084、不修改公网 DB/nginx/Redis。公网发布后仍需单独绑定 `codex-pool-135-usd` 与 `codex-pool-179-usd` 到 `cliproxy-local-openai` 并做真实请求验收。
