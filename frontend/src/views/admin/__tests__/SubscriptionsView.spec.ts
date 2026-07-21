import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { Group, UserSubscription } from '@/types'
import SubscriptionsView from '../SubscriptionsView.vue'

const {
  listSubscriptions,
  getAllGroups,
  searchUsers,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  listSubscriptions: vi.fn(),
  getAllGroups: vi.fn(),
  searchUsers: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    subscriptions: {
      list: listSubscriptions,
      assign: vi.fn(),
      extend: vi.fn(),
      revoke: vi.fn(),
      resetQuota: vi.fn(),
    },
    groups: {
      getAll: getAllGroups,
    },
    usage: {
      searchUsers,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const openAIGroup = (): Group => ({
  id: 2,
  name: 'codex-pool-19-usd',
  description: '29 元订阅池',
  platform: 'openai',
  rate_multiplier: 1,
  is_exclusive: false,
  status: 'active',
  subscription_type: 'subscription',
  daily_limit_usd: 15,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  allow_image_generation: true,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: '2026-07-21T00:00:00Z',
  updated_at: '2026-07-21T00:00:00Z',
})

const activeSubscription = (): UserSubscription => ({
  id: 1,
  user_id: 7,
  group_id: 2,
  status: 'active',
  starts_at: '2026-07-21T00:00:00+08:00',
  daily_usage_usd: 12.34,
  weekly_usage_usd: 12.34,
  monthly_usage_usd: 12.34,
  daily_window_start: '2026-07-21T00:00:00+08:00',
  weekly_window_start: null,
  monthly_window_start: null,
  created_at: '2026-07-21T00:00:00Z',
  updated_at: '2026-07-21T00:00:00Z',
  expires_at: '2026-08-20T00:00:00+08:00',
  user: {
    id: 7,
    username: 'display-check',
    email: 'display-check@example.com',
    role: 'user',
    balance: 0,
    concurrency: 1,
    status: 'active',
    allowed_groups: [],
    balance_notify_enabled: false,
    balance_notify_threshold: null,
    balance_notify_extra_emails: [],
    created_at: '2026-07-21T00:00:00Z',
    updated_at: '2026-07-21T00:00:00Z',
  },
  group: openAIGroup(),
})

const DataTableStub = {
  props: ['data'],
  template: `
    <div>
      <div v-for="row in data" :key="row.id" data-test="subscription-row">
        <slot name="cell-user" :row="row" />
        <slot name="cell-group" :row="row" />
        <slot name="cell-usage" :row="row" />
      </div>
    </div>
  `,
}

describe('admin SubscriptionsView', () => {
  beforeEach(() => {
    localStorage.clear()
    listSubscriptions.mockReset()
    getAllGroups.mockReset()
    searchUsers.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    listSubscriptions.mockResolvedValue({
      items: [activeSubscription()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getAllGroups.mockResolvedValue([openAIGroup()])
    searchUsers.mockResolvedValue([])
  })

  it('展示后端返回的订阅日用量和新日限额', async () => {
    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
          },
          DataTable: DataTableStub,
          Pagination: true,
          BaseDialog: true,
          ConfirmDialog: true,
          EmptyState: true,
          Select: true,
          GroupBadge: { props: ['name'], template: '<span>{{ name }}</span>' },
          GroupOptionItem: true,
          Icon: true,
          RouterLink: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('display-check@example.com')
    expect(wrapper.text()).toContain('codex-pool-19-usd')
    expect(wrapper.text()).toContain('$12.34')
    expect(wrapper.text()).toContain('$15.00')
    expect(wrapper.find('[data-test="subscription-row"]').text()).not.toContain('$0.00 / $15.00')
  })
})
