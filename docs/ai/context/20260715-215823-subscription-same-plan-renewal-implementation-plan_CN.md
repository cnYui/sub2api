# 同套餐续费与跨套餐购买拦截实施计划

> **执行要求：** 按任务顺序执行；每个任务先写失败测试，再做最小实现并复验。

**目标：** 用户可在当前套餐到期前购买同一 `group_id` 续期；购买不同 `group_id` 时在创建订单和扣余额之前被拒绝，并明确提示先退款。

**架构：** 购买资格由后端以目标套餐的 `group_id` 决定，外部支付与余额支付共用同一个校验函数；支付履约继续复用既有 `AssignOrExtendSubscription` 延长原订阅。前端仅按同一规则改善交互，后端仍是最终边界。退款服务、退款状态机和退款金额不改动。

**技术栈：** Go、Ent、PostgreSQL、Vue 3、Pinia、Vitest、Go testing。

---

### 任务 1：后端定义按套餐组的购买资格

**文件：**

- 修改：`backend/internal/service/payment_order.go:23-185`
- 修改：`backend/internal/service/payment_balance_pay_test.go:18-102`
- 修改：`backend/internal/service/payment_fulfillment_test.go`

- [ ] **步骤 1：编写失败测试，覆盖同套餐续费与跨套餐拒绝**

将现有“任意 active 订阅均拒绝”的测试拆为以下断言：

```go
func TestValidateSubOrderAllowsRenewingSameActiveSubscription(t *testing.T) {
    // 创建 group_id=7 的订阅商品和一条仍有效的 group_id=7 订阅。
    // 调用 validateSubOrder 后要求 err 为 nil。
}

func TestValidateSubOrderRejectsActiveSubscriptionInDifferentGroup(t *testing.T) {
    // 创建 group_id=7 的订阅商品和一条仍有效的 group_id=2 订阅。
    // 要求 reason 为 ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND。
}

func TestBalancePaySubscriptionRenewsSameActiveSubscription(t *testing.T) {
    // 创建余额充足的用户、group_id=7 商品和仍有效的 group_id=7 订阅。
    // 余额支付成功后断言原订阅到期时间增加 30 天，未创建第二条订阅，订单关联原 subscription_id。
}

func TestBalancePaySubscriptionRejectsDifferentActiveSubscription(t *testing.T) {
    // 创建余额充足的用户、group_id=7 商品和仍有效的 group_id=2 订阅。
    // 断言返回 ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND，余额、订单和原订阅均不变。
}
```

- [ ] **步骤 2：运行失败测试**

运行：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestValidateSubOrderAllowsRenewingSameActiveSubscription|TestValidateSubOrderRejectsActiveSubscriptionInDifferentGroup|TestBalancePaySubscriptionRenewsSameActiveSubscription|TestBalancePaySubscriptionRejectsDifferentActiveSubscription'
```

预期：同套餐续费的资格和余额支付测试因现有全局拦截返回 `ACTIVE_SUBSCRIPTION_EXISTS` 而失败。

- [ ] **步骤 3：实现最小后端校验**

在 `payment_order.go` 增加独立错误并把目标 `plan.GroupID` 传给购买资格函数：

```go
const activeSubscriptionSwitchRequiresRefundMessage = "当前套餐仍在有效期内，如需更换套餐，请先退款后再购买"

var ErrActiveSubscriptionSwitchRequiresRefund = infraerrors.Conflict(
    "ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND",
    activeSubscriptionSwitchRequiresRefundMessage,
)

