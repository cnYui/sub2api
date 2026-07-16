import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AdminPaymentDashboardView from '../AdminPaymentDashboardView.vue'

const getDashboard = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    getDashboard,
  },
  default: {
    getDashboard,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const labels: Record<string, string> = {
    'payment.admin.paymentDistribution': '支付方式分布',
    'payment.methods.offline': '私下付款',
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => labels[key] ?? key,
    }),
  }
})

describe('AdminPaymentDashboardView offline distribution', () => {
  beforeEach(() => {
    getDashboard.mockReset().mockResolvedValue({
      data: {
        today_amount: 29,
        total_amount: 145,
        today_count: 1,
        total_count: 5,
        avg_amount: 29,
        daily_series: [],
        payment_methods: [{ type: 'offline', amount: 29, count: 1 }],
        top_users: [],
      },
    })
  })

  it('renders offline payment distribution with a neutral color', async () => {
    const wrapper = mount(AdminPaymentDashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<section><slot /></section>' },
          DailyRevenueChart: true,
          Icon: true,
          LoadingSpinner: true,
          OrderStatsCards: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('私下付款')
    expect(wrapper.text()).toContain('¥29.00')
    expect(wrapper.text()).toContain('(1)')
    expect(wrapper.html()).toContain('bg-slate-500')
  })
})
