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
  active: false,
  detailRows: [
    { label: '周限额', value: '58刀' },
    { label: '28 天总额度', value: '232刀' },
    { label: '刷新时间', value: '每周刷新' },
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

    expect(card.classes()).toEqual(expect.arrayContaining([
      'rounded-2xl',
      'border',
      'bg-gradient-to-b',
      'from-white',
      'to-gray-50',
      'text-gray-900',
      'dark:bg-black',
      'dark:from-black',
      'dark:to-black',
    ]))
    expect(text).toContain('订阅')
    expect(text).toContain('28 天订阅套餐A')
    expect(text).toContain('价格')
    expect(text).toContain('¥29.29')
    expect(text).toContain('周限额58刀')
    expect(text).toContain('28 天总额度232刀')
    expect(text).toContain('刷新时间每周刷新')
    expect(text).toContain('手续费详情¥29元 + 1%')
    expect(button.classes()).toEqual(expect.arrayContaining([
      'rounded-full',
      'bg-gray-950',
      'text-white',
      'dark:bg-white',
      'dark:text-black',
      'py-4',
    ]))

    await button.trigger('click')

    expect(wrapper.emitted('select')?.[0]).toEqual([product])
  })

  it('renders traffic cards with the same component and validity row', () => {
    const wrapper = mount(PurchaseProductCard, {
      props: {
        product: productFixture({
          eyebrowText: '流量卡',
          title: '5刀流量卡',
          priceLabel: '价格',
          priceText: '¥2.02',
          buttonText: '立即购买',
          detailRows: [
            { label: '可用额度', value: '5刀' },
            { label: '有效期', value: '365天' },
            { label: '手续费详情', value: '¥2元 + 1%' },
          ],
        }),
      },
    })
    const text = wrapper.text()

    expect(text).toContain('5刀流量卡')
    expect(text).toContain('可用额度5刀')
    expect(text).toContain('有效期365天')
    expect(text).not.toContain('可用范围')
    expect(text).not.toContain('一次性流量包-有效期')
  })
})
