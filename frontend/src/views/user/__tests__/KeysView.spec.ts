import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import KeysView from '../KeysView.vue'

const keysList = vi.hoisted(() => vi.fn())
const keysCreate = vi.hoisted(() => vi.fn())
const keysUpdate = vi.hoisted(() => vi.fn())
const getPublicSettings = vi.hoisted(() => vi.fn())
const getAvailableGroups = vi.hoisted(() => vi.fn())
const getUserGroupRates = vi.hoisted(() => vi.fn())
const getDashboardApiKeysUsage = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/api', () => ({
  keysAPI: {
    list: keysList,
    create: keysCreate,
    update: keysUpdate,
    toggleStatus: vi.fn(),
    delete: vi.fn(),
  },
  authAPI: {
    getPublicSettings,
  },
  usageAPI: {
    getDashboardApiKeysUsage,
  },
  userGroupsAPI: {
    getAvailable: getAvailableGroups,
    getUserGroupRates,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep: vi.fn(() => false),
    nextStep: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn(() => Promise.resolve(true)),
  }),
}))

const automaticKey = {
  id: 11,
  user_id: 7,
  key: 'sk-test-automatic-key',
  name: 'Auto Key',
  group_id: null,
  group: null,
  status: 'active',
  ip_whitelist: [],
  ip_blacklist: [],
  last_used_at: null,
  quota: 0,
  quota_used: 0,
  expires_at: null,
  created_at: '2026-07-05T00:00:00Z',
  updated_at: '2026-07-05T00:00:00Z',
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  window_5h_start: null,
  window_1d_start: null,
  window_7d_start: null,
  reset_5h_at: null,
  reset_1d_at: null,
  reset_7d_at: null,
}

function mountKeysView() {
  return mount(KeysView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template:
            '<div><slot name="filters" /><slot name="actions" /><slot name="table" /><slot name="pagination" /></div>',
        },
        DataTable: {
          props: ['data'],
          template:
            '<div><div v-for="row in data" :key="row.id" data-test="key-row"><slot name="cell-group" :row="row" /><slot name="cell-actions" :row="row" /></div></div>',
        },
        BaseDialog: {
          props: ['show'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>',
        },
        Select: {
          props: ['options'],
          template:
            '<select><option v-for="option in options" :key="String(option.value)" :value="option.value">{{ option.label }}</option></select>',
        },
        SearchInput: { template: '<input />' },
        Pagination: true,
        ConfirmDialog: true,
        EmptyState: true,
        EndpointPopover: true,
        GroupBadge: {
          props: ['name'],
          template: '<span data-test="group-badge">{{ name }}</span>',
        },
        Icon: { template: '<span />' },
        UseKeyModal: true,
      },
    },
  })
}

describe('KeysView 自动 API Key', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    keysList.mockResolvedValue({
      items: [automaticKey],
      total: 1,
      pages: 1,
      page: 1,
      page_size: 10,
    })
    keysCreate.mockResolvedValue({ ...automaticKey, id: 12, name: 'Created Key' })
    keysUpdate.mockResolvedValue(automaticKey)
    getPublicSettings.mockResolvedValue({})
    getAvailableGroups.mockResolvedValue([
      {
        id: 2,
        name: 'codex-pool-19-usd',
        platform: 'openai',
        subscription_type: 'subscription',
        rate_multiplier: 1,
        description: '',
      },
    ])
    getUserGroupRates.mockResolvedValue({})
    getDashboardApiKeysUsage.mockResolvedValue({ stats: {} })
  })

  it('把 group_id 为空的 Key 显示为自动分组', async () => {
    const wrapper = mountKeysView()
    await flushPromises()

    expect(wrapper.text()).toContain('keys.autoGroup')
    expect(wrapper.text()).toContain('keys.autoGroupHint')
    expect(wrapper.find('[data-tour="key-form-group"]').exists()).toBe(false)
  })

  it('创建 Key 时不再传 group_id', async () => {
    const wrapper = mountKeysView()
    await flushPromises()

    await wrapper.find('[data-tour="keys-create-btn"]').trigger('click')
    await wrapper.find('[data-tour="key-form-name"]').setValue('Created Key')
    await wrapper.find('#key-form').trigger('submit')
    await flushPromises()

    expect(showError).not.toHaveBeenCalledWith('keys.groupRequired')
    expect(keysCreate).toHaveBeenCalledWith(
      'Created Key',
      undefined,
      undefined,
      [],
      [],
      0,
      undefined,
      { rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 }
    )
  })
})
