import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SubscriptionsView from '../SubscriptionsView.vue'

const { getMyBalancePackages, getMySubscriptions, showError, routerPush } = vi.hoisted(() => ({
  getMyBalancePackages: vi.fn(),
  getMySubscriptions: vi.fn(),
  showError: vi.fn(),
  routerPush: vi.fn()
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: { getMyBalancePackages }
}))

vi.mock('@/api/subscriptions', () => ({
  default: { getMySubscriptions }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    cachedPublicSettings: null
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'userSubscriptions.balancePackagesTitle': '余额套餐',
    'userSubscriptions.balancePackagesDesc': '套餐到账进度',
    'userSubscriptions.balancePackageValidity': '{days} 天有效',
    'userSubscriptions.weeklyRemaining': '本周剩余额度',
    'userSubscriptions.creditedProgress': '到账进度',
    'userSubscriptions.balancePackageProgress': '套餐周期进度',
    'userSubscriptions.expires': '到期时间',
    'userSubscriptions.nextRefresh': '下次刷新',
    'userSubscriptions.refreshCompleted': '本周期已完成，不再刷新',
    'userSubscriptions.status.active': '有效',
    'userSubscriptions.status.completed': '已到账',
    'userSubscriptions.status.expired': '已过期',
    'userSubscriptions.status.refunded': '已退款',
    'userSubscriptions.status.debt_paused': '欠费暂停',
    'userSubscriptions.buyAgain': '再次购买',
    'userSubscriptions.daysRemaining': '剩余 {days} 天',
    'userSubscriptions.renewedBadge': '已续费 ×{count}',
    'userSubscriptions.renewalExtended': '续费已重置周期，有效期延长至 {date}',
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        let message = messages[key] ?? key
        for (const [name, value] of Object.entries(params ?? {})) {
          message = message.replace(`{${name}}`, String(value))
        }
        return message
      }
    })
  }
})

vi.mock('@/utils/format', () => ({
  formatDateTimeToMinute: () => '2030-01-08 12:30'
}))

const futureDate = '2030-02-01T00:00:00.000Z'

function balancePackage(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    plan_id: 1,
    code: 'balance-49',
    name: '余额套餐 ¥49',
    price_cny: 49,
    weekly_credit_usd: 128,
    current_remaining_usd: 43.25,
    validity_days: 28,
    refresh_count: 4,
    refresh_interval_days: 7,
    credited_count: 2,
    renewal_count: 0,
    starts_at: '2030-01-01T00:00:00.000Z',
    next_credit_at: '2030-01-08T12:30:00.000Z',
    expires_at: futureDate,
    status: 'active',
    created_at: '2030-01-01T00:00:00.000Z',
    updated_at: '2030-01-01T00:00:00.000Z',
    ...overrides
  }
}

function mountView() {
  return mount(SubscriptionsView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: true
      }
    }
  })
}

describe('SubscriptionsView', () => {
  beforeEach(() => {
    getMySubscriptions.mockReset()
    getMyBalancePackages.mockReset()
    showError.mockReset()
    routerPush.mockReset()
    getMySubscriptions.mockResolvedValue([])
  })

  it('显示本周套餐剩余额度与下次刷新时间', async () => {
    getMyBalancePackages.mockResolvedValue({ data: [balancePackage()] })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('本周剩余额度')
    expect(wrapper.text()).toContain('$43.25')
    expect(wrapper.text()).toContain('下次刷新')
    expect(wrapper.text()).toContain('2030-01-08 12:30')
  })

  it('套餐周期完成后明确提示不再刷新', async () => {
    getMyBalancePackages.mockResolvedValue({
      data: [balancePackage({ status: 'completed', credited_count: 4, next_credit_at: undefined })]
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('本周期已完成，不再刷新')
    expect(wrapper.text()).not.toContain('下次刷新')
  })

  it('续费后展示续费标识与有效期延长提示', async () => {
    getMyBalancePackages.mockResolvedValue({
      data: [balancePackage({ renewal_count: 2, credited_count: 1 })]
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('已续费 ×2')
    expect(wrapper.text()).toContain('续费已重置周期，有效期延长至')
  })

  it('未续费套餐不展示续费标识', async () => {
    getMyBalancePackages.mockResolvedValue({ data: [balancePackage()] })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).not.toContain('已续费')
  })

  it('历史欠费暂停状态不再显示过时提示并展示下次刷新', async () => {
    getMyBalancePackages.mockResolvedValue({
      data: [balancePackage({ status: 'debt_paused', current_remaining_usd: 0 })]
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('欠费暂停')
    expect(wrapper.text()).not.toContain('首周额度不足以抵销欠费，后续额度已暂停，请联系管理员')
    expect(wrapper.text()).toContain('下次刷新')
    expect(wrapper.text()).not.toContain('原计划刷新时间')
  })
})
