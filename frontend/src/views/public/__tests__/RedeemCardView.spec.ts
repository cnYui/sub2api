import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import RedeemCardView from '../RedeemCardView.vue'
import { defaultRedeemCardProfile, emptyRedeemCardContent } from '@/components/redeemCard/redeemCardModel'

const { getPublicRedeemCard } = vi.hoisted(() => ({
  getPublicRedeemCard: vi.fn()
}))

vi.mock('@/api/redeemCards', () => ({ getPublicRedeemCard }))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { token: 'a'.repeat(32) } })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'redeemCardPage.notFound': '链接无效或已被撤销',
    'redeemCardPage.loadFailed': '加载失败，请刷新重试'
  }
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => messages[key] ?? key })
  }
})

function publicCard(overrides: Record<string, unknown> = {}) {
  return {
    theme: 'dark',
    code: 'a3f9c2e17b4d58e0c6a1f93b2d7e4c85',
    code_status: 'unused',
    content: { ...emptyRedeemCardContent(), amount: '$10', plan: '余额充值' },
    profile: defaultRedeemCardProfile(),
    ...overrides
  }
}

describe('RedeemCardView', () => {
  beforeEach(() => {
    getPublicRedeemCard.mockReset()
  })

  it('页面上只有卡片：卡面之外没有任何文字', async () => {
    getPublicRedeemCard.mockResolvedValue(publicCard())
    const wrapper = mount(RedeemCardView)
    await flushPromises()

    expect(getPublicRedeemCard).toHaveBeenCalledWith('a'.repeat(32))
    const page = wrapper.get('main')
    const card = page.get('.rcv-card')
    expect(page.text()).toBe(card.text())
    expect(page.find('.rcv-error').exists()).toBe(false)
    // 兑换码按原文印在背面，未使用时不盖章。
    expect(card.text().replace(/\s/g, '')).toContain('a3f9c2e17b4d58e0c6a1f93b2d7e4c85')
    expect(card.find('.rc-stamp').exists()).toBe(false)
  })

  it('厚度 0.6mm：两面各离中面 6 个设计像素，侧边小条高 12 个设计像素', async () => {
    getPublicRedeemCard.mockResolvedValue(publicCard())
    const wrapper = mount(RedeemCardView)
    await flushPromises()

    const faces = wrapper.findAll('.rc3d-face')
    expect(faces).toHaveLength(2)
    expect(faces[0].attributes('style')).toContain('translateZ(6px)')
    expect(faces[1].attributes('style')).toContain('rotateY(180deg) translateZ(6px)')
    const edges = wrapper.findAll('.rc3d-edge')
    expect(edges.length).toBeGreaterThan(50)
    expect(edges.every((edge) => (edge.attributes('style') ?? '').includes('height: 12px'))).toBe(true)
  })

  it('兑换码已兑换时在背面盖章', async () => {
    getPublicRedeemCard.mockResolvedValue(publicCard({ code_status: 'used', theme: 'light' }))
    const wrapper = mount(RedeemCardView)
    await flushPromises()

    expect(wrapper.get('main').classes()).toContain('rcv-light')
    expect(wrapper.get('.rc-stamp').text()).toBe('已兑换')
  })

  it('链接被撤销时只显示一句说明', async () => {
    getPublicRedeemCard.mockRejectedValue({ status: 404 })
    const wrapper = mount(RedeemCardView)
    await flushPromises()

    expect(wrapper.find('.rcv-card').exists()).toBe(false)
    expect(wrapper.get('.rcv-error').text()).toBe('链接无效或已被撤销')
  })

  it('网络或服务端错误时提示刷新', async () => {
    getPublicRedeemCard.mockRejectedValue({ status: 0 })
    const wrapper = mount(RedeemCardView)
    await flushPromises()

    expect(wrapper.get('.rcv-error').text()).toBe('加载失败，请刷新重试')
  })
})
