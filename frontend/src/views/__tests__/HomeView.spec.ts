import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import HomeView from '@/views/HomeView.vue'

const root = process.cwd()
const readSource = (path: string) => readFileSync(join(root, path), 'utf8')

const mockStores = vi.hoisted(() => ({
  authStore: {
    isAuthenticated: false,
    isAdmin: false,
    user: null as null | { email: string },
    checkAuth: vi.fn(),
  },
  appStore: {
    cachedPublicSettings: null as null | Record<string, string>,
    siteName: 'Sub2API',
    siteLogo: '',
    docUrl: '',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => mockStores.authStore,
  useAppStore: () => mockStores.appStore,
}))

vi.mock('@/components/common/LocaleSwitcher.vue', () => ({
  default: {
    name: 'LocaleSwitcher',
    template: '<div />',
  },
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

const RouterLinkStub = defineComponent({
  name: 'RouterLink',
  props: {
    to: {
      type: [String, Object],
      required: true,
    },
  },
  computed: {
    href(): string {
      const target = this.to as string | { path?: string }
      return typeof target === 'string' ? target : target.path || ''
    },
  },
  template: '<a :href="href"><slot /></a>',
})

const mountHomeView = () => mount(HomeView, {
  global: {
    stubs: {
      RouterLink: RouterLinkStub,
      LocaleSwitcher: true,
      Icon: true,
    },
  },
})

describe('HomeView', () => {
  beforeEach(() => {
    mockStores.authStore.isAuthenticated = false
    mockStores.authStore.isAdmin = false
    mockStores.authStore.user = null
    mockStores.authStore.checkAuth.mockClear()
    mockStores.appStore.cachedPublicSettings = null
    mockStores.appStore.siteName = 'Sub2API'
    mockStores.appStore.siteLogo = ''
    mockStores.appStore.docUrl = ''
    mockStores.appStore.publicSettingsLoaded = true
    mockStores.appStore.fetchPublicSettings.mockClear()
    localStorage.setItem('theme', 'light')
    window.matchMedia = vi.fn().mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })
  })

  it('未登录首页 Hero 区只保留一个主登录入口', () => {
    const wrapper = mountHomeView()
    const loginLinks = wrapper.findAll('a.btn[href="/login"]')

    expect(loginLinks).toHaveLength(1)
  })

  it('默认公共设置只展示天才程序员小站标题', () => {
    mockStores.appStore.cachedPublicSettings = {
      site_name: '天才程序员小站',
      site_subtitle: '',
    }

    const wrapper = mountHomeView()

    expect(wrapper.find('h1').text()).toBe('天才程序员小站')
    expect(wrapper.text()).not.toContain('Subscription to API Conversion Platform')
    expect(wrapper.text()).not.toContain('AI API Gateway')
  })

  it('中文首页文案展示 28 天订阅、周额度和刷新规则', () => {
    const source = readSource('src/i18n/locales/zh.ts')

    expect(source).toContain("getStarted: '立即登录'")
    expect(source).toContain("unifiedGateway: '29 元套餐'")
    expect(source).toContain('28 天订阅，每 7 天刷新，周额度 76 刀')
    expect(source).toContain("multiAccount: '39 元套餐'")
    expect(source).toContain('28 天订阅，每 7 天刷新，周额度 102 刀')
    expect(source).toContain("balanceQuota: '59 元套餐'")
    expect(source).toContain('28 天订阅，每 7 天刷新，周额度 154 刀')
    expect(source).toContain("premiumQuota: '99 元套餐'")
    expect(source).toContain('28 天订阅，每 7 天刷新，周额度 258 刀')
    expect(source).not.toContain("getStarted: '立即开始'")
    expect(source).not.toContain("realtimeBilling: '按量计费'")
    expect(source).not.toContain('使用 cc-switch 项目，一键接入 API 到 Codex')
  })

  it('只展示 GPT-5.6 模型', () => {
    const wrapper = mountHomeView()

    expect(wrapper.text()).toContain('gpt-5.6-luna')
    expect(wrapper.text()).toContain('gpt-5.6-sol')
    expect(wrapper.text()).toContain('gpt-5.6-terra')
    expect(wrapper.text()).not.toContain('GPT 5.3')
    expect(wrapper.text()).not.toContain('Codex 5.4')
    expect(wrapper.text()).not.toContain('GPT 5.5')
  })

  it('完整展示订阅套餐与流量卡价格', () => {
    const wrapper = mountHomeView()
    const priceList = wrapper.get('[data-testid="home-price-list"]')

    expect(priceList.findAll('[data-testid="home-product-card"]')).toHaveLength(13)
    for (const price of ['¥29', '¥39', '¥49', '¥59', '¥79', '¥99', '¥149', '¥199', '¥249', '¥299']) {
      expect(priceList.text()).toContain(price)
    }
    expect(priceList.text()).toContain('5 刀额度')
    expect(priceList.text()).toContain('¥2')
    expect(priceList.text()).toContain('10 刀额度')
    expect(priceList.text()).toContain('¥3')
    expect(priceList.text()).toContain('20 刀额度')
    expect(priceList.text()).toContain('¥5')
    expect(priceList.text()).toContain('365 天有效')
  })

  it('商品卡按登录状态前往登录页或购买页', () => {
    const loggedOut = mountHomeView()
    const loggedOutLinks = loggedOut.findAll('[data-testid="home-product-link"]')
    expect(loggedOutLinks).toHaveLength(13)
    loggedOutLinks.forEach(link => expect(link.attributes('href')).toBe('/login'))

    mockStores.authStore.isAuthenticated = true
    const loggedIn = mountHomeView()
    loggedIn.findAll('[data-testid="home-product-link"]').forEach(link => expect(link.attributes('href')).toBe('/purchase'))
  })
})
