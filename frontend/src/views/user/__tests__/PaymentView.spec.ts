import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, shallowMount } from '@vue/test-utils'
import PaymentView from '../PaymentView.vue'
import { PAYMENT_RECOVERY_STORAGE_KEY } from '@/components/payment/paymentFlow'

const routeState = vi.hoisted(() => ({
  path: '/purchase',
  query: {} as Record<string, unknown>,
}))

const routerReplace = vi.hoisted(() => vi.fn())
const routerPush = vi.hoisted(() => vi.fn())
const routerResolve = vi.hoisted(() => vi.fn(() => ({ href: '/payment/stripe?mock=1' })))
const createOrder = vi.hoisted(() => vi.fn())
const balancePayOrder = vi.hoisted(() => vi.fn())
const refreshUser = vi.hoisted(() => vi.fn())
const fetchActiveSubscriptions = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const activeSubscriptionsState = vi.hoisted(() => ({
  items: [] as Array<Record<string, unknown>>,
}))
const showError = vi.hoisted(() => vi.fn())
const showInfo = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const showWarning = vi.hoisted(() => vi.fn())
const getCheckoutInfo = vi.hoisted(() => vi.fn())
const authState = vi.hoisted(() => ({
  userBalance: 0,
}))
const bridgeInvoke = vi.hoisted(() => vi.fn())

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeState,
    useRouter: () => ({
      replace: routerReplace,
      push: routerPush,
      resolve: routerResolve,
    }),
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: {
      username: 'demo-user',
      balance: authState.userBalance,
    },
    refreshUser,
  }),
}))

vi.mock('@/stores/payment', () => ({
  usePaymentStore: () => ({
    createOrder,
  }),
}))

vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({
    activeSubscriptions: activeSubscriptionsState.items,
    fetchActiveSubscriptions,
  }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showInfo,
    showSuccess,
    showWarning,
  }),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    balancePayOrder,
    getCheckoutInfo,
  },
}))

vi.mock('@/utils/device', () => ({
  isMobileDevice: () => true,
}))

function checkoutInfoFixture() {
  return {
    data: {
      methods: {
        wxpay: {
          daily_limit: 0,
          daily_used: 0,
          daily_remaining: 0,
          single_min: 0,
          single_max: 0,
          fee_rate: 0,
          available: true,
        },
      },
      global_min: 0,
      global_max: 0,
      plans: [],
      traffic_packs: [
        {
          id: 1,
          code: 'gpt_traffic_5usd_2cny',
          name: 'GPT 流量包 5 刀',
          description: '2 元购买 5 USD GPT 额度，有效期 365 天，可用于写代码和生图。',
          price: 2,
          credit_usd: 5,
          validity_days: 365,
          platform: 'openai',
          for_sale: true,
          sort_order: 10,
        },
        {
          id: 2,
          code: 'gpt_traffic_10usd_3cny',
          name: 'GPT 流量包 10 刀',
          description: '3 元购买 10 USD GPT 额度，有效期 365 天，可用于写代码和生图。',
          price: 3,
          credit_usd: 10,
          validity_days: 365,
          platform: 'openai',
          for_sale: true,
          sort_order: 20,
        },
        {
          id: 3,
          code: 'gpt_traffic_20usd_5cny',
          name: 'GPT 流量包 20 刀',
          description: '5 元购买 20 USD GPT 额度，有效期 365 天，可用于写代码和生图。',
          price: 5,
          credit_usd: 20,
          validity_days: 365,
          platform: 'openai',
          for_sale: true,
          sort_order: 30,
        },
      ],
      traffic_credit_summary: {
        total_remaining_usd: 8,
        next_expiring_usd: 5,
        next_expires_at: '2027-06-22T12:00:00Z',
      },
      balance_disabled: false,
      balance_recharge_multiplier: 1,
      recharge_fee_rate: 0,
      help_text: '',
      help_image_url: '',
      stripe_publishable_key: '',
    },
  }
}

function checkoutInfoWithPlansFixture() {
  return {
    data: {
      ...checkoutInfoFixture().data,
      plans: [
        {
          id: 7,
          group_id: 3,
          name: 'Starter',
          description: '',
          price: 128,
          original_price: 0,
          validity_days: 30,
          validity_unit: 'day',
          rate_multiplier: 1,
          daily_limit_usd: null,
          weekly_limit_usd: null,
          monthly_limit_usd: null,
          features: [],
          group_platform: 'openai',
          sort_order: 1,
          for_sale: true,
          group_name: 'OpenAI',
        },
      ],
    },
  }
}

