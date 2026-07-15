import { describe, expect, it } from 'vitest'
import { canRefund } from '../orderUtils'

describe('canRefund', () => {
  it('allows explicitly retryable failed refunds', () => {
    expect(canRefund('REFUND_FAILED', true)).toBe(true)
  })

  it('blocks unknown or pending failed refunds', () => {
    expect(canRefund('REFUND_FAILED', false)).toBe(false)
    expect(canRefund('REFUNDING', true)).toBe(false)
  })
})
