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
        if (key === 'userSubscriptions.trafficPack.title') return `GPT 流量卡 #${params?.id}`
        if (key === 'userSubscriptions.trafficPack.description') return `剩余额度 ${params?.remaining}，当前可用 ${params?.available}。`
        if (key === 'userSubscriptions.trafficPack.settledUsage') return '已结算用量'
        if (key === 'userSubscriptions.trafficPack.currentAvailable') return `当前可用 ${params?.amount}`
        if (key === 'userSubscriptions.expires') return '到期时间'
        if (key === 'userSubscriptions.weeklyWindowNotActive') return '当前周额度窗口尚未激活'
        if (key === 'payment.planCard.weeklyDescription') return `每周 ${params?.quota}，${params?.days} 天有效期`
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
    expect(wrapper.find('[data-testid="traffic-credit-progress-20"]').attributes('style')).toContain('--meter-value: 0.4')
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
          description: '29 元订阅池，每周 58 USD，28 天有效期',
          rate_multiplier: 1,
          daily_limit_usd: null,
          weekly_limit_usd: 58,
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
    expect(wrapper.text()).toContain('每周 $58，28 天有效期')
    expect(wrapper.text()).not.toContain('29 元订阅池，每周 58 USD，28 天有效期')
    expect(wrapper.text()).not.toContain('payment.renewNow')
    expect(wrapper.findAll('button').some(button => button.text().includes('续费'))).toBe(false)
    expect(routerPush).not.toHaveBeenCalled()
  })

  it('公共 Codex 周订阅没有后端 reset 字段时明确展示窗口未激活', async () => {
    getMySubscriptions.mockResolvedValue([
      {
        id: 8,
        group_id: 2,
        status: 'active',
        starts_at: '2026-07-20T00:00:00+08:00',
        expires_at: '2099-01-01T00:00:00+08:00',
        daily_usage_usd: 0,
        weekly_usage_usd: 0,
        monthly_usage_usd: 0,
        daily_window_start: null,
        weekly_window_start: '2026-07-20T00:00:00+08:00',
        weekly_window_resets_at: null,
        effective_weekly_limit_usd: null,
        monthly_window_start: null,
        group: {
          id: 2,
          name: 'codex-pool-19-usd',
          platform: 'openai',
          description: '29 元订阅池，每周 58 USD，28 天有效期',
          rate_multiplier: 1,
          daily_limit_usd: null,
          weekly_limit_usd: 58,
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

    expect(wrapper.text()).toContain('当前周额度窗口尚未激活')
    expect(wrapper.text()).not.toContain('后重置')
    expect(wrapper.text()).not.toContain('$0 / $58')
  })

  it('公共 Codex 订阅即使 group 仍是旧日额度也展示有效周额度', async () => {
    getMySubscriptions.mockResolvedValue([
      {
        id: 9,
        group_id: 2,
        status: 'active',
        starts_at: '2026-07-20T00:00:00+08:00',
        expires_at: '2026-07-21T00:00:00+08:00',
        daily_usage_usd: 0,
        weekly_usage_usd: 1.2,
        monthly_usage_usd: 0,
        daily_window_start: '2026-07-20T00:00:00+08:00',
        weekly_window_start: '2026-07-20T00:00:00+08:00',
        weekly_window_resets_at: '2026-07-21T00:00:00+08:00',
        effective_weekly_limit_usd: 58 / 7,
        monthly_window_start: null,
        group: {
          id: 2,
          name: 'codex-pool-19-usd',
          platform: 'openai',
          description: '历史旧 group 行',
          rate_multiplier: 1,
          daily_limit_usd: 15,
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

    expect(wrapper.text()).toContain('$1 / $8')
    expect(wrapper.text()).not.toContain('$0 / $15')
  })
})
