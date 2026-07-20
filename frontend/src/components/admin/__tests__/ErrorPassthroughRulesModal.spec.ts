import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { createRule, listRules } = vi.hoisted(() => ({
  createRule: vi.fn(),
  listRules: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    errorPassthrough: {
      create: createRule,
      delete: vi.fn(),
      list: listRules,
      toggleEnabled: vi.fn(),
      update: vi.fn()
    }
  }
}))

vi.mock('@/i18n', () => ({
  i18n: { global: { t: (key: string) => key } }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

import ErrorPassthroughRulesModal from '../ErrorPassthroughRulesModal.vue'

describe('ErrorPassthroughRulesModal', () => {
  beforeEach(() => {
    createRule.mockReset()
    listRules.mockResolvedValue([])
  })

  it('创建规则时不提交已废弃的响应覆写字段', async () => {
    createRule.mockResolvedValue({})
    const wrapper = mount(ErrorPassthroughRulesModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show'],
            template: '<div v-if="show"><slot /><slot name="footer" /></div>'
          },
          ConfirmDialog: true,
          Icon: true
        }
      }
    })
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('createRule'))!.trigger('click')
    await wrapper.find('input[required]').setValue('rate limit classifier')
    await wrapper.find('input[placeholder="admin.errorPassthrough.form.errorCodesPlaceholder"]').setValue('429')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(createRule).toHaveBeenCalledWith(expect.objectContaining({
      name: 'rate limit classifier',
      error_codes: [429]
    }))
    expect(createRule.mock.calls[0][0]).not.toHaveProperty('passthrough_code')
    expect(createRule.mock.calls[0][0]).not.toHaveProperty('response_code')
    expect(createRule.mock.calls[0][0]).not.toHaveProperty('passthrough_body')
    expect(createRule.mock.calls[0][0]).not.toHaveProperty('custom_message')
  })
})
