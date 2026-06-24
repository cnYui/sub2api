import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TrafficPackCard from '../TrafficPackCard.vue'
import type { TrafficPack } from '@/types/payment'

const packFixture = (overrides: Partial<TrafficPack> = {}): TrafficPack => ({
  id: 1,
  code: 'gpt_traffic_5usd_2cny',
  name: 'GPT 流量包 5 刀',
  description: '2 元购买 5 USD GPT 额度，有效期 365 天，可用于写代码和生图。',
  price: 2,
  credit_usd: 5,
  validity_days: 365,
  platform: 'openai',
  for_sale: true,
  sort_order: 10,
  ...overrides,
})

describe('TrafficPackCard', () => {
  it('uses subscription card surface and button styling', () => {
    const wrapper = mount(TrafficPackCard, {
      props: {
        pack: packFixture(),
      },
    })
    const card = wrapper.get('[data-testid="traffic-pack-card"]')
    const accent = wrapper.get('[data-testid="traffic-pack-accent"]')
    const button = wrapper.get('button')

    expect(card.classes()).toEqual(expect.arrayContaining(['rounded-lg', 'border', 'overflow-hidden']))
    expect(accent.classes()).toContain('h-1.5')
    expect(button.classes()).toEqual(expect.arrayContaining(['rounded-xl', 'py-2.5']))
    expect(wrapper.text()).toContain('OpenAI')
    expect(wrapper.text()).toContain('¥2元')
    expect(wrapper.text()).toContain('一次性流量包-有效期 365天，额度 5刀')
  })

  it('emits select when buying', async () => {
    const pack = packFixture()
    const wrapper = mount(TrafficPackCard, {
      props: { pack },
    })

    await wrapper.get('button').trigger('click')

    expect(wrapper.emitted('select')?.[0]).toEqual([pack])
  })
})
