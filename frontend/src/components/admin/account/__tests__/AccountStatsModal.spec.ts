import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import AccountStatsModal from '../AccountStatsModal.vue'

const { getStats } = vi.hoisted(() => ({
  getStats: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getStats
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const statsFixture = {
  history: [],
  models: [],
  endpoints: [],
  upstream_endpoints: [],
  summary: {
    days: 30,
    actual_days_used: 1,
    total_cost: 12.34,
    total_user_cost: 15.67,
    total_standard_cost: 10.11,
    total_requests: 42,
    total_tokens: 2048,
    avg_daily_cost: 12.34,
    avg_daily_user_cost: 15.67,
    avg_daily_requests: 42,
    avg_daily_tokens: 2048,
    avg_duration_ms: 1234,
    today: {
      date: '2026-07-17',
      cost: 1.23,
      user_cost: 4.56,
      requests: 7,
      tokens: 890
    },
    highest_cost_day: null,
    highest_request_day: null
  }
}

describe('AccountStatsModal', () => {
  beforeEach(() => {
    getStats.mockReset()
    getStats.mockResolvedValue(statsFixture)
  })

  it('最近活动展示今日账号计费和用户扣费', async () => {
    const wrapper = mount(AccountStatsModal, {
      props: {
        show: false,
        account: {
          id: 52,
          name: 'OpenAI OAuth',
          status: 'active'
        }
      } as any,
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          LoadingSpinner: true,
          Line: true,
          ModelDistributionChart: true,
          EndpointDistributionChart: true,
          Icon: true
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    const recentActivity = wrapper
      .findAll('.card')
      .find((card) => card.text().includes('admin.accounts.stats.recentActivity'))

    expect(recentActivity).toBeTruthy()
    expect(recentActivity!.text()).toContain('usage.accountBilled')
    expect(recentActivity!.text()).toContain('$1.23')
    expect(recentActivity!.text()).toContain('usage.userBilled')
    expect(recentActivity!.text()).toContain('$4.56')
    expect(recentActivity!.text()).not.toContain('admin.accounts.stats.todayCost')
  })
})
