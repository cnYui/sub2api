import { describe, expect, it } from 'vitest'
import {
  buildPaymentErrorToastMessage,
  describePaymentScenarioError,
  normalizePaymentMethodForDisplay,
} from '../paymentUx'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

const zhMessages = zh as {
  payment: {
    errors: Record<string, string>
  }
}

const enMessages = en as {
  payment: {
    errors: Record<string, string>
  }
}

describe('normalizePaymentMethodForDisplay', () => {
  it('collapses visible payment aliases to canonical method ids', () => {
    expect(normalizePaymentMethodForDisplay(' alipay_direct ')).toBe('alipay')
    expect(normalizePaymentMethodForDisplay('wxpay_direct')).toBe('wxpay')
    expect(normalizePaymentMethodForDisplay('wechat_pay')).toBe('wxpay')
  })

  it('leaves non-aliased methods untouched', () => {
    expect(normalizePaymentMethodForDisplay('stripe')).toBe('stripe')
  })
})

describe('describePaymentScenarioError', () => {
  it('maps WeChat H5 authorization errors to explicit in-app guidance', () => {
    expect(describePaymentScenarioError(
      { reason: 'WECHAT_H5_NOT_AUTHORIZED' },
      { paymentMethod: 'wxpay', isMobile: true, isWechatBrowser: false },
    )).toEqual({
      messageKey: 'payment.errors.wechatH5NotAuthorized',
      hintKey: 'payment.errors.wechatOpenInWeChatHint',
    })
  })

  it('maps WeChat H5 authorization errors when provider aliases use wxpay_direct', () => {
    expect(describePaymentScenarioError(
      { reason: 'WECHAT_H5_NOT_AUTHORIZED' },
      { paymentMethod: 'wxpay_direct', isMobile: true, isWechatBrowser: false },
    )).toEqual({
      messageKey: 'payment.errors.wechatH5NotAuthorized',
      hintKey: 'payment.errors.wechatOpenInWeChatHint',
    })
  })

  it('maps missing WeixinJSBridge to a JSAPI-specific prompt', () => {
    expect(describePaymentScenarioError(
      new Error('WeixinJSBridge is unavailable'),
      { paymentMethod: 'wxpay', isMobile: true, isWechatBrowser: true },
    )).toEqual({
      messageKey: 'payment.errors.wechatJsapiUnavailable',
      hintKey: 'payment.errors.wechatOpenInWeChatHint',
    })
  })

  it('maps the internal JSAPI unavailable marker to the same prompt', () => {
    expect(describePaymentScenarioError(
      new Error('WECHAT_JSAPI_UNAVAILABLE'),
      { paymentMethod: 'wxpay', isMobile: true, isWechatBrowser: true },
    )).toEqual({
      messageKey: 'payment.errors.wechatJsapiUnavailable',
      hintKey: 'payment.errors.wechatOpenInWeChatHint',
    })
  })

  it('maps generic desktop Alipay failures to QR guidance', () => {
    expect(describePaymentScenarioError(
      { reason: 'PAYMENT_GATEWAY_ERROR' },
      { paymentMethod: 'alipay', isMobile: false, isWechatBrowser: false },
    )).toEqual({
      messageKey: 'payment.errors.alipayDesktopUnavailable',
      hintKey: 'payment.errors.alipayDesktopQrHint',
    })
  })
})

describe('buildPaymentErrorToastMessage', () => {
  it('returns the main message when no hint is present', () => {
    expect(buildPaymentErrorToastMessage('Payment failed')).toBe('Payment failed')
  })

  it('appends the hint to the toast body when present', () => {
    expect(buildPaymentErrorToastMessage('Payment failed', 'Open WeChat to continue.')).toBe(
      'Payment failed Open WeChat to continue.'
    )
  })
})

describe('payment subscription guard copy', () => {
  it('uses self-service prorated refund guidance for subscription switches', () => {
    const zhCopy = '仅可续费当前套餐；购买新套餐前，请在“我的订单”按比例退款。'
    const enCopy = 'You can only renew your current plan. Before buying a new plan, request a prorated refund in My Orders.'

    expect(zhMessages.payment.errors.ACTIVE_SUBSCRIPTION_EXISTS).toBe(zhCopy)
    expect(zhMessages.payment.errors.ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND).toBe(zhCopy)
    expect(enMessages.payment.errors.ACTIVE_SUBSCRIPTION_EXISTS).toBe(enCopy)
    expect(enMessages.payment.errors.ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND).toBe(enCopy)
  })
})
