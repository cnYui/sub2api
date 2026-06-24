# 2026-06-24 本地 main 与远端 main 同步计划

## 背景

- 当前需要把本地已经合并业务修复的 `main`，同步到远端最新 `main`。
- 官方远端为 `origin`，用户个人 GitHub fork 远端为 `personal`。
- 本次目标是：
  1. 把 `origin/main` 的最新提交合入本地 `main`
  2. 保留本地主线已有业务提交
  3. 验证关键前端购买链路相关测试、类型检查和构建
  4. 只把同步后的 `main` 推送到 `personal/main`

## 当前已知状态

- 本地 `main` 位于 worktree：`/Users/wujianxiang/CodeSpace/sub2api-mobile-touch-overlay-fix-20260623`
- 远端抓取后，`origin/main` 与 `personal/main` 都在 `85a3b122`
- 本地 `main` 在合并前相对 `origin/main` 为 `ahead 37, behind 119`
- 已执行 `git merge --no-ff origin/main`
- 真实冲突只有：
  - `.dockerignore`
  - `frontend/src/router/title.ts`

## 冲突解决原则

- `.dockerignore`：同时保留本地 `docs/legal` Docker 构建白名单和远端新增注释
- `frontend/src/router/title.ts`：同时保留本地品牌默认名逻辑和远端新的路由标题解析函数

## 风险边界

- 不改动 `origin/main`
- 不覆盖或回退用户现有历史提交
- 不向官方仓库 push
- 先验证，再提交 merge commit，再推送到 `personal`

## 执行步骤

1. 确认 merge 冲突已全部解决
2. 跑关键验证：
   - `pnpm test:run src/views/user/__tests__/PaymentView.spec.ts`
   - `pnpm test:run src/components/payment/__tests__/ManualPaymentDialog.spec.ts`
   - `pnpm typecheck`
   - `pnpm build`
3. 提交 merge commit
4. 推送到 `personal/main`
