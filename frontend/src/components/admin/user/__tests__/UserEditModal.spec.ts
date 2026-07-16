import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const mocks = vi.hoisted(() => ({
  update: vi.fn(),
  updateUserAttributeValues: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { update: mocks.update },
    userAttributes: { updateUserAttributeValues: mocks.updateUserAttributeValues }
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

vi.mock('@/components/user/UserAttributeForm.vue', () => ({
  default: { template: '<div />' }
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: { template: '<span />' }
}))

import UserEditModal from '../UserEditModal.vue'

const makeUser = (concurrency = 5) =>
  ({
    id: 13,
    email: 'xiaobianfuai@gmail.com',
    username: 'xiaobianfuai@gmail.com',
    notes: '',
    role: 'admin',
    status: 'active',
    balance: 0,
    concurrency,
    rpm_limit: 0,
    allowed_groups: []
  }) as any

const mountModal = (concurrency = 5) =>
  mount(UserEditModal, {
    props: { show: true, user: makeUser(concurrency) }
  })

const getConcurrencyInput = (wrapper: ReturnType<typeof mountModal>) => {
  const inputs = wrapper.findAll('input[type="number"]')
  expect(inputs).toHaveLength(2)
  return inputs[0]
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.update.mockResolvedValue(makeUser(0))
  mocks.updateUserAttributeValues.mockResolvedValue(undefined)
})

describe('UserEditModal', () => {
  it('显示已有的 0 并发并标明不限并发契约', () => {
    const wrapper = mountModal(0)
    const input = getConcurrencyInput(wrapper)

    expect((input.element as HTMLInputElement).value).toBe('0')
    expect(input.attributes('min')).toBe('0')
    expect(input.attributes('step')).toBe('1')
    expect(wrapper.text()).toContain('admin.users.form.concurrencyHint')
  })

  it('允许提交 0 并发', async () => {
    const wrapper = mountModal()
    await getConcurrencyInput(wrapper).setValue(0)

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(mocks.update).toHaveBeenCalledWith(
      13,
      expect.objectContaining({ concurrency: 0 })
    )
    expect(mocks.showError).not.toHaveBeenCalled()
  })

  it.each([-1, 0.5])('拒绝非法并发值 %s', async (value) => {
    const wrapper = mountModal()
    await getConcurrencyInput(wrapper).setValue(value)

    const form = wrapper.get('form').element as HTMLFormElement
    form.requestSubmit()
    await flushPromises()

    expect(mocks.showError).toHaveBeenCalledWith('admin.users.concurrencyInvalid')
    expect(mocks.update).not.toHaveBeenCalled()
  })
})