function checkoutInfoWithManualPlansFixture() {
  return {
    data: {
      ...checkoutInfoWithPlansFixture().data,
      methods: {},
    },
  }
}

function checkoutInfoWithFiveManualPlansFixture() {
  return {
    data: {
      ...checkoutInfoWithManualPlansFixture().data,
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
          id: 5,
          group_id: 5,
          name: '79 元订阅池',
          price: 79,
          daily_limit_usd: 69,
          group_name: 'codex-pool-69-usd',
          sort_order: 79,
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

function checkoutInfoWithFiveZPayPlansFixture() {
  return {
    data: {
      ...checkoutInfoWithFiveManualPlansFixture().data,
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
    },
  }
}

function jsapiOrderFixture(resumeToken: string) {
  return {
    order_id: 123,
    amount: 88,
    pay_amount: 88,
    fee_rate: 0,
    expires_at: '2099-01-01T00:10:00.000Z',
    payment_type: 'wxpay',
    out_trade_no: 'sub2_jsapi_123',
    result_type: 'jsapi_ready' as const,
    resume_token: resumeToken,
    jsapi: {
      appId: 'wx123',
      timeStamp: '1712345678',
      nonceStr: 'nonce',
      package: 'prepay_id=wx123',
      signType: 'RSA',
      paySign: 'signed',
    },
  }
}

const purchaseProductCardStub = {
  name: 'PurchaseProductCard',
  props: ['product'],
  template: `
    <button data-testid="purchase-product-card" @click="$emit('select', product)">
      {{ product.title }} {{ product.priceText }}
      <span v-for="row in product.detailRows" :key="row.label">{{ row.label }}{{ row.value }}</span>
    </button>
  `,
}

const paymentMethodSelectorStub = {
  name: 'PaymentMethodSelector',
  props: ['methods', 'selected'],
  emits: ['select'],
  template: `
    <div>
      <button
        v-for="method in methods"
        :key="method.type"
        :data-testid="'payment-method-' + method.type"
        @click="$emit('select', method.type)"
      >
        payment.methods.{{ method.type }}
      </button>
    </div>
  `,
}

function oauthOrderFixture() {
  return {
    order_id: 456,
    amount: 128,
    pay_amount: 128,
    fee_rate: 0,
    expires_at: '2099-01-01T00:10:00.000Z',
    payment_type: 'wxpay',
    result_type: 'oauth_required' as const,
    oauth: {
      authorize_url: '/api/v1/auth/oauth/wechat/payment/start?payment_type=wxpay&redirect=%2Fpurchase%3Ffrom%3Dwechat',
      appid: 'wx123',
      scope: 'snsapi_base',
      redirect_url: '/auth/wechat/payment/callback',
    },
  }
}

describe('PaymentView tab defaults', () => {
  beforeEach(() => {
    routeState.path = '/purchase'
    routeState.query = {}
    routerReplace.mockReset().mockResolvedValue(undefined)
    routerPush.mockReset().mockResolvedValue(undefined)
    routerResolve.mockClear()
    createOrder.mockReset()
    balancePayOrder.mockReset().mockResolvedValue({
      data: {
        order_id: 9001,
        amount: 29,
        pay_amount: 29,
        fee_rate: 0,
        status: 'COMPLETED',
        payment_type: 'balance',
        order_type: 'subscription',
        out_trade_no: 'sub2_balance_test',
      },
    })
    refreshUser.mockReset()
    fetchActiveSubscriptions.mockReset().mockResolvedValue(undefined)
    activeSubscriptionsState.items = []
    showError.mockReset()
    showInfo.mockReset()
    showSuccess.mockReset()
    showWarning.mockReset()
    getCheckoutInfo.mockReset().mockResolvedValue(checkoutInfoWithPlansFixture())
    authState.userBalance = 0
    bridgeInvoke.mockReset()
    window.localStorage.clear()
  })

  it('renders only subscription purchases and hides the balance recharge page', async () => {
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    const tabTexts = wrapper
      .findAll('button')
      .map(button => button.text())
      .filter(text => text === 'payment.tabSubscribe' || text === 'payment.tabTopUp')

    expect(tabTexts).toEqual([])
    expect(wrapper.find('[data-testid="purchase-product-card"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('payment.rechargeAccount')
    expect(wrapper.text()).not.toContain('payment.currentBalance')
    expect(wrapper.text()).not.toContain('payment.paymentAmount')
    expect(wrapper.findComponent({ name: 'AmountInput' }).exists()).toBe(false)
  })

  it('shows balance recharge as the first purchase product', async () => {
    getCheckoutInfo.mockResolvedValueOnce(checkoutInfoWithFiveZPayPlansFixture())

    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    const cards = wrapper.findAll('[data-testid="purchase-product-card"]')
    expect(cards[0].text()).toContain('payment.recharge.title')
    expect(cards[0].text()).toContain('¥1 起')
  })

  it('opens recharge confirm with default amount 1 and alipay only', async () => {
    getCheckoutInfo.mockResolvedValueOnce(checkoutInfoWithFiveZPayPlansFixture())

    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
          PaymentMethodSelector: paymentMethodSelectorStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()
    await wrapper.findAll('[data-testid="purchase-product-card"]')[0].trigger('click')

    expect(wrapper.text()).toContain('payment.recharge.title')
    expect((wrapper.get('[data-testid="balance-recharge-amount"]').element as HTMLInputElement).value).toBe('1')
    expect(wrapper.text()).toContain('payment.methods.alipay')
    expect(wrapper.text()).not.toContain('payment.methods.wxpay')
    expect(wrapper.text()).not.toContain('payment.methods.stripe')
  })

  it('shows alipay and balance only for product checkout', async () => {
    getCheckoutInfo.mockResolvedValueOnce(checkoutInfoWithFiveZPayPlansFixture())

    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
          PaymentMethodSelector: paymentMethodSelectorStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()
    await wrapper.findAll('[data-testid="purchase-product-card"]')[1].trigger('click')

    expect(wrapper.text()).toContain('payment.methods.alipay')
    expect(wrapper.text()).toContain('payment.methods.balance')
    expect(wrapper.text()).not.toContain('payment.methods.wxpay')
    expect(wrapper.text()).not.toContain('payment.methods.stripe')
    expect(wrapper.text()).not.toContain('payment.methods.airwallex')
  })

  it('opens recharge confirm with rounded shortage when balance is insufficient', async () => {
    getCheckoutInfo.mockResolvedValueOnce({
      data: {
        ...checkoutInfoWithFiveZPayPlansFixture().data,
        recharge_fee_rate: 1,
      },
    })
    authState.userBalance = 10
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
          PaymentMethodSelector: paymentMethodSelectorStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()
    await wrapper.findAll('[data-testid="purchase-product-card"]')[1].trigger('click')
    await wrapper.find('[data-testid="payment-method-balance"]').trigger('click')
    await wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))?.trigger('click')

    expect(wrapper.text()).toContain('payment.recharge.title')
    expect((wrapper.get('[data-testid="balance-recharge-amount"]').element as HTMLInputElement).value).toBe('20')
  })

  it('calls balance pay api when balance is sufficient', async () => {
    getCheckoutInfo.mockResolvedValueOnce(checkoutInfoWithFiveZPayPlansFixture())
    authState.userBalance = 100

    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
          PaymentMethodSelector: paymentMethodSelectorStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()
    await wrapper.findAll('[data-testid="purchase-product-card"]')[1].trigger('click')
    await wrapper.find('[data-testid="payment-method-balance"]').trigger('click')
    await wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))?.trigger('click')
    await flushPromises()

    expect(balancePayOrder).toHaveBeenCalledWith({
      order_type: 'subscription',
      plan_id: 1,
    })
    expect(createOrder).not.toHaveBeenCalled()
  })

  it.each(['0', '1.5', '101', ''])('rejects invalid recharge amount %s', async (value) => {
    getCheckoutInfo.mockResolvedValueOnce(checkoutInfoWithFiveZPayPlansFixture())

    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
          PaymentMethodSelector: paymentMethodSelectorStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()
    await wrapper.findAll('[data-testid="purchase-product-card"]')[0].trigger('click')
    await wrapper.get('[data-testid="balance-recharge-amount"]').setValue(value)

    expect(wrapper.get('[data-testid="balance-recharge-submit"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('payment.recharge.invalidAmount')
  })

  it('does not hide purchasable items inside a native template element', async () => {
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<main><slot /></main>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(wrapper.find('main > div > template').exists()).toBe(false)
  })

  it('ignores the legacy recharge tab query and stays on subscription purchases', async () => {
    routeState.query = { tab: 'recharge' }
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(wrapper.find('[data-testid="purchase-product-card"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('payment.rechargeAccount')
    expect(wrapper.findComponent({ name: 'AmountInput' }).exists()).toBe(false)
  })

  it('renders GPT traffic pack cards and creates a traffic pack order', async () => {
    getCheckoutInfo.mockResolvedValueOnce({
      data: {
        ...checkoutInfoWithPlansFixture().data,
        methods: checkoutInfoWithFiveZPayPlansFixture().data.methods,
      },
    })
    createOrder.mockResolvedValue({
      order_id: 991,
      amount: 5,
      pay_amount: 5,
      fee_rate: 0,
      expires_at: '2099-01-01T00:10:00.000Z',
      payment_type: 'alipay',
      qr_code: 'weixin://wxpay/bizpayurl?pr=traffic-pack',
      out_trade_no: 'sub2_traffic_pack_991',
    })
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('5刀流量卡')
    expect(wrapper.text()).toContain('10刀流量卡')
    expect(wrapper.text()).toContain('20刀流量卡')
    expect(wrapper.text()).not.toContain('一次性流量包-有效期')
    expect(wrapper.text()).toContain('刷新时间365天')

    const purchaseCards = wrapper.findAll('[data-testid="purchase-product-card"]')
    expect(purchaseCards).toHaveLength(5)
    await purchaseCards[4].trigger('click')
    await flushPromises()

    const confirmButton = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))
    expect(confirmButton?.attributes('disabled')).toBeUndefined()
    await confirmButton?.trigger('click')
    await flushPromises()

    expect(createOrder).toHaveBeenCalledWith(expect.objectContaining({
      amount: 5,
      payment_type: 'alipay',
      order_type: 'traffic_pack',
      traffic_pack_id: 3,
      is_mobile: true,
    }))
  })

  it('passes the checkout fee rate to the unified purchase cards', async () => {
    getCheckoutInfo.mockResolvedValue({
      data: {
        ...checkoutInfoWithFiveZPayPlansFixture().data,
        recharge_fee_rate: 1,
      },
    })
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('阅读订阅套餐A')
    expect(text).toContain('手续费详情¥29元 + 1%')
    expect(text).toContain('5刀流量卡')
    expect(text).toContain('手续费详情¥2元 + 1%')
  })

  it('marks active subscription purchase cards without rendering the duplicated current subscription block', async () => {
    activeSubscriptionsState.items = [
      {
        id: 42,
        group_id: 2,
        expires_at: '2099-01-01T00:00:00Z',
        group: {
          name: 'codex-pool-19-usd',
          platform: 'openai',
          rate_multiplier: 1,
          daily_limit_usd: 19,
          weekly_limit_usd: null,
          monthly_limit_usd: null,
        },
      },
    ]

    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: {
            name: 'PurchaseProductCard',
            props: ['product'],
            template: '<div data-testid="purchase-product-card">{{ product.active ? "active" : "inactive" }}</div>',
          },
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(wrapper.find('[data-testid="purchase-product-card"]').text()).toContain('active')
    expect(wrapper.text()).not.toContain('payment.activeSubscription')
    expect(wrapper.text()).not.toContain('codex-pool-19-usd')
  })

  it.each([
    { index: 1, planId: 1, amount: 29, name: '29 元订阅池' },
    { index: 2, planId: 2, amount: 39, name: '39 元订阅池' },
    { index: 4, planId: 5, amount: 79, name: '79 元订阅池' },
  ])('creates a ZPay dynamic subscription order for $name', async ({ index, planId, amount }) => {
    getCheckoutInfo.mockResolvedValue(checkoutInfoWithFiveZPayPlansFixture())
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
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
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

    const purchaseCards = wrapper.findAll('[data-testid="purchase-product-card"]')
    expect(purchaseCards).toHaveLength(9)
    expect(purchaseCards[0].element.parentElement?.className).toContain('lg:grid-cols-4')
    expect(wrapper.text()).toContain('阅读订阅套餐A')
    expect(wrapper.text()).toContain('阅读订阅套餐D')
    expect(wrapper.text()).toContain('5刀流量卡')

    await purchaseCards[index].trigger('click')
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
    expect(wrapper.html()).not.toContain('manual-payment-dialog')
  })

  it('does not render traffic packs when backend returns none', async () => {
    getCheckoutInfo.mockResolvedValue({
      data: {
        ...checkoutInfoWithPlansFixture().data,
        traffic_packs: [],
        traffic_credit_summary: null,
      },
    })
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).not.toContain('GPT 流量包')
    expect(wrapper.findAll('button').some(button => button.text() === '立即购买')).toBe(false)
  })
})

describe('PaymentView without configured payment methods', () => {
  beforeEach(() => {
    routeState.path = '/purchase'
    routeState.query = {
      tab: 'subscription',
    }
    routerReplace.mockReset().mockResolvedValue(undefined)
    routerPush.mockReset().mockResolvedValue(undefined)
    routerResolve.mockClear()
    createOrder.mockReset()
    refreshUser.mockReset()
    fetchActiveSubscriptions.mockReset().mockResolvedValue(undefined)
    activeSubscriptionsState.items = []
    showError.mockReset()
    showInfo.mockReset()
    showWarning.mockReset()
    getCheckoutInfo.mockReset().mockResolvedValue(checkoutInfoWithManualPlansFixture())
    bridgeInvoke.mockReset()
    window.localStorage.clear()
    ;(window as Window & { WeixinJSBridge?: { invoke: typeof bridgeInvoke } }).WeixinJSBridge = {
      invoke: bridgeInvoke,
    }
  })

  it('opens recharge confirmation for subscription checkout when only balance is available and insufficient', async () => {
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    const purchaseCards = wrapper.findAllComponents({ name: 'PurchaseProductCard' })
    expect(purchaseCards[1].exists()).toBe(true)
    await purchaseCards[1].vm.$emit('select', purchaseCards[1].props('product'))
    await flushPromises()

    const confirmButton = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))
    expect(confirmButton?.attributes('disabled')).toBeUndefined()
    await confirmButton?.trigger('click')
    await flushPromises()

    expect(createOrder).not.toHaveBeenCalled()
    expect(balancePayOrder).not.toHaveBeenCalled()
    expect(wrapper.html()).not.toContain('manual-payment-dialog')
    expect(wrapper.text()).toContain('payment.recharge.title')
    expect((wrapper.get('[data-testid="balance-recharge-amount"]').element as HTMLInputElement).value).toBe('100')
    expect(showWarning).toHaveBeenCalledWith('payment.recharge.maxOnce')
  })

  it('renders five subscription tiers in a four-column desktop grid and routes the 79 yuan tier to recharge when balance is insufficient', async () => {
    getCheckoutInfo.mockResolvedValue(checkoutInfoWithFiveManualPlansFixture())

    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    const purchaseCards = wrapper.findAll('[data-testid="purchase-product-card"]')
    expect(purchaseCards).toHaveLength(9)
    expect(purchaseCards[0].element.parentElement?.className).toContain('lg:grid-cols-4')

    await purchaseCards[4].trigger('click')
    await flushPromises()

    const confirmButton = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))
    expect(confirmButton?.attributes('disabled')).toBeUndefined()
    await confirmButton?.trigger('click')
    await flushPromises()

    expect(createOrder).not.toHaveBeenCalled()
    expect(balancePayOrder).not.toHaveBeenCalled()
    expect(wrapper.html()).not.toContain('manual-payment-dialog')
    expect(wrapper.text()).toContain('payment.recharge.title')
    expect((wrapper.get('[data-testid="balance-recharge-amount"]').element as HTMLInputElement).value).toBe('79')
  })

  it('opens recharge confirmation for traffic pack checkout when only balance is available and insufficient', async () => {
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    const purchaseCards = wrapper.findAll('[data-testid="purchase-product-card"]')
    expect(purchaseCards).toHaveLength(5)
    await purchaseCards[2].trigger('click')
    await flushPromises()

    const confirmButton = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))
    expect(confirmButton?.attributes('disabled')).toBeUndefined()
    await confirmButton?.trigger('click')
    await flushPromises()

    expect(createOrder).not.toHaveBeenCalled()
    expect(balancePayOrder).not.toHaveBeenCalled()
    expect(wrapper.html()).not.toContain('manual-payment-dialog')
    expect(wrapper.text()).toContain('payment.recharge.title')
    expect((wrapper.get('[data-testid="balance-recharge-amount"]').element as HTMLInputElement).value).toBe('2')
  })

  it('shows a back action in traffic pack confirm view and returns to the list', async () => {
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          PurchaseProductCard: purchaseProductCardStub,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    const purchaseCards = wrapper.findAll('[data-testid="purchase-product-card"]')
    expect(purchaseCards).toHaveLength(5)
    await purchaseCards[3].trigger('click')
    await flushPromises()

    const backButton = wrapper.findAll('button').find(button => button.text().includes('common.back'))
    expect(backButton).toBeDefined()

    await backButton?.trigger('click')
    await flushPromises()

    expect(wrapper.findAll('[data-testid="purchase-product-card"]')).toHaveLength(5)
    expect(wrapper.findAll('button').some(button => button.text().includes('payment.createOrder'))).toBe(false)
  })
})

