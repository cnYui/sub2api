import { describe, expect, it } from 'vitest'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'
import { canRefund, isAutomaticRefundAllowed } from '../orderUtils'

describe('canRefund', () => {
  it('allows explicitly retryable failed refunds', () => {
    expect(canRefund('REFUND_FAILED', true)).toBe(true)
  })

  it('blocks unknown or pending failed refunds', () => {
    expect(canRefund('REFUND_FAILED', false)).toBe(false)
    expect(canRefund('REFUNDING', true)).toBe(false)
  })
})

describe('isAutomaticRefundAllowed', () => {
  it('blocks offline automatic refunds', () => {
    expect(isAutomaticRefundAllowed('offline')).toBe(false)
    expect(isAutomaticRefundAllowed('alipay')).toBe(true)
    expect(isAutomaticRefundAllowed('balance')).toBe(true)
  })
})

describe('payment method locale labels', () => {
  it('labels offline and manual grant payment records', () => {
    expect(zh.payment.methods.offline).toBe('私下付款')
    expect(zh.payment.methods.manual_grant).toBe('赠送金额')
    expect(en.payment.methods.offline).toBe('Offline payment')
    expect(en.payment.methods.manual_grant).toBe('Gift amount')
  })
})
