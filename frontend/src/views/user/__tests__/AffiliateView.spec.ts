import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AffiliateView from '../AffiliateView.vue'

const getAffiliateDetail = vi.hoisted(() => vi.fn())
const transferAffiliateQuota = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const refreshUser = vi.hoisted(() => vi.fn())

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail,
    transferAffiliateQuota,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'affiliate.transfer.success') return `已转入 ${params?.amount}`
        return key
      },
    }),
  }
})

function affiliateDetailFixture() {
  return {
    user_id: 1,
    aff_code: 'AFF123',
    inviter_id: null,
    aff_count: 1,
    aff_quota: 12.34,
    aff_frozen_quota: 1.23,
    aff_history_quota: 56.78,
    effective_rebate_rate_percent: 8,
    invitees: [
      {
        user_id: 2,
        email: 'friend@example.com',
        username: 'friend',
        created_at: '2026-07-08T00:00:00Z',
        total_rebate: 9.87,
      },
    ],
  }
}

describe('AffiliateView', () => {
  beforeEach(() => {
    getAffiliateDetail.mockReset().mockResolvedValue(affiliateDetailFixture())
    transferAffiliateQuota.mockReset().mockResolvedValue({
      transferred_quota: 12.34,
      balance: 100,
    })
    showSuccess.mockReset()
    showError.mockReset()
    refreshUser.mockReset().mockResolvedValue(undefined)
  })

  it('返利金额展示为人民币符号', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('¥12.34')
    expect(text).toContain('¥56.78')
    expect(text).toContain('¥1.23')
    expect(text).toContain('¥9.87')
    expect(text).not.toContain('US$')
    expect(text).not.toContain('$12.34')

    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()

    expect(showSuccess).toHaveBeenCalledWith('已转入 ¥12.34')
  })
})
