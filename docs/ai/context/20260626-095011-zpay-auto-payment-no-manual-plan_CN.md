# ZPay Auto Payment No Manual Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 下线购买页全部人工支付入口，让订阅、余额充值和 GPT 流量包都只能通过 ZPay/EasyPay 动态订单二维码或跳转链接自动支付并自动履约。

**Architecture:** 前端 `PaymentView.vue` 删除 `ManualPaymentDialog` 状态机，无可用支付方式时禁用购买动作；有支付方式时继续走现有 `/api/v1/payment/orders -> PaymentStatusPanel -> verify/webhook` 链路。后端保留现有 EasyPay/ZPay provider 和金额校验逻辑，只补金额匹配/不匹配的订阅履约回归测试。

**Tech Stack:** Vue 3、Vitest、TypeScript、Go、Ent、EasyPay/ZPay 兼容支付接口。

---

## 前置步骤

- [ ] **Step 1: 基于本地 `main` 新开实现分支**

```bash
git switch main
git status --short --branch
git switch -c codex/zpay-auto-payment-no-manual-20260626
```

Expected:

- 当前分支从 `main` 开出。
- `backend/migrations/154_seed_codex_99_subscription_plan.sql` 存在。
- 工作区除 `docs/ai/context` 忽略文档外干净。

- [ ] **Step 2: 确认四档套餐和 ZPay `qr_image_url` 都在实现基线中**

```bash
test -f backend/migrations/154_seed_codex_99_subscription_plan.sql
rg -n "QRImageURL|qr_image_url|Img" backend/internal/payment backend/internal/service frontend/src
git show --stat --oneline -1
```

Expected:

- `test -f` 退出码为 0。
- `rg` 能看到 EasyPay `img`、后端响应 `qr_image_url`、前端 `qrImageUrl`。

## Task 1: 前端失败测试，锁定“无人工支付”和“29/39 动态订单”

**Files:**

- Modify: `frontend/src/views/user/__tests__/PaymentView.spec.ts`

- [ ] **Step 1: 修改 fixture，增加四档 ZPay 可支付套餐**

在 `checkoutInfoWithManualPlansFixture()` 后加入：

```ts
function checkoutInfoWithFourZPayPlansFixture() {
  return {
    data: {
      ...checkoutInfoWithPlansFixture().data,
      methods: {
        alipay: {
          currency: 'CNY',
          daily_limit: 0,
          daily_used: 0,
          daily_remaining: 0,
          single_min: 0,
          single_max: 0,
          fee_rate: 0,
          available: true,
        },
      },
      plans: [
        {
          ...checkoutInfoWithPlansFixture().data.plans[0],
          id: 1,
          group_id: 2,
          name: '29 元订阅池',
          price: 29,
          daily_limit_usd: 19,
        },
        {
          ...checkoutInfoWithPlansFixture().data.plans[0],
          id: 2,
          group_id: 3,
          name: '39 元订阅池',
          price: 39,
          daily_limit_usd: 29,
        },
        {
          ...checkoutInfoWithPlansFixture().data.plans[0],
          id: 3,
          group_id: 4,
          name: '59 元订阅池',
          price: 59,
          daily_limit_usd: 49,
        },
        {
          ...checkoutInfoWithPlansFixture().data.plans[0],
          id: 4,
          group_id: 6,
          name: '99 元订阅池',
          price: 99,
          daily_limit_usd: 89,
          group_name: 'codex-pool-89-usd',
          sort_order: 99,
        },
      ],
    },
  }
}
```

- [ ] **Step 2: 添加 29/39 元套餐自动订单测试**

在 `describe('PaymentView tab defaults', ...)` 中加入：

