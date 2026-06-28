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
const refreshUser = vi.hoisted(() => vi.fn())
const fetchActiveSubscriptions = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const activeSubscriptionsState = vi.hoisted(() => ({
  items: [] as Array<Record<string, unknown>>,
}))
const showError = vi.hoisted(() => vi.fn())
const showInfo = vi.hoisted(() => vi.fn())
const showWarning = vi.hoisted(() => vi.fn())
const getCheckoutInfo = vi.hoisted(() => vi.fn())
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
      balance: 0,
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
    showWarning,
  }),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
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
          price: 79.79,
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
    refreshUser.mockReset()
    fetchActiveSubscriptions.mockReset().mockResolvedValue(undefined)
    activeSubscriptionsState.items = []
    showError.mockReset()
    showInfo.mockReset()
    showWarning.mockReset()
    getCheckoutInfo.mockReset().mockResolvedValue(checkoutInfoWithPlansFixture())
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
          SubscriptionPlanCard: {
            name: 'SubscriptionPlanCard',
            template: '<div data-testid="subscription-plan-card"></div>',
          },
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
    expect(wrapper.find('[data-testid="subscription-plan-card"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('payment.rechargeAccount')
    expect(wrapper.text()).not.toContain('payment.currentBalance')
    expect(wrapper.text()).not.toContain('payment.paymentAmount')
    expect(wrapper.findComponent({ name: 'AmountInput' }).exists()).toBe(false)
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
          SubscriptionPlanCard: {
            name: 'SubscriptionPlanCard',
            template: '<div data-testid="subscription-plan-card"></div>',
          },
          TrafficPackCard: {
            name: 'TrafficPackCard',
            template: '<div data-testid="traffic-pack-card"></div>',
          },
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
          SubscriptionPlanCard: {
            name: 'SubscriptionPlanCard',
            template: '<div data-testid="subscription-plan-card"></div>',
          },
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(wrapper.find('[data-testid="subscription-plan-card"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('payment.rechargeAccount')
    expect(wrapper.findComponent({ name: 'AmountInput' }).exists()).toBe(false)
  })

  it('renders GPT traffic pack cards and creates a traffic pack order', async () => {
    createOrder.mockResolvedValue({
      order_id: 991,
      amount: 5,
      pay_amount: 5,
      fee_rate: 0,
      expires_at: '2099-01-01T00:10:00.000Z',
      payment_type: 'wxpay',
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
          SubscriptionPlanCard: {
            name: 'SubscriptionPlanCard',
            template: '<div data-testid="subscription-plan-card"></div>',
          },
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

    expect(wrapper.text()).toContain('GPT 流量包 5 刀')
    expect(wrapper.text()).toContain('GPT 流量包 10 刀')
    expect(wrapper.text()).toContain('GPT 流量包 20 刀')

    const trafficPackCards = wrapper.findAll('[data-testid="traffic-pack-card"]')
    expect(trafficPackCards).toHaveLength(3)
    await trafficPackCards[2].trigger('click')
    await flushPromises()

    const confirmButton = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))
    expect(confirmButton?.attributes('disabled')).toBeUndefined()
    await confirmButton?.trigger('click')
    await flushPromises()

    expect(createOrder).toHaveBeenCalledWith(expect.objectContaining({
      amount: 5,
      payment_type: 'wxpay',
      order_type: 'traffic_pack',
      traffic_pack_id: 3,
      is_mobile: true,
    }))
  })

  it('passes the checkout fee rate to subscription and traffic pack cards', async () => {
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
          SubscriptionPlanCard: {
            name: 'SubscriptionPlanCard',
            props: ['plan', 'feeRate'],
            template: '<div data-testid="subscription-plan-card">{{ plan.name }} {{ feeRate }}</div>',
          },
          TrafficPackCard: {
            name: 'TrafficPackCard',
            props: ['pack', 'feeRate'],
            template: '<div data-testid="traffic-pack-card">{{ pack.name }} {{ feeRate }}</div>',
          },
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(wrapper.find('[data-testid="subscription-plan-card"]').text()).toContain('1')
    expect(wrapper.find('[data-testid="traffic-pack-card"]').text()).toContain('1')
  })

  it('keeps active subscription state for plan cards without rendering the duplicated current subscription block', async () => {
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
          SubscriptionPlanCard: {
            name: 'SubscriptionPlanCard',
            props: ['activeSubscriptions'],
            template: '<div data-testid="subscription-plan-card">{{ activeSubscriptions.length }}</div>',
          },
        },
      },
    })
    await flushPromises()
    await flushPromises()

    const planCard = wrapper.findComponent({ name: 'SubscriptionPlanCard' })
    expect(planCard.props('activeSubscriptions')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('payment.activeSubscription')
    expect(wrapper.text()).not.toContain('codex-pool-19-usd')
  })

  it.each([
    { index: 0, planId: 1, amount: 29, name: '29 元订阅池' },
    { index: 1, planId: 2, amount: 39, name: '39 元订阅池' },
    { index: 3, planId: 5, amount: 79.79, name: '79 元订阅池' },
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
    expect(planCards).toHaveLength(5)
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
          SubscriptionPlanCard: {
            name: 'SubscriptionPlanCard',
            template: '<div data-testid="subscription-plan-card"></div>',
          },
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).not.toContain('GPT 流量包')
    expect(wrapper.findAll('button').some(button => button.text() === '购买')).toBe(false)
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

  it('disables subscription confirmation instead of opening manual payment when no payment methods are configured', async () => {
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
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

  it('renders five subscription tiers in a four-column desktop grid and disables the 79 yuan tier without payment methods', async () => {
    getCheckoutInfo.mockResolvedValue(checkoutInfoWithFiveManualPlansFixture())

    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
          Teleport: true,
          Transition: false,
          SubscriptionPlanCard: {
            name: 'SubscriptionPlanCard',
            props: ['plan'],
            template: '<button data-testid="subscription-plan-card" @click="$emit(\'select\', plan)">{{ plan.name }}</button>',
          },
        },
      },
    })
    await flushPromises()
    await flushPromises()

    const planCards = wrapper.findAll('[data-testid="subscription-plan-card"]')
    expect(planCards).toHaveLength(5)
    expect(planCards[0].element.parentElement?.className).toContain('lg:grid-cols-4')

    await planCards[3].trigger('click')
    await flushPromises()

    const confirmButton = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))
    expect(confirmButton?.attributes('disabled')).toBeDefined()
    await confirmButton?.trigger('click')
    await flushPromises()

    expect(createOrder).not.toHaveBeenCalled()
    expect(wrapper.html()).not.toContain('manual-payment-dialog')
    expect(wrapper.text()).toContain('payment.notAvailable')
  })

  it('disables traffic pack confirmation instead of opening manual payment when no payment methods are configured', async () => {
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
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

  it('shows a back action in traffic pack confirm view and returns to the list', async () => {
    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<div><slot /></div>',
          },
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
    await trafficPackCards[1].trigger('click')
    await flushPromises()

    const backButton = wrapper.findAll('button').find(button => button.text().includes('common.back'))
    expect(backButton).toBeDefined()

    await backButton?.trigger('click')
    await flushPromises()

    expect(wrapper.findAll('[data-testid="traffic-pack-card"]')).toHaveLength(3)
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
          SubscriptionPlanCard: { template: '<div />' },
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
    activeSubscriptionsState.value = [
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
