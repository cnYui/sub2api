import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SubscriptionsView from '../SubscriptionsView.vue'

const getMySubscriptions = vi.hoisted(() => vi.fn())
const getCheckoutInfo = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const routerPush = vi.hoisted(() => vi.fn())

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRouter: () => ({
      push: routerPush,
    }),
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'userSubscriptions.daysRemaining') return `剩余 ${params?.days} 天`
        return key
      },
    }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
  }),
}))

vi.mock('@/api/subscriptions', () => ({
  default: {
    getMySubscriptions,
  },
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    getCheckoutInfo,
  },
}))

describe('SubscriptionsView traffic packs', () => {
  beforeEach(() => {
    getMySubscriptions.mockReset()
    getCheckoutInfo.mockReset()
    showError.mockReset()
    routerPush.mockReset()
  })

  it('按 API 顺序逐张展示 GPT 流量卡', async () => {
    getMySubscriptions.mockResolvedValue([])
    getCheckoutInfo.mockResolvedValue({
      data: {
        methods: {},
        global_min: 0,
        global_max: 0,
        plans: [],
        traffic_packs: [
          {
            id: 2,
            code: 'gpt_traffic_10usd_3cny',
            name: 'GPT 流量包 10 刀',
            description: '一次性流量包',
            price: 3,
            credit_usd: 10,
            validity_days: 365,
            platform: 'openai',
            for_sale: true,
            sort_order: 2,
          },
        ],
        traffic_credit_summary: null,
        traffic_credits: [
          {
            id: 20,
            order_id: 101,
            pack_id: 2,
            initial_usd: 5,
            remaining_usd: 3,
            reserved_usd: 0.5,
            available_usd: 2.5,
            credited_at: '2026-07-16T08:57:24+08:00',
            expires_at: '2027-06-26T08:57:24+08:00',
          },
          {
            id: 10,
            order_id: 99,
            pack_id: 1,
            initial_usd: 3,
            remaining_usd: 2,
            reserved_usd: 2,
            available_usd: 0,
            credited_at: '2026-07-15T08:57:24+08:00',
            expires_at: '2027-07-26T08:57:24+08:00',
          },
        ],
        balance_disabled: false,
        recharge_fee_rate: 0,
        help_text: '',
        help_image_url: '',
        stripe_publishable_key: '',
      },
    })

    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: { template: '<span />' },
        },
      },
    })
    await flushPromises()

    const pageText = wrapper.text()
    expect(pageText.indexOf('GPT 流量卡 #20')).toBeGreaterThanOrEqual(0)
    expect(pageText.indexOf('GPT 流量卡 #10')).toBeGreaterThan(pageText.indexOf('GPT 流量卡 #20'))
    expect(pageText).toContain('已结算用量')
    expect(pageText).toContain('$2.00 / $5.00')
    expect(pageText).toContain('$1.00 / $3.00')
    expect(pageText).toContain('当前可用 $2.50')
    expect(pageText).toContain('当前可用 $0.00')
    expect(pageText).toContain('2027')
    expect(wrapper.find('[data-testid="traffic-credit-progress-20"]').attributes('style')).toContain('width: 40%')
    expect(wrapper.text()).not.toContain('2027/6/26')
    expect(wrapper.text()).not.toContain('GPT 流量包 10 刀')
    expect(wrapper.text()).not.toContain('¥3')
    expect(wrapper.findAll('button').some(button => button.text().includes('购买'))).toBe(false)
    expect(routerPush).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('userSubscriptions.noActiveSubscriptions')
  })

  it('后端只返回剩余流量卡时页面只展示该卡', async () => {
    getMySubscriptions.mockResolvedValue([])
    getCheckoutInfo.mockResolvedValue({
      data: {
        methods: {},
        global_min: 0,
        global_max: 0,
        plans: [],
        traffic_packs: [],
        traffic_credit_summary: null,
        traffic_credits: [
          {
            id: 10,
            order_id: 99,
            pack_id: 1,
            initial_usd: 3,
            remaining_usd: 2,
            reserved_usd: 0,
            available_usd: 2,
            credited_at: '2026-07-15T08:57:24+08:00',
            expires_at: '2027-07-26T08:57:24+08:00',
          },
        ],
        balance_disabled: false,
        recharge_fee_rate: 0,
        help_text: '',
        help_image_url: '',
        stripe_publishable_key: '',
      },
    })

    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: { template: '<span />' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('GPT 流量卡 #10')
    expect(wrapper.text()).not.toContain('GPT 流量卡 #20')
    expect(wrapper.text()).toContain('$1.00 / $3.00')
    expect(wrapper.text()).toContain('当前可用 $2.00')
  })

  it('所有流量卡用满后隐藏流量包卡片', async () => {
    getMySubscriptions.mockResolvedValue([])
    getCheckoutInfo.mockResolvedValue({
      data: {
        methods: {},
        global_min: 0,
        global_max: 0,
        plans: [],
        traffic_packs: [],
        traffic_credit_summary: null,
        traffic_credits: [],
        balance_disabled: false,
        recharge_fee_rate: 0,
        help_text: '',
        help_image_url: '',
        stripe_publishable_key: '',
      },
    })

    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: { template: '<span />' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="traffic-credit-progress"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('userSubscriptions.noActiveSubscriptions')
    expect(wrapper.text()).not.toContain('GPT 流量包')
  })

  it('订阅页展示 active 订阅时不提供续费入口', async () => {
    getMySubscriptions.mockResolvedValue([
      {
        id: 7,
        group_id: 2,
        status: 'active',
        expires_at: '2026-07-21T00:00:00+08:00',
        daily_usage_usd: 0,
        weekly_usage_usd: 0,
        monthly_usage_usd: 0,
        daily_window_start: '2026-06-26T00:00:00+08:00',
        weekly_window_start: null,
        monthly_window_start: null,
        group: {
          id: 2,
          name: 'codex-pool-19-usd',
          platform: 'openai',
          description: 'yui.web 29 元订阅池迁移：每日 19 USD',
          rate_multiplier: 1,
          daily_limit_usd: 19,
          weekly_limit_usd: null,
          monthly_limit_usd: null,
        },
      },
    ])
    getCheckoutInfo.mockResolvedValue({
      data: {
        methods: {},
        global_min: 0,
        global_max: 0,
        plans: [],
        traffic_packs: [],
        traffic_credit_summary: null,
        traffic_credits: [],
        balance_disabled: false,
        recharge_fee_rate: 0,
        help_text: '',
        help_image_url: '',
        stripe_publishable_key: '',
      },
    })

    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: { template: '<span />' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('codex-pool-19-usd')
    expect(wrapper.text()).not.toContain('payment.renewNow')
    expect(wrapper.findAll('button').some(button => button.text().includes('续费'))).toBe(false)
    expect(routerPush).not.toHaveBeenCalled()
  })
})