```ts
it.each([
  { index: 0, planId: 1, amount: 29, name: '29 元订阅池' },
  { index: 1, planId: 2, amount: 39, name: '39 元订阅池' },
])('creates a ZPay dynamic subscription order for $name', async ({ index, planId, amount }) => {
  getCheckoutInfo.mockResolvedValue(checkoutInfoWithFourZPayPlansFixture())
  createOrder.mockResolvedValue({
    order_id: 8800 + planId,
    amount,
    pay_amount: amount,
    fee_rate: 0,
    expires_at: '2099-01-01T00:10:00.000Z',
    payment_type: 'alipay',
    qr_image_url: `https://zpayz.cn/qrcode/${planId}.jpg`,
    out_trade_no: `sub2_plan_${planId}`,
  })

  const wrapper = shallowMount(PaymentView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Teleport: true,
        Transition: false,
        SubscriptionPlanCard: {
          name: 'SubscriptionPlanCard',
          props: ['plan'],
          template: '<button data-testid="subscription-plan-card" @click="$emit(\'select\', plan)">{{ plan.name }}</button>',
        },
        PaymentStatusPanel: {
          name: 'PaymentStatusPanel',
          props: ['orderId', 'qrImageUrl', 'orderType'],
          template: '<div data-testid="payment-status-panel">{{ orderId }} {{ qrImageUrl }} {{ orderType }}</div>',
        },
      },
    },
  })
  await flushPromises()
  await flushPromises()

  const planCards = wrapper.findAll('[data-testid="subscription-plan-card"]')
  expect(planCards).toHaveLength(4)
  expect(planCards[0].element.parentElement?.className).toContain('lg:grid-cols-4')

  await planCards[index].trigger('click')
  await flushPromises()

  const confirmButton = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))
  expect(confirmButton?.attributes('disabled')).toBeUndefined()
  await confirmButton?.trigger('click')
  await flushPromises()

  expect(createOrder).toHaveBeenCalledWith(expect.objectContaining({
    amount,
    payment_type: 'alipay',
    order_type: 'subscription',
    plan_id: planId,
    is_mobile: true,
  }))
  expect(wrapper.find('[data-testid="payment-status-panel"]').text()).toContain(`https://zpayz.cn/qrcode/${planId}.jpg`)
  expect(wrapper.find('[data-testid="manual-payment-dialog-stub"]').exists()).toBe(false)
})
```

- [ ] **Step 3: 改写无支付方式测试**

把 `describe('PaymentView manual subscription payment', ...)` 改名为：

```ts
describe('PaymentView without configured payment methods', () => {
```

把“opens manual payment dialog without creating an order when no payment methods are configured”测试替换为：

```ts
it('disables subscription confirmation instead of opening manual payment when no payment methods are configured', async () => {
  const wrapper = shallowMount(PaymentView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Teleport: true,
        Transition: false,
      },
    },
  })
  await flushPromises()
  await flushPromises()

  const planCard = wrapper.findComponent({ name: 'SubscriptionPlanCard' })
  expect(planCard.exists()).toBe(true)
  await planCard.vm.$emit('select', checkoutInfoWithManualPlansFixture().data.plans[0])
  await flushPromises()

  const confirmButton = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))
  expect(confirmButton?.attributes('disabled')).toBeDefined()
  await confirmButton?.trigger('click')
  await flushPromises()

  expect(createOrder).not.toHaveBeenCalled()
  expect(wrapper.html()).not.toContain('manual-payment-dialog')
  expect(wrapper.text()).toContain('payment.notAvailable')
})
```

把“opens manual payment dialog for traffic packs when no payment methods are configured”测试替换为：

```ts
it('disables traffic pack confirmation instead of opening manual payment when no payment methods are configured', async () => {
  const wrapper = shallowMount(PaymentView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Teleport: true,
        Transition: false,
        TrafficPackCard: {
          name: 'TrafficPackCard',
          props: ['pack'],
          template: '<button data-testid="traffic-pack-card" @click="$emit(\'select\', pack)">{{ pack.name }}</button>',
        },
      },
    },
  })
  await flushPromises()
  await flushPromises()

  const trafficPackCards = wrapper.findAll('[data-testid="traffic-pack-card"]')
  expect(trafficPackCards).toHaveLength(3)
  await trafficPackCards[0].trigger('click')
  await flushPromises()

  const confirmButton = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))
  expect(confirmButton?.attributes('disabled')).toBeDefined()
  await confirmButton?.trigger('click')
  await flushPromises()

  expect(createOrder).not.toHaveBeenCalled()
  expect(wrapper.html()).not.toContain('manual-payment-dialog')
  expect(wrapper.text()).toContain('payment.notAvailable')
})
```

- [ ] **Step 4: 运行前端目标测试确认失败**

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts
```

Expected before implementation:

- 新的无支付方式测试失败，因为确认按钮仍可点击并打开人工支付。
- 新的 29/39 测试可能部分通过，但 `ManualPaymentDialog` 相关残留仍会导致后续静态检查失败。

## Task 2: 前端实现，移除购买页人工支付入口

**Files:**

- Modify: `frontend/src/views/user/PaymentView.vue`
- Delete: `frontend/src/components/payment/ManualPaymentDialog.vue`
- Delete: `frontend/src/components/payment/__tests__/ManualPaymentDialog.spec.ts`
- Delete: `frontend/src/assets/payment/manual-alipay.jpg`
- Delete: `frontend/src/assets/payment/manual-alipay.png`
- Delete: `frontend/src/assets/payment/manual-wxpay.jpg`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