describe('PaymentView WeChat JSAPI flow', () => {
  beforeEach(() => {
    routeState.path = '/purchase'
    routeState.query = {
      wechat_resume: '1',
      wechat_resume_token: 'resume-token-123',
    }
    routerReplace.mockReset().mockResolvedValue(undefined)
    routerPush.mockReset().mockResolvedValue(undefined)
    routerResolve.mockClear()
    createOrder.mockReset()
    refreshUser.mockReset()
    fetchActiveSubscriptions.mockReset().mockResolvedValue(undefined)
    activeSubscriptionsState.items = []
    showError.mockReset()
    showInfo.mockReset()
    showWarning.mockReset()
    getCheckoutInfo.mockReset().mockResolvedValue(checkoutInfoFixture())
    bridgeInvoke.mockReset()
    window.localStorage.clear()
    ;(window as Window & { WeixinJSBridge?: { invoke: typeof bridgeInvoke } }).WeixinJSBridge = {
      invoke: bridgeInvoke,
    }
  })

  it('resets payment state and redirects to /payment/result after JSAPI reports success', async () => {
    createOrder.mockResolvedValue(jsapiOrderFixture('resume-token-123'))
    bridgeInvoke.mockImplementation((_action, _payload, callback) => {
      callback({ err_msg: 'get_brand_wcpay_request:ok' })
    })

    shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(routerReplace).toHaveBeenCalledWith({ path: '/purchase', query: {} })
    expect(routerPush).toHaveBeenCalledWith({
      path: '/payment/result',
      query: {
        order_id: '123',
        out_trade_no: 'sub2_jsapi_123',
        resume_token: 'resume-token-123',
      },
    })
    expect(window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toBeNull()
  })

  it('resets payment state when JSAPI reports cancellation', async () => {
    createOrder.mockResolvedValue(jsapiOrderFixture('resume-token-cancel'))
    bridgeInvoke.mockImplementation((_action, _payload, callback) => {
      callback({ err_msg: 'get_brand_wcpay_request:cancel' })
    })

    shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(showInfo).toHaveBeenCalledWith('payment.qr.cancelled')
    expect(routerPush).not.toHaveBeenCalled()
    expect(window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toBeNull()
  })

  it('clears stale recovery state when JSAPI never becomes available', async () => {
    vi.useFakeTimers()
    createOrder.mockResolvedValue(jsapiOrderFixture('resume-token-missing-bridge'))
    ;(window as Window & { WeixinJSBridge?: { invoke: typeof bridgeInvoke } }).WeixinJSBridge = undefined

    const wrapper = mount(PaymentView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Teleport: true,
          Transition: false,
          Icon: { template: '<span />' },
          PurchaseProductCard: purchaseProductCardStub,
        },
      },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(4000)
    await flushPromises()
    await flushPromises()

    expect(showError).toHaveBeenCalledWith(
      'payment.errors.wechatJsapiUnavailable payment.errors.wechatOpenInWeChatHint',
    )
    expect(routerPush).not.toHaveBeenCalled()
    expect(window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toBeNull()
    expect(wrapper.html()).not.toContain('payment-status-panel-stub')
  })

  it('clears a stale recovery snapshot before handling wechat resume callback params', async () => {
    createOrder.mockRejectedValueOnce(new Error('resume failed'))
    window.localStorage.setItem(PAYMENT_RECOVERY_STORAGE_KEY, JSON.stringify({
      orderId: 999,
      amount: 66,
      qrCode: 'stale-qr',
      expiresAt: '2099-01-01T00:10:00.000Z',
      paymentType: 'alipay',
      payUrl: 'https://pay.example.com/stale',
      outTradeNo: 'stale-out-trade-no',
      clientSecret: '',
      intentId: '',
      currency: '',
      countryCode: '',
      paymentEnv: '',
      payAmount: 66,
      orderType: 'balance',
      paymentMode: 'popup',
      resumeToken: '',
      createdAt: Date.UTC(2099, 0, 1, 0, 0, 0),
    }))

    shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(createOrder).toHaveBeenCalledWith(expect.objectContaining({
      wechat_resume_token: 'resume-token-123',
    }))
    expect(window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toBeNull()
  })

  it('keeps subscription resume context for token-only WeChat callbacks', async () => {
    routeState.query = {
      wechat_resume: '1',
      wechat_resume_token: 'resume-subscription-7',
      payment_type: 'wxpay_direct',
      order_type: 'subscription',
      plan_id: '7',
    }
    getCheckoutInfo.mockResolvedValue(checkoutInfoWithPlansFixture())
    createOrder.mockResolvedValue(oauthOrderFixture())

    const originalLocation = window.location
    const locationState = {
      href: 'http://localhost/purchase',
      origin: 'http://localhost',
    }
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: locationState,
    })

    shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(routerReplace).toHaveBeenCalledWith({ path: '/purchase', query: {} })
    expect(createOrder).toHaveBeenCalledWith(expect.objectContaining({
      payment_type: 'wxpay',
      order_type: 'subscription',
      plan_id: 7,
      wechat_resume_token: 'resume-subscription-7',
    }))
    expect(locationState.href).toContain('/api/v1/auth/oauth/wechat/payment/start?')
    expect(new URL(locationState.href, 'http://localhost').searchParams.get('redirect')).toBe(
      '/purchase?from=wechat&payment_type=wxpay&order_type=subscription&plan_id=7',
    )

    Object.defineProperty(window, 'location', {
      configurable: true,
      value: originalLocation,
    })
  })

  it('falls back to QR flow when mobile WeChat payment is unavailable', async () => {
    routeState.query = {
      wechat_resume: '1',
      wechat_resume_token: 'resume-token-h5',
      payment_type: 'wxpay_direct',
    }
    createOrder
      .mockRejectedValueOnce({ reason: 'WECHAT_H5_NOT_AUTHORIZED' })
      .mockResolvedValueOnce({
        order_id: 778,
        amount: 88,
        pay_amount: 88,
        fee_rate: 0,
        expires_at: '2099-01-01T00:10:00.000Z',
        payment_type: 'wxpay',
        qr_code: 'weixin://wxpay/bizpayurl?pr=fallback-native',
        out_trade_no: 'sub2_qr_778',
      })

    shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(createOrder).toHaveBeenNthCalledWith(1, expect.objectContaining({
      payment_type: 'wxpay',
      is_mobile: true,
      wechat_resume_token: 'resume-token-h5',
    }))
    expect(createOrder).toHaveBeenNthCalledWith(2, expect.objectContaining({
      payment_type: 'wxpay',
      is_mobile: false,
      payment_source: 'hosted_redirect',
    }))
    expect(showWarning).toHaveBeenCalledWith('payment.errors.mobilePaymentFallbackToQr')
    expect(showError).not.toHaveBeenCalled()
    expect(window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toContain('weixin://wxpay/bizpayurl?pr=fallback-native')
  })
})

describe('PaymentView page responsibility', () => {
  beforeEach(() => {
    routeState.path = '/purchase'
    routeState.query = { tab: 'subscription' }
    routerReplace.mockReset().mockResolvedValue(undefined)
    routerPush.mockReset().mockResolvedValue(undefined)
    routerResolve.mockClear()
    createOrder.mockReset()
    refreshUser.mockReset()
    fetchActiveSubscriptions.mockReset().mockResolvedValue(undefined)
    showError.mockReset()
    showInfo.mockReset()
    showWarning.mockReset()
    getCheckoutInfo.mockReset().mockResolvedValue(checkoutInfoWithPlansFixture())
    activeSubscriptionsState.items = [
      {
        id: 101,
        group_id: 2,
        status: 'active',
        expires_at: '2099-01-01T00:00:00.000Z',
        group: {
          name: 'codex-pool-19-usd',
          platform: 'openai',
          rate_multiplier: 1,
          daily_limit_usd: 19,
          weekly_limit_usd: null,
          monthly_limit_usd: null,
        },
      },
    ]
  })

  it('不在购买页展示当前订阅详情', async () => {
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).not.toContain('payment.activeSubscription')
    expect(wrapper.text()).not.toContain('codex-pool-19-usd')
  })
})
