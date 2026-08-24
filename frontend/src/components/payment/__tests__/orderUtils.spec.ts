import { describe, expect, it } from 'vitest'
import { isRealPaidBalancePackageOrder } from '@/components/payment/orderUtils'
import type { PaymentOrder } from '@/types/payment'

function order(overrides: Partial<PaymentOrder> = {}): PaymentOrder {
  return {
    id: 1,
    user_id: 1,
    amount: 29,
    pay_amount: 29,
    fee_rate: 0,
    payment_type: 'alipay',
    out_trade_no: 'order-1',
    status: 'COMPLETED',
    order_type: 'balance_subscription',
    created_at: '2026-08-24T00:00:00Z',
    expires_at: '2026-09-21T00:00:00Z',
    paid_at: '2026-08-24T00:01:00Z',
    refund_amount: 0,
    ...overrides,
  }
}

describe('isRealPaidBalancePackageOrder', () => {
  it('accepts a user-paid balance package', () => {
    expect(isRealPaidBalancePackageOrder(order())).toBe(true)
  })

  it.each([
    ['traffic pack', { order_type: 'traffic_pack' as const }],
    ['administrator grant', { payment_type: 'admin_grant' }],
    ['redeem code', { payment_type: 'redeem_code' }],
    ['zero payment', { pay_amount: 0 }],
    ['unpaid order', { paid_at: undefined }],
  ])('rejects %s', (_name, overrides) => {
    expect(isRealPaidBalancePackageOrder(order(overrides))).toBe(false)
  })
})
