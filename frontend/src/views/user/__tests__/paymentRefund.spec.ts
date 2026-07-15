import { describe, expect, it } from 'vitest'
import type { PaymentOrder } from '@/types/payment'
import { canRequestOrderRefund } from '../paymentRefund'

const baseOrder = (overrides: Partial<PaymentOrder>): PaymentOrder => ({
  id: 1,
  user_id: 1,
  amount: 29,
  pay_amount: 29.29,
  fee_rate: 1,
  payment_type: 'alipay',
  out_trade_no: 'sub2_test',
  status: 'COMPLETED',
  order_type: 'subscription',
  created_at: '2026-07-09T00:00:00Z',
  expires_at: '2026-07-09T01:00:00Z',
  refund_amount: 0,
  ...overrides,
})

describe('canRequestOrderRefund', () => {
  it('shows refund for completed alipay subscription with eligible provider', () => {
    expect(canRequestOrderRefund(baseOrder({ provider_instance_id: '1' }), new Set(['1']))).toBe(true)
  })

  it('shows refund for completed balance subscription without provider instance', () => {
    expect(canRequestOrderRefund(baseOrder({ payment_type: 'balance', provider_instance_id: undefined }), new Set())).toBe(true)
  })

  it('hides refund for traffic packs', () => {
    expect(canRequestOrderRefund(baseOrder({ order_type: 'traffic_pack', provider_instance_id: '1' }), new Set(['1']))).toBe(false)
  })

  it('shows retry when a failed refund is explicitly retryable', () => {
    expect(canRequestOrderRefund(baseOrder({ status: 'REFUND_FAILED', refund_retryable: true }), new Set())).toBe(true)
  })

  it('hides retry for unknown or pending refund results', () => {
    expect(canRequestOrderRefund(baseOrder({ status: 'REFUND_FAILED', refund_retryable: false }), new Set())).toBe(false)
    expect(canRequestOrderRefund(baseOrder({ status: 'REFUNDING', refund_retryable: true }), new Set())).toBe(false)
  })
})
