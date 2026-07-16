import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AdminOrdersView from '../AdminOrdersView.vue'

const getOrders = vi.hoisted(() => vi.fn())
const getOrder = vi.hoisted(() => vi.fn())
const cancelOrder = vi.hoisted(() => vi.fn())
const retryRecharge = vi.hoisted(() => vi.fn())
const refundOrder = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    getOrders,
    getOrder,
    cancelOrder,
    retryRecharge,
    refundOrder,
  },
  default: {
    getOrders,
    getOrder,
    cancelOrder,
    retryRecharge,
    refundOrder,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const labels: Record<string, string> = {
    'payment.admin.allPaymentTypes': '全部支付方式',
    'payment.methods.offline': '私下付款',
    'payment.methods.alipay': '支付宝',
    'payment.methods.wxpay': '微信支付',
    'payment.methods.stripe': 'Stripe',
    'payment.methods.airwallex': 'Airwallex',
    'payment.admin.refund': '退款',
    'common.view': '查看',
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => labels[key] ?? key,
    }),
  }
})

const order = () => ({
  id: 1,
  user_id: 3,
  amount: 29,
  pay_amount: 29,
  currency: 'CNY',
  fee_rate: 0,
  payment_type: 'offline',
  out_trade_no: 'offline_paid_backfill_20260716_s2',
  status: 'COMPLETED',
  order_type: 'subscription',
  created_at: '2026-07-16T04:08:33.371Z',
  expires_at: '2026-07-16T04:08:33.371Z',
  refund_amount: 0,
})

const mountView = () => mount(AdminOrdersView, {
  global: {
    stubs: {
      AppLayout: { template: '<section><slot /></section>' },
      BaseDialog: { template: '<div><slot /></div>' },
      AdminRefundDialog: true,
      Icon: true,
      OrderStatusBadge: true,
      Pagination: true,
      Select: {
        props: ['options'],
        template: '<div><span v-for="option in options" :key="option.value">{{ option.label }}</span></div>',
      },
      OrderTable: {
        props: ['orders'],
        template: `
          <div>
            <div v-for="row in orders" :key="row.id">
              <span>{{ row.payment_type }}</span>
              <slot name="actions" :row="row" />
            </div>
          </div>
        `,
      },
    },
  },
})

describe('AdminOrdersView offline payment records', () => {
  beforeEach(() => {
    getOrders.mockReset().mockResolvedValue({ data: { items: [], total: 0 } })
    getOrder.mockReset()
    cancelOrder.mockReset()
    retryRecharge.mockReset()
    refundOrder.mockReset()
  })

  it('shows offline as an admin payment type filter option', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('私下付款')
  })

  it('hides refund actions for completed offline orders', async () => {
    getOrders.mockResolvedValueOnce({ data: { items: [order()], total: 1 } })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('offline')
    expect(wrapper.text()).toContain('查看')
    expect(wrapper.text()).not.toContain('退款')
  })
})