- [ ] **Step 1: 修改 `PaymentView.vue` 模板**

删除末尾的：

```vue
<ManualPaymentDialog
  v-if="manualPaymentItem"
  :show="showManualPaymentDialog"
  :item="manualPaymentItem"
  :locale-code="localeCode"
  @close="showManualPaymentDialog = false"
  @redeem="goRedeem"
/>
```

在订阅确认态的 `PaymentMethodSelector` 后、费用卡片前加入：

```vue
<div v-if="!hasPaymentMethods" class="card p-4 text-center">
  <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.notAvailable') }}</p>
</div>
```

在流量包确认态的 `PaymentMethodSelector` 后、费用卡片前加入同样的不可用提示：

```vue
<div v-if="!hasPaymentMethods" class="card p-4 text-center">
  <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.notAvailable') }}</p>
</div>
```

- [ ] **Step 2: 修改 `PaymentView.vue` script**

删除 import：

```ts
import ManualPaymentDialog from '@/components/payment/ManualPaymentDialog.vue'
```

删除状态：

```ts
const showManualPaymentDialog = ref(false)
```

新增 computed：

```ts
const hasPaymentMethods = computed(() => enabledMethods.value.length > 0)
```

删除 `selectPlan`、`selectTrafficPack`、`backToSubscriptionList`、`selectPlanFromModal` 中所有：

```ts
showManualPaymentDialog.value = false
```

删除 `manualPaymentItem` computed：

```ts
const manualPaymentItem = computed(() => {
  if (selectedPlan.value) {
    return {
      name: selectedPlan.value.name,
      price: selectedPlan.value.price,
    }
  }
  if (selectedTrafficPack.value) {
    return {
      name: selectedTrafficPack.value.name,
      price: selectedTrafficPack.value.price,
    }
  }
  return null
})
```

把 `canSubmitSubscription` 改为：

```ts
const canSubmitSubscription = computed(() => {
  if (!selectedPlan.value || !hasPaymentMethods.value) return false
  return amountFitsMethod(selectedPlan.value.price, selectedMethod.value)
    && selectedLimit.value?.available !== false
})
```

把 `canSubmitTrafficPack` 改为：

```ts
const canSubmitTrafficPack = computed(() => {
  if (!selectedTrafficPack.value || !hasPaymentMethods.value) return false
  return amountFitsMethod(selectedTrafficPack.value.price, selectedMethod.value)
    && selectedLimit.value?.available !== false
})
```

把 `confirmSubscribe` 改为：

```ts
async function confirmSubscribe() {
  if (!selectedPlan.value || submitting.value) return
  if (!hasPaymentMethods.value) {
    appStore.showError(t('payment.notAvailable'))
    return
  }
  await createOrder(selectedPlan.value.price, 'subscription', selectedPlan.value.id)
}
```

把 `confirmTrafficPack` 改为：

```ts
async function confirmTrafficPack() {
  if (!selectedTrafficPack.value || submitting.value) return
  if (!hasPaymentMethods.value) {
    appStore.showError(t('payment.notAvailable'))
    return
  }
  await createOrder(selectedTrafficPack.value.price, 'traffic_pack', undefined, {
    trafficPackId: selectedTrafficPack.value.id,
  })
}
```

删除 `goRedeem`：

```ts
function goRedeem() {
  showManualPaymentDialog.value = false
  router.push('/redeem')
}
```

- [ ] **Step 3: 删除人工支付组件、测试和静态收款码**

使用 `apply_patch` 删除以下文件：

```text
*** Delete File: frontend/src/components/payment/ManualPaymentDialog.vue
*** Delete File: frontend/src/components/payment/__tests__/ManualPaymentDialog.spec.ts
*** Delete File: frontend/src/assets/payment/manual-alipay.jpg
*** Delete File: frontend/src/assets/payment/manual-alipay.png
*** Delete File: frontend/src/assets/payment/manual-wxpay.jpg
```

- [ ] **Step 4: 删除 `payment.manual` i18n 子树**

从 `frontend/src/i18n/locales/zh.ts` 删除：

```ts
manual: {
  title: '扫码付款',
  planLabel: '订阅套餐',
  productLabel: '支付商品',
  wxpay: '微信支付',
  alipay: '支付宝',
  wxpayQrAlt: '微信收款码',
  alipayQrAlt: '支付宝收款码',
  scanHint: '请扫码完成付款。付款完成后点击“我已完成支付”，再使用管理员发放的兑换码开通订阅。',
  complete: '我已完成支付',
  submittedTitle: '支付已提交',
  submittedHint: '页面不会自动开通订阅。请等待管理员确认到账并发放兑换码，然后前往兑换页面输入兑换码。',
  goRedeem: '前往兑换',
},
```

