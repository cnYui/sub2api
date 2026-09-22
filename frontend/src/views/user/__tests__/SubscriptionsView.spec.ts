import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SubscriptionsView from '../SubscriptionsView.vue'

const {
  getMyBalancePackages,
  getMySubscriptions,
  creditNextEarlyBalancePackage,
  showError,
  showSuccess,
  refreshUser,
  routerPush
} = vi.hoisted(() => ({
  getMyBalancePackages: vi.fn(),
  getMySubscriptions: vi.fn(),
  creditNextEarlyBalancePackage: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  refreshUser: vi.fn(),
  routerPush: vi.fn()
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: { getMyBalancePackages, creditNextEarlyBalancePackage }
}))

vi.mock('@/api/subscriptions', () => ({
  default: { getMySubscriptions }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    cachedPublicSettings: null
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ refreshUser })
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
    'userSubscriptions.earlyRefresh': '提前刷新',
    'userSubscriptions.earlyRefreshTitle': '提前刷新下一期额度',
    'userSubscriptions.earlyRefreshMessage': '将立即发放第 {next} / {total} 期额度 ${amount}',
    'userSubscriptions.earlyRefreshNote': '套餐总额度仍是 {total} 期',
    'userSubscriptions.earlyRefreshConfirm': '确认刷新',
    'userSubscriptions.earlyRefreshBlockedQuota': '本周还有 ${amount} 未用完，用完后即可提前刷新下一期。',
    'userSubscriptions.earlyRefreshSuccess': '已发放第 {count} / {total} 期额度 ${amount}',
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
    can_credit_next_early: false,
    early_credit_block_reason: 'weekly_quota_remaining',
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
    creditNextEarlyBalancePackage.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    refreshUser.mockReset()
    routerPush.mockReset()
    getMySubscriptions.mockResolvedValue([])
    refreshUser.mockResolvedValue(undefined)
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

  it('本周额度未用完时提前刷新按钮禁用并说明原因', async () => {
    getMyBalancePackages.mockResolvedValue({ data: [balancePackage()] })

    const wrapper = mountView()
    await flushPromises()

    const button = wrapper.get('button[disabled]')
    expect(button.text()).toContain('提前刷新')
    expect(wrapper.text()).toContain('本周还有 $43.25 未用完')
  })

  it('本周额度用尽后可提前刷新，并刷新套餐与余额', async () => {
    getMyBalancePackages.mockResolvedValue({
      data: [balancePackage({ current_remaining_usd: 0, can_credit_next_early: true, early_credit_block_reason: undefined })]
    })
    creditNextEarlyBalancePackage.mockResolvedValue({
      data: { credited_count: 3, refresh_count: 4, credit_usd: 128 }
    })

    const wrapper = mountView()
    await flushPromises()

    const button = wrapper.findAll('button').find((item) => item.text().includes('提前刷新'))
    expect(button).toBeTruthy()
    expect(button!.attributes('disabled')).toBeUndefined()

    await button!.trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('提前刷新下一期额度')

    const confirm = Array.from(document.querySelectorAll('button')).find((item) => item.textContent?.includes('确认刷新'))
    confirm!.click()
    await flushPromises()

    expect(creditNextEarlyBalancePackage).toHaveBeenCalledWith(1)
    expect(showSuccess).toHaveBeenCalledWith('已发放第 3 / 4 期额度 $128.00')
    expect(refreshUser).toHaveBeenCalled()
    // 发放后必须重新拉取套餐，否则卡片还停在旧的到账进度上
    expect(getMyBalancePackages).toHaveBeenCalledTimes(2)
  })

  it('套餐期数发完后不展示提前刷新入口', async () => {
    getMyBalancePackages.mockResolvedValue({
      data: [balancePackage({ status: 'completed', credited_count: 4, next_credit_at: undefined, can_credit_next_early: false, early_credit_block_reason: 'fully_credited' })]
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).not.toContain('提前刷新')
  })
})
