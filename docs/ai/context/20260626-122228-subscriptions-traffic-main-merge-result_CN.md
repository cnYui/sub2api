# 订阅页流量包展示改动合入本地 main 记录

- 时间：2026-06-26 12:22 JST
- 目标：将 `codex/subscriptions-traffic-unified-20260626` worktree 中“订阅页只读展示 + GPT 流量包复用订阅用量卡片”的未提交修改合入本地 `main`。
- 处理方式：该 worktree 的提交历史包含 `origin/main` 侧提交，直接 merge 会把远端差异一起带入本地 `main`；因此只提取未提交 patch，基于当前本地 `main` 新建 `codex/subscriptions-traffic-unified-main-20260626` 承载分支后应用。
- 冲突处理：`frontend/src/views/user/__tests__/PaymentView.spec.ts` 的测试 mock 字段名发生冲突，保留本地 `main` 的 `activeSubscriptionsState.items` 结构，并保留订阅页职责拆分新增测试。
- 验证：
  - `npm run test:run -- src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/SubscriptionsView.spec.ts src/views/user/__tests__/paymentWechatResume.spec.ts src/components/payment/__tests__/TrafficPackCard.spec.ts src/components/payment/__tests__/SubscriptionPlanCard.spec.ts src/__tests__/visualThemeSource.spec.ts`
  - `npm run typecheck`
  - `git diff --check`
  - `rg "购买|renewNow|router\\.push|useRouter|TrafficPackCard|trafficPacks|price" frontend/src/views/user/SubscriptionsView.vue -n || true`