从 `frontend/src/i18n/locales/en.ts` 删除对应：

```ts
manual: {
  title: 'Scan to Pay',
  planLabel: 'Subscription Plan',
  productLabel: 'Product',
  wxpay: 'WeChat Pay',
  alipay: 'Alipay',
  wxpayQrAlt: 'WeChat collection QR code',
  alipayQrAlt: 'Alipay QR code',
  scanHint: 'Scan the QR code to pay. After payment, click "I have paid", then use the redeem code issued by the admin to activate your subscription.',
  complete: 'I have paid',
  submittedTitle: 'Payment Submitted',
  submittedHint: 'This page does not activate the subscription automatically. Wait for the admin to confirm the payment and issue a redeem code, then redeem it on the redeem page.',
  goRedeem: 'Go to Redeem',
},
```

只删除 `payment` 命名空间下的 `manual` 对象，保留其它普通 `manual` 文案。

- [ ] **Step 5: 运行前端测试和静态残留扫描**

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/PaymentStatusPanel.spec.ts
pnpm typecheck
cd ..
rg -n "ManualPaymentDialog|payment\\.manual|manual-alipay|manual-wxpay|showManualPaymentDialog|goRedeem" frontend/src
```

Expected:

- Vitest 通过。
- Typecheck 通过。
- `rg` 无输出。

## Task 3: 后端回归测试，确认金额不匹配不履约、金额匹配才发放

**Files:**

- Modify: `backend/internal/service/payment_fulfillment_test.go`

- [ ] **Step 1: 添加金额不匹配订阅回调测试**

在 `TestPaymentAmountToleranceForThreeDecimalCurrency` 后加入：

```go
func TestConfirmPaymentRejectsSubscriptionAmountMismatch(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("zpay-mismatch@example.com").
		SetPasswordHash("hash").
		SetUsername("zpay-mismatch-user").
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(39).
		SetPayAmount(39).
		SetFeeRate(0).
		SetRechargeCode("PAY-ZPAY-MISMATCH").
		SetOutTradeNo("sub2_zpay_mismatch").
		SetPaymentType(payment.TypeAlipay).
		SetProviderKey(payment.TypeEasyPay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(2).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(30).
		SetStatus(OrderStatusPending).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	subRepo := newSubscriptionUserSubRepoStub()
	subscriptionSvc := NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, subRepo, nil, nil, nil)
	svc := &PaymentService{
		entClient:       client,
		groupRepo:       &subscriptionGroupRepoStub{group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription}},
		subscriptionSvc: subscriptionSvc,
	}

	err = svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		TradeNo: "zpay-trade-mismatch",
		OrderID: order.OutTradeNo,
		Amount:  29,
		Status:  payment.NotificationStatusSuccess,
	}, payment.TypeEasyPay)
	require.Error(t, err)
	require.Contains(t, err.Error(), "amount mismatch")

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, reloaded.Status)
	require.Zero(t, subRepo.createCalls)
}
```

- [ ] **Step 2: 添加金额匹配订阅回调测试**

紧接着加入：

```go
func TestConfirmPaymentCompletesSubscriptionWhenAmountMatches(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	user, err := client.User.Create().
		SetEmail("zpay-match@example.com").
		SetPasswordHash("hash").
		SetUsername("zpay-match-user").
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(39).
		SetPayAmount(39).
		SetFeeRate(0).
		SetRechargeCode("PAY-ZPAY-MATCH").
		SetOutTradeNo("sub2_zpay_match").
		SetPaymentType(payment.TypeAlipay).
		SetProviderKey(payment.TypeEasyPay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(2).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(30).
		SetStatus(OrderStatusPending).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	subRepo := newSubscriptionUserSubRepoStub()
	subscriptionSvc := NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, subRepo, nil, nil, nil)
	svc := &PaymentService{
		entClient:       client,
		groupRepo:       &subscriptionGroupRepoStub{group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription}},
		subscriptionSvc: subscriptionSvc,
	}

	err = svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		TradeNo: "zpay-trade-match",
		OrderID: order.OutTradeNo,
		Amount:  39,
		Status:  payment.NotificationStatusSuccess,
	}, payment.TypeEasyPay)
	require.NoError(t, err)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.Equal(t, "zpay-trade-match", reloaded.PaymentTradeNo)
	require.Equal(t, 1, subRepo.createCalls)
}
```

- [ ] **Step 3: 运行后端目标测试**

```bash
go test -tags=unit ./internal/service -run 'TestConfirmPaymentRejectsSubscriptionAmountMismatch|TestConfirmPaymentCompletesSubscriptionWhenAmountMatches|TestPaymentAmountToleranceForThreeDecimalCurrency'
```

Expected:

- 新增测试应通过。如果金额匹配测试失败，优先修测试夹具里的 provider key、group repo、subscription repo 初始化，不改业务逻辑绕过金额校验。

## Task 4: EasyPay/ZPay 固定金额请求回归

**Files:**

- Modify: `backend/internal/payment/provider/easypay_create_test.go`

- [ ] **Step 1: 加强 `money` 字段断言**

确认 `TestEasyPayCreateAPIPaymentReturnsQRCodeImageURL` 已断言：

```go
"money": "1.00",
```

若要更贴近套餐金额，把测试请求金额从 `"1.00"` 改为 `"29.00"`，并同步断言：

```go
Amount: "29.00",
```

```go
"money": "29.00",
```

- [ ] **Step 2: 运行 provider 测试**

```bash
go test ./internal/payment/provider
```

Expected:

- EasyPay provider 测试通过。
- 测试请求体中 `money` 等于本地订单传入金额。

## Task 5: 全量检查和上下文更新

**Files:**

- Modify: `AGENTS.md`
- Create: `docs/ai/context/20260626-095011-zpay-auto-payment-no-manual-result_CN.md`

- [ ] **Step 1: 运行组合验证**

```bash
go test ./internal/payment/provider
go test -tags=unit ./internal/service -run 'TestConfirmPaymentRejectsSubscriptionAmountMismatch|TestConfirmPaymentCompletesSubscriptionWhenAmountMatches|TestPaymentAmountToleranceForThreeDecimalCurrency'
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/PaymentStatusPanel.spec.ts
pnpm typecheck
cd ..
git diff --check
rg -n "ManualPaymentDialog|payment\\.manual|manual-alipay|manual-wxpay|showManualPaymentDialog|goRedeem" frontend/src
```

Expected:

- 所有测试通过。
- `git diff --check` 无输出。
- `rg` 无输出。

- [ ] **Step 2: 更新 `AGENTS.md` 长期记忆**

在“当前运行态提醒”或“最高优先级定论”附近补充一条：

```markdown
- 订阅、余额充值和 GPT 流量包购买全部走 Sub2API 支付订单与 ZPay/EasyPay 动态二维码；购买页人工静态收款码和人工发码链路已下线。自动履约必须以签名正确、订单号匹配、provider/merchant metadata 匹配、回调或查单金额等于本地 `pay_amount` 为准。
```

- [ ] **Step 3: 新建结果文档**

新建 `docs/ai/context/20260626-095011-zpay-auto-payment-no-manual-result_CN.md`，内容包含：

```markdown
# ZPay 动态订单自动支付与人工支付下线结果