func (s *PaymentService) ensureSubscriptionPurchaseAllowed(ctx context.Context, userID, targetGroupID int64) error {
    subs, err := s.subscriptionSvc.userSubRepo.ListActiveByUserID(ctx, userID)
    if err != nil {
        return fmt.Errorf("list active subscriptions: %w", err)
    }
    for _, sub := range subs {
        if sub.GroupID != targetGroupID {
            return ErrActiveSubscriptionSwitchRequiresRefund
        }
    }
    return nil
}
```

`validateSubOrder()` 调用 `ensureSubscriptionPurchaseAllowed(ctx, req.UserID, plan.GroupID)`；删除旧的用户级 `ensureNoActiveSubscriptionForPurchase()`。

- [ ] **步骤 4：运行后端资格测试**

运行任务 1 步骤 2 的命令。

预期：四个测试通过。

- [ ] **步骤 5：提交后端资格校验**

```bash
git add backend/internal/service/payment_order.go backend/internal/service/payment_balance_pay_test.go backend/internal/service/payment_fulfillment_test.go
git commit -m "feat: allow same subscription renewal"
```

### 任务 2：前端按目标套餐组决定续费或退款提示

**文件：**

- 修改：`frontend/src/views/user/PaymentView.vue:592-608, 628-647, 784-850`
- 修改：`frontend/src/views/user/__tests__/PaymentView.spec.ts:528-603`
- 修改：`frontend/src/views/user/__tests__/paymentUx.spec.ts`
- 修改：`frontend/src/i18n/locales/zh.ts`
- 修改：`frontend/src/i18n/locales/en.ts`

- [ ] **步骤 1：编写失败前端测试**

新增或改写测试，覆盖同组与跨组：

```ts
it('allows selecting the current subscription group for renewal', async () => {
  activeSubscriptionsState.items = [{ id: 42, group_id: 2, status: 'active', expires_at: '2099-01-01T00:00:00Z' }]
  // 点击 group_id=2 的卡片后，支付方法可见，且不显示错误。
})

it('blocks selecting another subscription group until refund', async () => {
  activeSubscriptionsState.items = [{ id: 42, group_id: 2, status: 'active', expires_at: '2099-01-01T00:00:00Z' }]
  // 点击 group_id=7 的卡片后，显示 ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND，支付方法不可见。
})
```

- [ ] **步骤 2：运行前端失败测试**

运行：

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/paymentUx.spec.ts
```

预期：同组续费仍被现有全局 `hasActiveSubscriptionForPurchase` 拦截而失败。

- [ ] **步骤 3：实现目标套餐组判断与文案**

将全局阻断函数替换为接收目标套餐的函数：

```ts
async function refreshAndBlockDifferentActiveSubscription(plan: SubscriptionPlan): Promise<boolean> {
  try {
    await subscriptionStore.fetchActiveSubscriptions(true)
  } catch {
    // 网络失败时保留当前缓存，后端仍会再次校验。
  }
  const hasDifferentActiveSubscription = activeSubscriptions.value.some(
    subscription => subscription.status === 'active' && subscription.group_id !== plan.group_id,
  )
  if (!hasDifferentActiveSubscription) return false
  appStore.showError(t('payment.errors.ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND'))
  return true
}
```

`selectPlan()`、`selectPlanFromModal()`、`confirmSubscribe()` 都传入 `selectedPlan` 并复用该函数。卡片的 `active` 保持以同 `group_id` 判断，因此同套餐继续显示“续费”。增加中英文 `ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND` 翻译键。

- [ ] **步骤 4：运行前端目标测试**

运行任务 2 步骤 2 的命令。

预期：全部通过。

- [ ] **步骤 5：提交前端交互**

```bash
git add frontend/src/views/user/PaymentView.vue frontend/src/views/user/__tests__/PaymentView.spec.ts frontend/src/views/user/__tests__/paymentUx.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: distinguish renewal from subscription switch"
```

### 任务 3：完整验证与结果记录

**文件：**

- 新建：`docs/ai/context/20260715-215823-subscription-same-plan-renewal-result_CN.md`
- 修改：`AGENTS.md`

- [ ] **步骤 1：运行后端回归**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
```

预期：通过。

- [ ] **步骤 2：运行前端回归与构建**

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/paymentUx.spec.ts
pnpm typecheck
pnpm build
```

预期：通过。

- [ ] **步骤 3：检查差异**

```bash
git diff --check
git status --short
```

预期：无空白错误；仅包含本功能代码、测试和上下文文档。

- [ ] **步骤 4：记录结果与项目记忆**

新建结果文档，记录分支、行为变化、验证命令及结果、未部署和未修改退款逻辑的事实；在 `AGENTS.md` 最高优先级定论新增同等摘要。不得修改历史上下文文档。

- [ ] **步骤 5：提交结果文档与项目记忆**

```bash
git add AGENTS.md docs/ai/context/20260715-215823-subscription-same-plan-renewal-result_CN.md
git commit -m "docs: 记录同套餐续费结果"
```
