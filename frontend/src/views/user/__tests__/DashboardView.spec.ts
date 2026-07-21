import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

import DashboardView from '../DashboardView.vue'
import type { UserDashboardStats } from '@/api/usage'

const mockRefreshUser = vi.fn()
const mockGetDashboardStats = vi.fn()
const mockGetDashboardQuota = vi.fn()
const mockGetDashboardTrend = vi.fn()
const mockGetDashboardModels = vi.fn()
const mockGetByDateRange = vi.fn()
const mockGetMyPlatformQuotas = vi.fn()

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 1, email: 'test@example.com', balance: 100, role: 'user' },
    isSimpleMode: false,
    refreshUser: mockRefreshUser,
  }),
}))

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardStats: (...args: unknown[]) => mockGetDashboardStats(...args),
    getDashboardQuota: (...args: unknown[]) => mockGetDashboardQuota(...args),
    getDashboardTrend: (...args: unknown[]) => mockGetDashboardTrend(...args),
    getDashboardModels: (...args: unknown[]) => mockGetDashboardModels(...args),
    getByDateRange: (...args: unknown[]) => mockGetByDateRange(...args),
  },
}))

vi.mock('@/api/user', () => ({
  getMyPlatformQuotas: (...args: unknown[]) => mockGetMyPlatformQuotas(...args),
}))

const stats: UserDashboardStats = {
  total_api_keys: 1,
  active_api_keys: 1,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  average_duration_ms: 0,
  rpm: 0,
  tpm: 0,
  by_platform: [],
  quota: {
    period_mode: 'entitlement_period',
    today_usage_usd: 0,
    today_limit_usd: 15,
    today_limit_unlimited: false,
    period_usage_usd: 0,
    period_limit_usd: 570,
    period_limit_unlimited: false,
    period_days: 30,
  },
}

