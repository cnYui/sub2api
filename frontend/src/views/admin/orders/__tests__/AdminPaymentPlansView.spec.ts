import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AdminPaymentPlansView from '../AdminPaymentPlansView.vue'

const getPlans = vi.hoisted(() => vi.fn())
const updatePlan = vi.hoisted(() => vi.fn())
const deletePlan = vi.hoisted(() => vi.fn())
const getAllGroups = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const labels: Record<string, string> = {
    'payment.admin.price': '基础价',
    'payment.admin.days': '天',
    'common.actions': '操作',
    'common.refresh': '刷新',
    'common.edit': '编辑',
    'common.delete': '删除',
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => labels[key] ?? key,
    }),
  }
})

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    getPlans,
    updatePlan,
    deletePlan,
  },
}))

vi.mock('@/api/admin', () => ({
  default: {
    groups: {
      getAll: getAllGroups,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

const planFixture = () => ({
  id: 5,
  group_id: 5,
  group_platform: 'openai',
  group_name: 'codex-pool-69-usd',
  name: '79 元订阅池',
  description: '',
  price: 79,
  original_price: 0,
  validity_days: 30,
  validity_unit: 'day',
  rate_multiplier: 1,
  daily_limit_usd: 69,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  supported_model_scopes: [],
  features: '',
  for_sale: true,
  sort_order: 79,
})

describe('AdminPaymentPlansView', () => {
  beforeEach(() => {
    getPlans.mockReset().mockResolvedValue({ data: [planFixture()] })
    updatePlan.mockReset()
    deletePlan.mockReset()
    getAllGroups.mockReset().mockResolvedValue([
      {
        id: 5,
        name: 'codex-pool-69-usd',
        platform: 'openai',
        rate_multiplier: 1,
      },
    ])
    showError.mockReset()
    showSuccess.mockReset()
  })

  it('renders subscription plan price as the RMB base price in admin plans', async () => {
    const wrapper = mount(AdminPaymentPlansView, {
      global: {
        stubs: {
          AppLayout: {
            template: '<section><slot /></section>',
          },
          DataTable: {
            props: ['columns', 'data', 'loading'],
            template: `
              <div data-testid="plans-table">
                <div>
                  <span v-for="column in columns" :key="column.key" data-testid="plan-column">
                    {{ column.label }}
                  </span>
                </div>
                <div v-for="row in data" :key="row.id" data-testid="plan-row">
                  <div v-for="column in columns" :key="column.key" :data-cell="column.key">
                    <slot :name="'cell-' + column.key" :row="row" :value="row[column.key]">
                      {{ row[column.key] }}
                    </slot>
                  </div>
                </div>
              </div>
            `,
          },
          ConfirmDialog: true,
          PlanEditDialog: true,
          Icon: true,
          GroupBadge: {
            props: ['name'],
            template: '<span>{{ name }}</span>',
          },
        },
      },
    })
    await flushPromises()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('基础价')
    expect(text).toContain('¥79.00')
    expect(text).not.toContain('$79.00')
    expect(text).not.toContain('¥79.79')
  })
})
