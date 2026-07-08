import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PaymentMethodSelector from '../PaymentMethodSelector.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('PaymentMethodSelector', () => {
  it('余额支付按钮不显示手续费副文案', () => {
    const wrapper = mount(PaymentMethodSelector, {
      props: {
        selected: 'alipay',
        methods: [
          { type: 'alipay', fee_rate: 1, available: true },
          { type: 'balance', fee_rate: 1, available: true },
        ],
      },
    })

    const alipay = wrapper.get('[data-testid="payment-method-alipay"]')
    const balance = wrapper.get('[data-testid="payment-method-balance"]')

    expect(alipay.text()).toContain('payment.fee')
    expect(alipay.text()).toContain('1%')
    expect(balance.text()).toContain('payment.methods.balance')
    expect(balance.text()).not.toContain('payment.fee')
    expect(balance.text()).not.toContain('1%')
  })
})
