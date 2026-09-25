import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

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

const CODE = 'a3f9c2e17b4d58e0c6a1f93b2d7e4c85'

function publicCard(overrides: Record<string, unknown> = {}) {
  return {
    theme: 'dark',
    code: CODE,
    code_status: 'unused',
    content: { ...emptyRedeemCardContent(), amount: '$10', plan: '余额充值' },
    profile: defaultRedeemCardProfile(),
    ...overrides
  }
}

describe('RedeemCardView', () => {
  beforeEach(() => {
    getPublicRedeemCard.mockReset()
    // jsdom 没有 WebGL：3D 卡会退回平面卡，正好用来测交互。
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
  })

  afterEach(() => {
    vi.restoreAllMocks()
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
    // 截贴图用的卡面：兑换码按原文印在背面，未使用时不盖章，读屏软件不读它。
    const capture = card.get('[data-rc-capture]')
    expect(capture.attributes('aria-hidden')).toBe('true')
    expect(capture.findAll('.rc-face')).toHaveLength(2)
    expect(capture.text().replace(/\s/g, '')).toContain(CODE)
    expect(card.find('.rc-stamp').exists()).toBe(false)
  })

  it('兑换码已兑换时在背面盖章', async () => {
    getPublicRedeemCard.mockResolvedValue(publicCard({ code_status: 'used', theme: 'light' }))
    const wrapper = mount(RedeemCardView)
    await flushPromises()

    expect(wrapper.get('[data-rc-capture] .rc-back .rc-stamp').text()).toBe('已兑换')
  })

  it('WebGL 不可用时退回平面卡：轻点翻面，背面轻点兑换码复制并发绿光', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(window, 'isSecureContext', { value: true, configurable: true })
    Object.defineProperty(window.navigator, 'clipboard', { value: { writeText }, configurable: true })
    getPublicRedeemCard.mockResolvedValue(publicCard())
    const wrapper = mount(RedeemCardView)
    await flushPromises()

    const fallback = wrapper.get('.rc3d-fallback-card')
    expect(fallback.find('.rc-front').exists()).toBe(true)

    await fallback.trigger('click')
    expect(fallback.find('.rc-back').exists()).toBe(true)

    await fallback.get('.rc-code-panel').trigger('click')
    await flushPromises()
    expect(writeText).toHaveBeenCalledWith(CODE)
    expect(fallback.classes()).toContain('rc3d-copied')
    // 复制不翻面。
    expect(fallback.find('.rc-back').exists()).toBe(true)
    Reflect.deleteProperty(window, 'isSecureContext')
    Reflect.deleteProperty(window.navigator, 'clipboard')
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
