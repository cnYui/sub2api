import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SubscriptionProgressMini from '../SubscriptionProgressMini.vue'

const fetchActiveSubscriptions = vi.hoisted(() => vi.fn())
const mockStore = vi.hoisted(() => ({
  activeSubscriptions: [] as any[],
  hasActiveSubscriptions: false,
  fetchActiveSubscriptions,
}))

vi.mock('@/stores', () => ({
  useSubscriptionStore: () => mockStore,
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string) => `格式化时间:${value}`,
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (key === 'subscriptionProgress.title') return '我的订阅'
      if (key === 'subscriptionProgress.activeCount') return `${params?.count} 个有效订阅`
      if (key === 'subscriptionProgress.weekly') return '每周'
      if (key === 'subscriptionProgress.resetsAt') return `重置于 ${params?.time}`
      if (key === 'subscriptionProgress.weeklyWindowNotActive') return '当前周额度窗口尚未激活'
      if (key === 'subscriptionProgress.daysRemaining') return `剩余 ${params?.days} 天`
      if (key === 'subscriptionProgress.viewAll') return '查看全部订阅'
      return key
    },
  }),
}))

describe('SubscriptionProgressMini', () => {
  beforeEach(() => {
    fetchActiveSubscriptions.mockReset()
    fetchActiveSubscriptions.mockResolvedValue([])
    mockStore.activeSubscriptions = []
    mockStore.hasActiveSubscriptions = false
  })

  it('周订阅进度展示后端返回的窗口重置时间', async () => {
    mockStore.activeSubscriptions = [
      {
        id: 1,
        group_id: 2,
        group: {
          id: 2,
          name: 'codex-pool-19-usd',
          weekly_limit_usd: 76,
          daily_limit_usd: null,
          monthly_limit_usd: null,
        },
        weekly_usage_usd: 12.4,
        effective_weekly_limit_usd: 76,
        weekly_window_resets_at: '2026-07-29T00:00:00+09:00',
        expires_at: '2026-08-19T00:00:00+09:00',
      },
    ]
    mockStore.hasActiveSubscriptions = true

    const wrapper = mount(SubscriptionProgressMini, {
      global: {
        stubs: {
          Icon: { template: '<span />' },
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })

    await wrapper.find('button').trigger('click')

    expect(wrapper.text()).toContain('$12/$76')
    expect(wrapper.text()).toContain('重置于 格式化时间:2026-07-29T00:00:00+09:00')
  })

  it('公共 Codex 周订阅没有 reset 字段时提示窗口未激活', async () => {
    mockStore.activeSubscriptions = [
      {
        id: 2,
        group_id: 2,
        group: {
          id: 2,
          name: 'codex-pool-19-usd',
          weekly_limit_usd: 76,
          daily_limit_usd: null,
          monthly_limit_usd: null,
        },
        weekly_usage_usd: 0,
        effective_weekly_limit_usd: null,
        weekly_window_resets_at: null,
        expires_at: '2026-08-19T00:00:00+09:00',
      },
    ]
    mockStore.hasActiveSubscriptions = true

    const wrapper = mount(SubscriptionProgressMini, {
      global: {
        stubs: {
          Icon: { template: '<span />' },
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })

    await wrapper.find('button').trigger('click')

    expect(wrapper.text()).toContain('当前周额度窗口尚未激活')
    expect(wrapper.text()).not.toContain('重置于')
    expect(wrapper.text()).not.toContain('$0/$76')
  })

  it('公共 Codex 旧日额度 group 行不会在顶部进度显示成日额度', async () => {
    mockStore.activeSubscriptions = [
      {
        id: 3,
        group_id: 2,
        group: {
          id: 2,
          name: 'codex-pool-19-usd',
          weekly_limit_usd: null,
          daily_limit_usd: 15,
          monthly_limit_usd: null,
        },
        daily_usage_usd: 0,
        weekly_usage_usd: 1.2,
        effective_weekly_limit_usd: 76 / 7,
        weekly_window_resets_at: '2026-07-21T00:00:00+09:00',
        expires_at: '2026-07-21T00:00:00+09:00',
      },
    ]
    mockStore.hasActiveSubscriptions = true

    const wrapper = mount(SubscriptionProgressMini, {
      global: {
        stubs: {
          Icon: { template: '<span />' },
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })

    await wrapper.find('button').trigger('click')

    expect(wrapper.text()).toContain('$1/$11')
    expect(wrapper.text()).not.toContain('$0/$15')
  })
})
