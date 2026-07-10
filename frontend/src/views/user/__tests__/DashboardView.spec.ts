import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

import DashboardView from '../DashboardView.vue'
import type { UserDashboardStats } from '@/api/usage'

const mockRefreshUser = vi.fn()
const mockGetDashboardStats = vi.fn()
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
}

describe('DashboardView', () => {
  it('加载 dashboard 时不再请求平台 quota 接口', async () => {
    mockRefreshUser.mockResolvedValue(undefined)
    mockGetDashboardStats.mockResolvedValue(stats)
    mockGetDashboardTrend.mockResolvedValue({ trend: [] })
    mockGetDashboardModels.mockResolvedValue({ models: [] })
    mockGetByDateRange.mockResolvedValue({ items: [] })
    mockGetMyPlatformQuotas.mockResolvedValue({ platform_quotas: [] })

    mount(DashboardView, {
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
  })
})
