import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PurchaseProductCard from '../PurchaseProductCard.vue'
import type { PurchaseProductCardModel } from '../purchaseProductCard'

const productFixture = (overrides: Partial<PurchaseProductCardModel> = {}): PurchaseProductCardModel => ({
  testId: 'purchase-product-card',
  eyebrowText: '订阅',
  title: '28 天订阅套餐A',
  priceLabel: '价格',
  priceText: '¥29.29',
  buttonText: '立即开通',
  detailRows: [
    { label: '周限额', value: '58刀' },
    { label: '有效期', value: '28天' },
    { label: '手续费', value: '无' },
  ],
  ...overrides,
})

describe('PurchaseProductCard', () => {
  it('renders the 18080 store card structure and emits the selected product', async () => {
    const product = productFixture()
    const wrapper = mount(PurchaseProductCard, { props: { product } })

    const card = wrapper.get('[data-testid="purchase-product-card"]')
    const button = wrapper.get('button')

    expect(card.classes()).toEqual(expect.arrayContaining([
      'rounded-2xl',
      'border',
      'bg-white/70',
      'backdrop-blur-xl',
      'shadow-card',
      'dark:bg-dark-800/60',
    ]))
    expect(wrapper.text()).toContain('28 天订阅套餐A')
    expect(wrapper.text()).toContain('周限额58刀')
    expect(button.classes()).toEqual(expect.arrayContaining([
      'btn',
      'btn-primary',
      'py-3',
    ]))

    await button.trigger('click')

    expect(wrapper.emitted('select')?.[0]).toEqual([product])
  })
})
