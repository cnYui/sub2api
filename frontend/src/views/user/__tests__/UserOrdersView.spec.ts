import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UserOrdersView from '../UserOrdersView.vue'

const getMyOrders = vi.hoisted(() => vi.fn())
const getRefundEligibleProviders = vi.hoisted(() => vi.fn())
const getRefundQuote = vi.hoisted(() => vi.fn())
const cancelOrder = vi.hoisted(() => vi.fn())
const requestRefund = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const routerPush = vi.hoisted(() => vi.fn())

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    getMyOrders,
    getRefundEligibleProviders,
    getRefundQuote,
    cancelOrder,
    requestRefund,
  },
}))

const order = {
  id: 11,
  user_id: 7,
  amount: 29,
  pay_amount: 30.45,
  currency: 'CNY',
  fee_rate: 0.05,
  payment_type: 'alipay',
  provider_instance_id: 'zpay-main',
  out_trade_no: 'sub2-11',
  status: 'COMPLETED',
  order_type: 'subscription',
  created_at: '2026-07-20T00:00:00+08:00',
  expires_at: '2026-07-20T00:30:00+08:00',
  refund_amount: 0,
}

function mountView() {
  return mount(UserOrdersView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: { template: '<span />' },
        Select: { props: ['modelValue', 'options'], template: '<div />' },
        Pagination: { template: '<div />' },
        BaseDialog: {
          props: ['show'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>',
        },
        OrderTable: {
          props: ['orders'],
          template: `
            <div>
              <div v-for="row in orders" :key="row.id">
                <slot name="actions" :row="row" />
              </div>
            </div>
          `,
        },
      },
    },
  })
}

describe('UserOrdersView', () => {
  beforeEach(() => {
    getMyOrders.mockReset()
    getRefundEligibleProviders.mockReset()
    getRefundQuote.mockReset()
    cancelOrder.mockReset()
    requestRefund.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    routerPush.mockReset()

    getMyOrders.mockResolvedValue({
      data: { items: [order], total: 1 },
    })
    getRefundEligibleProviders.mockResolvedValue({
      data: { provider_instance_ids: ['zpay-main'] },
    })
    getRefundQuote.mockResolvedValue({
      data: {
        eligible: true,
        manual_review_required: false,
        entitlement_period_id: 101,
        purchase_base_amount: 29,
        non_refundable_fee: 1.45,
        period_total_quota_usd: 288,
        used_quota_usd: 36.2,
        usage_ratio: 0.125,
        estimated_refund_amount: 25.375,
        calculated_at: '2026-07-20T01:00:00+08:00',
      },
    })
  })

  it('退款报价中订单金额使用订单币种，额度使用 USD 整数展示', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('button.text-purple-600').trigger('click')
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('¥29.00')
    expect(text).toContain('¥1.45')
    expect(text).toContain('¥25.38')
    expect(text).toContain('$36 / $288')
    expect(text).not.toContain('$29.00')
  })
})
