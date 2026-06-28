import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PurchaseProductCard from '../PurchaseProductCard.vue'
import type { PurchaseProductCardModel } from '../purchaseProductCard'

const productFixture = (overrides: Partial<PurchaseProductCardModel> = {}): PurchaseProductCardModel => ({
  testId: 'purchase-product-card',
  title: '阅读订阅套餐A',
  priceText: '¥29.29',
  buttonText: '立即开通',
  active: false,
  detailRows: [
    { label: '日限额', value: '19刀' },
    { label: '刷新时间', value: '24点刷新' },
    { label: '手续费详情', value: '¥29元 + 1%' },
  ],
  ...overrides,
})

describe('PurchaseProductCard', () => {
  it('renders the shared iLiquid product card for subscription products', async () => {
    const product = productFixture()
    const wrapper = mount(PurchaseProductCard, {
      props: { product },
    })

    const card = wrapper.get('[data-testid="purchase-product-card"]')
    const button = wrapper.get('button')
    const text = wrapper.text()

    expect(card.classes()).toEqual(expect.arrayContaining(['rounded-2xl', 'border', 'bg-black']))
    expect(text).toContain('PLAN')
    expect(text).toContain('阅读订阅套餐A')
    expect(text).toContain('Price')
    expect(text).toContain('¥29.29')
    expect(text).toContain('日限额19刀')
    expect(text).toContain('刷新时间24点刷新')
    expect(text).toContain('手续费详情¥29元 + 1%')
    expect(button.classes()).toEqual(expect.arrayContaining(['rounded-full', 'bg-white', 'py-4']))

    await button.trigger('click')

    expect(wrapper.emitted('select')?.[0]).toEqual([product])
  })

  it('renders traffic cards with the same component and refresh-time row', () => {
    const wrapper = mount(PurchaseProductCard, {
      props: {
        product: productFixture({
          title: '5刀流量卡',
          priceText: '¥2.02',
          buttonText: '立即购买',
          detailRows: [
            { label: '可用额度', value: '5刀' },
            { label: '刷新时间', value: '365天' },
            { label: '手续费详情', value: '¥2元 + 1%' },
          ],
        }),
      },
    })
    const text = wrapper.text()

    expect(text).toContain('5刀流量卡')
    expect(text).toContain('可用额度5刀')
    expect(text).toContain('刷新时间365天')
    expect(text).not.toContain('可用范围')
    expect(text).not.toContain('一次性流量包-有效期')
  })
})