describe('DashboardView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setDocumentVisibility('visible')
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('加载 dashboard 时不再请求平台 quota 接口', async () => {
    mockRefreshUser.mockResolvedValue(undefined)
    mockGetDashboardStats.mockResolvedValue(stats)
    mockGetDashboardTrend.mockResolvedValue({ trend: [] })
    mockGetDashboardModels.mockResolvedValue({ models: [] })
    mockGetByDateRange.mockResolvedValue({ items: [] })
    mockGetMyPlatformQuotas.mockResolvedValue({ platform_quotas: [] })

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: { template: '<div />' },
          UserDashboardStats: { template: '<section data-testid="stats" />' },
          UserDashboardCharts: { template: '<section />' },
          UserDashboardRecentUsage: { template: '<section />' },
          UserDashboardQuickActions: { template: '<section />' },
        },
      },
    })

    await flushPromises()

    expect(mockGetDashboardStats).toHaveBeenCalledTimes(1)
    expect(mockGetDashboardTrend).toHaveBeenCalledTimes(1)
    expect(mockGetDashboardModels).toHaveBeenCalledTimes(1)
    expect(mockGetByDateRange).toHaveBeenCalledTimes(1)
    expect(mockGetMyPlatformQuotas).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('每 15 秒只刷新轻量 quota，并在卸载时清理计时器', async () => {
    vi.useFakeTimers()
    mockRefreshUser.mockResolvedValue(undefined)
    mockGetDashboardStats.mockResolvedValue(stats)
    mockGetDashboardQuota.mockResolvedValue({
      period_mode: 'entitlement_period',
      today_usage_usd: 2.5,
      today_limit_usd: 15,
      today_limit_unlimited: false,
      period_usage_usd: 8,
      period_limit_usd: 570,
      period_limit_unlimited: false,
      period_days: 30,
    })
    mockGetDashboardTrend.mockResolvedValue({ trend: [] })
    mockGetDashboardModels.mockResolvedValue({ models: [] })
    mockGetByDateRange.mockResolvedValue({ items: [] })

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: { template: '<div />' },
          UserDashboardStats: {
            props: ['stats'],
            template: '<section data-testid="stats">{{ stats.quota.today_usage_usd }}</section>',
          },
          UserDashboardCharts: { template: '<section />' },
          UserDashboardRecentUsage: { template: '<section />' },
          UserDashboardQuickActions: { template: '<section />' },
        },
      },
    })

    await flushPromises()
    expect(wrapper.find('[data-testid="stats"]').text()).toBe('0')

    await vi.advanceTimersByTimeAsync(15000)
    await flushPromises()

    expect(mockGetDashboardQuota).toHaveBeenCalledTimes(1)
    expect(mockGetDashboardStats).toHaveBeenCalledTimes(1)
    expect(mockGetDashboardTrend).toHaveBeenCalledTimes(1)
    expect(mockGetDashboardModels).toHaveBeenCalledTimes(1)
    expect(mockGetByDateRange).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="stats"]').text()).toBe('2.5')

    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(15000)
    expect(mockGetDashboardQuota).toHaveBeenCalledTimes(1)
  })

  it('页面恢复前台时立即刷新 quota', async () => {
    vi.useFakeTimers()
    mockRefreshUser.mockResolvedValue(undefined)
    mockGetDashboardStats.mockResolvedValue(stats)
    mockGetDashboardQuota.mockResolvedValue({
      period_mode: 'entitlement_period',
      today_usage_usd: 3.5,
      today_limit_usd: 15,
      today_limit_unlimited: false,
      period_usage_usd: 9,
      period_limit_usd: 570,
      period_limit_unlimited: false,
      period_days: 30,
    })
    mockGetDashboardTrend.mockResolvedValue({ trend: [] })
    mockGetDashboardModels.mockResolvedValue({ models: [] })
    mockGetByDateRange.mockResolvedValue({ items: [] })

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: { template: '<div />' },
          UserDashboardStats: {
            props: ['stats'],
            template: '<section data-testid="stats">{{ stats.quota.today_usage_usd }}</section>',
          },
          UserDashboardCharts: { template: '<section />' },
          UserDashboardRecentUsage: { template: '<section />' },
          UserDashboardQuickActions: { template: '<section />' },
        },
      },
    })

    await flushPromises()
    window.dispatchEvent(new Event('focus'))
    await flushPromises()

    expect(mockGetDashboardQuota).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="stats"]').text()).toBe('3.5')

    wrapper.unmount()
  })

  it('隐藏时暂停 quota 轮询，恢复和连续 focus 只合并一条刷新请求', async () => {
    vi.useFakeTimers()
    mockRefreshUser.mockResolvedValue(undefined)
    mockGetDashboardStats.mockResolvedValue(stats)
    mockGetDashboardTrend.mockResolvedValue({ trend: [] })
    mockGetDashboardModels.mockResolvedValue({ models: [] })
    mockGetByDateRange.mockResolvedValue({ items: [] })
    let resolveQuota: ((value: unknown) => void) | null = null
    mockGetDashboardQuota.mockImplementation(() => new Promise(resolve => {
      resolveQuota = resolve
    }))

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: { template: '<div />' },
          UserDashboardStats: { template: '<section data-testid="stats" />' },
          UserDashboardCharts: { template: '<section />' },
          UserDashboardRecentUsage: { template: '<section />' },
          UserDashboardQuickActions: { template: '<section />' },
        },
      },
    })

    await flushPromises()
    setDocumentVisibility('hidden')
    document.dispatchEvent(new Event('visibilitychange'))

    await vi.advanceTimersByTimeAsync(30000)
    expect(mockGetDashboardQuota).not.toHaveBeenCalled()

    setDocumentVisibility('visible')
    document.dispatchEvent(new Event('visibilitychange'))
    window.dispatchEvent(new Event('focus'))
    window.dispatchEvent(new Event('focus'))

    expect(mockGetDashboardQuota).toHaveBeenCalledTimes(1)
    resolveQuota?.({
      period_mode: 'entitlement_period',
      today_usage_usd: 4,
      today_limit_usd: 15,
      today_limit_unlimited: false,
      period_usage_usd: 10,
      period_limit_usd: 570,
      period_limit_unlimited: false,
      period_days: 30,
    })
    await flushPromises()

    await vi.advanceTimersByTimeAsync(15000)
    expect(mockGetDashboardQuota).toHaveBeenCalledTimes(2)
    expect(mockGetDashboardStats).toHaveBeenCalledTimes(1)
    expect(mockGetDashboardTrend).toHaveBeenCalledTimes(1)
    expect(mockGetDashboardModels).toHaveBeenCalledTimes(1)
    expect(mockGetByDateRange).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })
})

function setDocumentVisibility(value: DocumentVisibilityState) {
  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    value,
  })
}
