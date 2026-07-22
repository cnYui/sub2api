import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PlanEditDialog from '../PlanEditDialog.vue'

const updatePlan = vi.hoisted(() => vi.fn())
const createPlan = vi.hoisted(() => vi.fn())
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

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    createPlan,
    updatePlan,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

describe('PlanEditDialog', () => {
  it('保存公共 Codex 套餐时固定提交 28 天有效期', async () => {
    updatePlan.mockResolvedValue({})
    createPlan.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    const wrapper = mount(PlanEditDialog, {
      props: {
        show: false,
        plan: {
          id: 5,
          group_id: 5,
          group_platform: 'openai',
          group_name: 'codex-pool-69-usd',
          name: '79 元订阅池',
          description: '历史漂移套餐',
          price: 79,
          original_price: 0,
          validity_days: 365,
          validity_unit: 'year',
          rate_multiplier: 1,
          daily_limit_usd: null,
          weekly_limit_usd: 206,
          monthly_limit_usd: null,
          supported_model_scopes: [],
          features: [],
          for_sale: true,
          sort_order: 79,
        },
        groups: [
          {
            id: 5,
            name: 'codex-pool-69-usd',
            platform: 'openai',
            rate_multiplier: 1,
            subscription_type: 'subscription',
            weekly_limit_usd: null,
          },
        ],
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show'],
            template: '<section v-if="show"><slot /><slot name="footer" /></section>',
          },
          Select: {
            props: ['modelValue', 'options', 'disabled'],
            template: '<select :disabled="disabled"><option>{{ modelValue }}</option></select>',
          },
          Icon: true,
          GroupBadge: true,
        },
      },
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    const validityInput = wrapper.find('input[type="number"][min="1"]')
    expect(validityInput.element).toHaveProperty('disabled', true)
    const textareas = wrapper.findAll('textarea')
    expect((textareas[0].element as HTMLTextAreaElement).value).toContain('28 天订阅，每 7 天刷新 206 USD 周额度，购买时间起滚动计算')
    expect((textareas[0].element as HTMLTextAreaElement).value).not.toContain('历史漂移套餐')
    expect((textareas[1].element as HTMLTextAreaElement).value).toContain('周额度 206 USD')
    expect(wrapper.text()).toContain('payment.admin.publicCodex28DayHint')
    expect(wrapper.text()).toContain('$206')
    expect(wrapper.text()).toContain('$824')
    expect(wrapper.text()).not.toContain('payment.admin.monthlyLimit')

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(updatePlan).toHaveBeenCalledWith(5, expect.objectContaining({
      validity_days: 28,
      validity_unit: 'days',
    }))
    expect(createPlan).not.toHaveBeenCalled()
  })
})
