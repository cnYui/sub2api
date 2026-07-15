# 同套餐续费与跨套餐购买拦截实施结果

## 背景

用户确认续费和退款必须分开处理：

- 无有效订阅时，可以购买任意套餐。
- 有有效订阅时，只允许购买相同 `group_id` 作为续费。
- 购买不同 `group_id` 时直接拒绝，提示先退款后再购买。
- 本次不自动退款、不自动切换套餐、不做补差，也不改退款状态机。

## 分支

- 工作区：`/Users/wujianxiang/CodeSpace/sub2api/.worktrees/codex-subscription-same-plan-renewal`
- 分支：`codex/subscription-same-plan-renewal`
- 基线：本地 `main@ba3a90f6d`

## 实现

### 后端

- `PaymentService.validateSubOrder()` 不再用用户级“任意 active 订阅”拦截。
- 新增 `ensureSubscriptionPurchaseAllowed(ctx, userID, targetGroupID)`：
  - active 订阅 `group_id` 与目标套餐一致时放行。
  - active 订阅 `group_id` 与目标套餐不一致时返回 `ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND`。
- 外部支付和余额支付继续复用同一个订阅下单校验。
- 余额支付同套餐续费会复用既有 `AssignOrExtendSubscription()`，从原订阅 `expires_at` 继续增加有效期，并把订单关联到原 `subscription_id`。
- 跨套餐余额支付在扣余额和创建订单前被拒绝。

### 前端

- `/purchase` 的订阅选择、续费弹窗选择、最终确认支付都改为按目标套餐 `group_id` 判断。
- 同 `group_id` 的套餐卡片仍显示“续费”，点击后允许进入支付方式选择。
- 不同 `group_id` 的套餐卡片显示 `当前套餐仍在有效期内，如需更换套餐，请先退款后再购买`，不打开支付方式。
- 中文和英文都新增 `ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND` 文案；旧 `ACTIVE_SUBSCRIPTION_EXISTS` 文案也改为相同语义，避免旧错误码继续显示“联系管理员退款”。

## 验证

已按 TDD 先看到失败，再实现通过：

- 后端 RED：同套餐续费和跨套餐新错误码目标用例先失败，旧逻辑返回 `ACTIVE_SUBSCRIPTION_EXISTS`。
- 前端 RED：同组续费被旧全局 active 判断拦住，新文案键缺失。

完整验证：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server

cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/paymentUx.spec.ts
pnpm typecheck
pnpm build

git diff --check
git status --short
```

结果：

- `./internal/service`：通过，`0.950s` 目标 GREEN，完整包回归 `88.463s`。
- `./cmd/server`：通过，`0.766s`。
- 前端目标 Vitest：`44/44` 通过。
- `pnpm typecheck`：退出码 0。
- `pnpm build`：退出码 0；仅有既有 Vite chunk / dynamic import / Browserslist 警告。
- `git diff --check`：通过。
- `git status --short`：实现提交后干净；结果文档和 AGENTS 更新前无残留构建产物。

## 未改范围

- 未修改退款金额、退款状态机、撤权、`MANUAL_REVIEW` 保护。
- 未修改数据库 schema、Redis、Nginx、容器或运行态数据。
- 未部署。

## 提交

- `6be0b98eb feat: allow same subscription renewal`
- `068448df4 feat: distinguish renewal from subscription switch`