## 完成内容

- 购买页已移除 `ManualPaymentDialog` 人工支付入口。
- 订阅、余额充值、GPT 流量包都不再展示静态收款码。
- 无可用支付方式时，购买确认按钮禁用并展示不可用提示。
- 29 / 39 / 59 / 99 元套餐继续通过 `plan_id` 创建订阅订单。
- ZPay/EasyPay 动态二维码继续使用 `qr_code` 或 `qr_image_url` 展示。
- 后端测试覆盖 ZPay/EasyPay 订阅回调金额匹配才履约、金额不匹配不履约。

## 验证

- `go test ./internal/payment/provider`
- `go test -tags=unit ./internal/service -run 'TestConfirmPaymentRejectsSubscriptionAmountMismatch|TestConfirmPaymentCompletesSubscriptionWhenAmountMatches|TestPaymentAmountToleranceForThreeDecimalCurrency'`
- `pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/PaymentStatusPanel.spec.ts`
- `pnpm typecheck`
- `git diff --check`
- `rg -n "ManualPaymentDialog|payment\\.manual|manual-alipay|manual-wxpay|showManualPaymentDialog|goRedeem" frontend/src` 无输出

## 注意

- 本次不删除 `/redeem` 页面和兑换码系统，只下线购买页人工支付入口。
- 未记录 ZPay PID、密钥或任何完整敏感凭据。
```

- [ ] **Step 4: 最终 diff 复查**

```bash
git status --short
git diff --stat
git diff -- frontend/src/views/user/PaymentView.vue frontend/src/views/user/__tests__/PaymentView.spec.ts backend/internal/service/payment_fulfillment_test.go backend/internal/payment/provider/easypay_create_test.go AGENTS.md
```

Expected:

- diff 只包含人工支付下线、自动订单测试、金额校验测试和上下文记忆。
- 不包含 ZPay PID、密钥、完整 API Key。
