import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const mocks = vi.hoisted(() => ({
  create: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { create: mocks.create }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: mocks.showError,
    showSuccess: mocks.showSuccess
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    props: ['show', 'title', 'width'],
    template: '<div v-if="show"><slot /><slot name="footer" /></div>'
  }
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: { template: '<span />' }
}))

import UserCreateModal from '../UserCreateModal.vue'

const mountModal = () => mount(UserCreateModal, { props: { show: true } })

const getConcurrencyInput = (wrapper: ReturnType<typeof mountModal>) => {
  const inputs = wrapper.findAll('input[type="number"]')
  expect(inputs).toHaveLength(3)
  return inputs[1]
}

const fillRequiredFields = async (wrapper: ReturnType<typeof mountModal>) => {
  await wrapper.get('input[type="email"]').setValue('new@example.com')
  const textInputs = wrapper.findAll('input[type="text"]')
  expect(textInputs.length).toBeGreaterThanOrEqual(2)
  await textInputs[0].setValue('secret123')
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.create.mockResolvedValue(undefined)
})

describe('UserCreateModal', () => {
  it('标明 0 表示不限并发', () => {
    const wrapper = mountModal()
    const input = getConcurrencyInput(wrapper)

    expect(input.attributes('min')).toBe('0')
    expect(input.attributes('step')).toBe('1')
    expect(wrapper.text()).toContain('admin.users.form.concurrencyHint')
  })

  it('允许提交 0 并发', async () => {
    const wrapper = mountModal()
    await fillRequiredFields(wrapper)
    await getConcurrencyInput(wrapper).setValue(0)

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(mocks.create).toHaveBeenCalledWith(
      expect.objectContaining({ concurrency: 0 })
    )
    expect(mocks.showError).not.toHaveBeenCalled()
  })

  it.each([-1, 0.5])('拒绝非法并发值 %s', async (value) => {
    const wrapper = mountModal()
    await fillRequiredFields(wrapper)
    await getConcurrencyInput(wrapper).setValue(value)

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(mocks.showError).toHaveBeenCalledWith('admin.users.concurrencyInvalid')
    expect(mocks.create).not.toHaveBeenCalled()
  })
})
