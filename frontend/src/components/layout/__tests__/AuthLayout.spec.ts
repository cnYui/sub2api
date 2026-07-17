import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AuthLayout from '@/components/layout/AuthLayout.vue'

const fetchPublicSettingsMock = vi.hoisted(() => vi.fn())

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    siteName: '天才程序员小站',
    siteLogo: '',
    cachedPublicSettings: { site_subtitle: 'AI API Gateway' },
    publicSettingsLoaded: true,
    fetchPublicSettings: fetchPublicSettingsMock,
  }),
}))

describe('AuthLayout', () => {
  it('默认不渲染世界地图背景', () => {
    const wrapper = mount(AuthLayout)

    expect(wrapper.find('[data-testid="auth-world-map-background"]').exists()).toBe(false)
  })

  it('显式启用后渲染地图并使用固定深色背景', () => {
    const wrapper = mount(AuthLayout, {
      props: { worldMapBackground: true },
    })

    expect(wrapper.find('[data-testid="auth-world-map-background"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="auth-layout"]').classes()).toContain('bg-[#0a0a12]')
    expect(wrapper.get('[data-testid="auth-brand-title"]').classes()).toContain('text-gray-100')
  })
})
