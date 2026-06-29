# 2026-06-29 本地未提交内容与未合并分支并入 main 结果

## 背景

用户要求把当前本地未提交、未跟踪文件和本地未合并分支全部合并到 `main`。本轮只处理本地 Git 工作区与分支合并，没有启动、停止、重建或改写 18084 公网运行链路。

## 已完成

- 先提交了 `main` 上原有未提交/未跟踪内容：`7a9b213d8 chore: archive preview restart fixes`。
- 已将 `codex/deploy-runtime-scripts-20260624` 合并到 `main`：`3c97beb06`。
- 已将 `codex/gpt-traffic-pack-purchase-fix-20260624` 合并到 `main`：`4dac86bc7`。
- 已将 `codex/sub2api-candidate-rehearsal-20260626` 合并到 `main`：`d42c5e6dd`。
- 已将 `codex/subscriptions-traffic-unified-20260626` 合并到 `main`：`f9283b625`。
- 已将 `codex/usage-guide-99-plan-20260624` 合并到 `main`：`f73f1a9fc`；该分支包含 `codex/usage-guide-image-edit-20260624` 与 `codex/usage-guide-image-edit-20260624-localonly` 的提交，因此合并后 `git branch --no-merged main` 为空。

## 关键冲突取舍

- `AGENTS.md`：保留当前新版压缩入口，不把旧分支长运行记录重新塞回；仅补充仍有效的 Git 远端规则和 `/usage-guide` 图生图短记忆。
- `PaymentView.vue`：保留当前 `PurchaseProductCard` 统一商品卡片结构，不恢复旧 `SubscriptionPlanCard` / `TrafficPackCard` 分开渲染，也不恢复已下架的余额充值计算状态。
- `UsageGuideView.vue`：保留当前“完成支付后去 API Key 页面生成密钥”的支付直达流程，不恢复旧兑换码流程；保留 Base URL、相对路径和完整 URL 的拆分说明，并保留 Trae 接入栏目。
- `.gitignore`：同时保留 `deploy/backups/`、`!deploy/.env.candidate.local.example` 和 `deploy/candidate/`，避免 dump 进仓库，同时允许候选环境示例 env 提交。

## 验证

- `pnpm install`：通过，提示依赖已是最新。
- `go test ./migrations ./internal/repository -run 'TestMigration15(6|7)|TestIsMigrationChecksumCompatible|TestAuthIdentityPaymentMigrationsRegression' -count=1`：通过。
- `pnpm vitest run src/views/auth/__tests__/RegisterView.spec.ts src/views/user/__tests__/UsageGuideView.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/SubscriptionsView.spec.ts`：4 个文件 32 个测试通过。
- `bash deploy/redeploy-sub2api-image.test.sh`：通过。
- `bash deploy/restart-sub2api.test.sh`：通过。
- `bash deploy/test-candidate-rehearsal-scripts.sh`：通过。

## 当前状态

- `main` 上本地未合并分支列表为空。
- 合并期间未触碰公网 18084 容器、数据库、Redis、nginx 或 Cloudflare Tunnel。
