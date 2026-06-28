# 2026-06-28 合并邀请返利分支与 main 分批提交结果

## 结果

已按分批策略完成本地 `main` 收尾：

- `2f0ac55c7 fix: 修正 79 元订阅池基础价`
- `17665ab1f docs: 记录预览环境和套餐修正上下文`
- `e820a64b7 feat: 默认开启邀请返利并隐藏注册优惠码`
- `1334767a7 merge: 合并邀请返利默认开启改动`

`codex/enable-affiliate-hide-register-promo` 已通过 merge commit 合并进本地 `main`。

## 邀请返利与注册页行为

- 注册页保留后端 `promo_code` 能力，但前端手填优惠码入口固定不展示；即使公开设置 `promo_code_enabled=true`，`RegisterView` 也不会渲染优惠码输入框。
- 新用户仍可通过 URL 或 OAuth/session 传递 `aff_code`，不在注册页新增可见输入框。
- 邀请返利默认开关改为开启：
  - `AffiliateEnabledDefault=true`
  - `SettingService.IsAffiliateEnabled()` 在设置缺失时返回 true
  - 前端 app store 和管理端 settings 默认值同步为 true
  - 新增 `158_enable_affiliate_default.sql`，将既有环境的 `settings.affiliate_enabled` 写为 true

## 79 元套餐基础价

- `157_fix_codex_79_subscription_plan_base_price.sql` 保留给 79 元套餐基础价修复。
- 为避免迁移号冲突，邀请返利迁移已从原计划 `157` 顺延为 `158`。
- `79 元订阅池` 的基础价为 79 元；用户购买页继续在 1% 手续费下展示和支付 79.79 元。

## 验证

合并进 `main` 后已重新运行：

- `git diff --check` 通过。
- `go test -count=1 -tags=unit ./internal/service` 通过，耗时约 88 秒。
- `go test -count=1 -tags=unit ./internal/repository` 通过。
- `go test -count=1 ./migrations` 通过。
- `npm test -- --run src/views/auth/__tests__/RegisterView.spec.ts src/stores/__tests__/app.spec.ts src/views/admin/__tests__/SettingsView.spec.ts src/components/payment/__tests__/SubscriptionPlanCard.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/admin/orders/__tests__/AdminPaymentPlansView.spec.ts` 通过，6 个测试文件、71 个测试通过。

## 说明

本次只处理本地 git 提交与合并，未推送远端，未部署或重启运行态容器，未写入 18084 公网候选数据库。
