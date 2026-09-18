import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { BalancePackageRefundQuote, PaymentOrder } from '@/types/payment'
import AdminOrderDetail from '../AdminOrderDetail.vue'
import AdminOrderTable from '../AdminOrderTable.vue'
import AdminRefundDialog from '../AdminRefundDialog.vue'
import OrderTable from '@/components/payment/OrderTable.vue'

const { getRefundQuote } = vi.hoisted(() => ({ getRefundQuote: vi.fn() }))

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: { getRefundQuote },
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

const BaseDialogStub = {
  props: ['show'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

const DataTableStub = {
  props: ['data'],
  template: `
    <div>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-pay_amount" :value="row.pay_amount" :row="row" />
      </div>
    </div>
  `,
}

function orderFactory(overrides: Partial<PaymentOrder> = {}): PaymentOrder {
  return {
    id: 1,
    user_id: 10,
    amount: 100,
    pay_amount: 108,
    currency: 'USD',
    fee_rate: 8,
    payment_type: 'stripe',
    out_trade_no: 'sub2_202606250001',
    status: 'COMPLETED',
    order_type: 'balance_subscription',
    created_at: '2026-06-25T10:00:00Z',
    expires_at: '2026-06-25T10:30:00Z',
    refund_amount: 25,
    ...overrides,
  }
}

function quoteFactory(overrides: Partial<BalancePackageRefundQuote> = {}): BalancePackageRefundQuote {
  return {
    eligible: true,
    manual_review_required: false,
    purchase_base_amount: 100,
    non_refundable_fee: 8,
    period_total_quota_usd: 512,
    used_quota_usd: 128,
    retained_quota_usd: 0,
    reclaim_quota_usd: 0,
    usage_ratio: 0.25,
    time_ratio: 0.01,
    consumption_ratio: 0.25,
    estimated_refund_amount: 75,
    calculated_at: '2026-09-17T10:00:00Z',
    ...overrides,
  }
}

function mountRefundDialog(order: PaymentOrder) {
  return mount(AdminRefundDialog, {
    props: { show: true, order },
    global: { stubs: { BaseDialog: BaseDialogStub } },
  })
}

beforeEach(() => {
  getRefundQuote.mockReset()
})

describe('admin order currency display', () => {
  it('uses order currency for paid/base/fee amounts and USD for credited/refund amounts', () => {
    const wrapper = mount(AdminOrderDetail, {
      props: {
        show: true,
        order: orderFactory({ currency: 'CNY' }),
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('¥100.00')
    expect(text).toContain('¥8.00')
    expect(text).toContain('¥108.00')
    expect(text).toContain('$100.00')
    expect(text).toContain('$25.00')
  })

  it('uses order currency for pay_amount and USD for the server-calculated refund amount', async () => {
    getRefundQuote.mockResolvedValue({ data: quoteFactory({ estimated_refund_amount: 60 }) })
    const wrapper = mountRefundDialog(orderFactory({
      currency: 'USD',
      status: 'REFUND_FAILED',
      refund_amount: 80,
    }))
    await flushPromises()

    const text = wrapper.text()
    expect(getRefundQuote).toHaveBeenCalledWith(1)
    expect(text).toContain('$108.00')
    expect(text).toContain('$100.00')
    // 实时报价是真正要退的金额，订单上的旧金额只作参考。
    expect(text).toContain('$60.00')
    expect(text).toContain('payment.admin.lastAttemptedRefund')
    expect(text).toContain('$80.00')
  })

  it('shows the weekly quota reclaimed from the balance and allows confirming', async () => {
    getRefundQuote.mockResolvedValue({ data: quoteFactory({ reclaim_quota_usd: 46.19 }) })
    const wrapper = mountRefundDialog(orderFactory({ refund_amount: 0 }))
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('payment.refundQuote.reclaimQuota')
    expect(text).toContain('$46.19')
    expect(text).toContain('payment.admin.refundReclaimNotice')
    expect(text).not.toContain('payment.admin.lastAttemptedRefund')
    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })

  it('blocks confirming when the quote needs manual review', async () => {
    getRefundQuote.mockResolvedValue({ data: quoteFactory({ manual_review_required: true, estimated_refund_amount: 0 }) })
    const wrapper = mountRefundDialog(orderFactory({ status: 'REFUND_FAILED', refund_amount: 89.1 }))
    await flushPromises()

    expect(wrapper.text()).toContain('payment.refundQuote.manualReviewRequired')
    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeDefined()
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('confirm')).toBeUndefined()
  })

  it('blocks confirming when the quote cannot be loaded', async () => {
    getRefundQuote.mockRejectedValue(new Error('network down'))
    const wrapper = mountRefundDialog(orderFactory())
    await flushPromises()

    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeDefined()
    expect(wrapper.emitted('confirm')).toBeUndefined()
  })

  it('renders payment currency consistently in the shared order table', () => {
    const wrapper = mount(OrderTable, {
      props: {
        orders: [
          orderFactory({ id: 1, currency: 'USD', amount: 100, pay_amount: 108 }),
          orderFactory({ id: 2, currency: 'CNY', amount: 100, pay_amount: 108 }),
        ],
        loading: false,
        showUser: true,
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          OrderStatusBadge: true,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('$108.00')
    expect(text).toContain('¥108.00')
    expect(text).toContain('$100.00')
  })

  it('renders payment currency consistently in the admin order table', () => {
    const wrapper = mount(AdminOrderTable, {
      props: {
        orders: [
          orderFactory({ id: 1, currency: 'USD', amount: 100, pay_amount: 108 }),
          orderFactory({ id: 2, currency: 'CNY', amount: 100, pay_amount: 108 }),
        ],
        loading: false,
        page: 1,
        pageSize: 20,
        total: 2,
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          Icon: true,
          Pagination: true,
          Select: true,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('$108.00')
    expect(text).toContain('¥108.00')
    expect(text).toContain('$100.00')
  })
})
