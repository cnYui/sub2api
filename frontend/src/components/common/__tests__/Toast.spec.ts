import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

const copyToClipboard = vi.fn().mockResolvedValue(true)

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

import Toast from '../Toast.vue'
import { useAppStore } from '@/stores/app'

describe('Toast', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    copyToClipboard.mockClear()
  })

  afterEach(() => {
    document.body.innerHTML = ''
    vi.restoreAllMocks()
  })

  it('使用命名 toast-motion 且不声明错误方向的过渡类', () => {
    const source = readFileSync(join(process.cwd(), 'src/components/common/Toast.vue'), 'utf8')

    expect(source).toContain('name="toast-motion"')
    expect(source).not.toContain('ease-in')
    expect(source).not.toContain('transition-all')
  })

  it('展示并复制规范错误引用', async () => {
    const store = useAppStore()
    store.showError('请求过于频繁', 5000, {
      errorId: 'S2A-5004',
      errorCode: 'UPSTREAM_RATE_LIMITED',
      requestId: 'req_contract_1'
    })

    const wrapper = mount(Toast, { attachTo: document.body })

    expect(document.body.textContent).toContain('S2A-5004')
    expect(document.body.textContent).toContain('UPSTREAM_RATE_LIMITED')
    expect(document.body.textContent).toContain('req_contract_1')
    expect(document.body.querySelector('[aria-label="common.copyErrorReference"]')).not.toBeNull()

    await document.body.querySelector<HTMLButtonElement>('[data-testid="toast-copy-error-reference"]')!.click()

    expect(copyToClipboard).toHaveBeenCalledWith(
      'S2A-5004\nUPSTREAM_RATE_LIMITED\nRequest ID: req_contract_1'
    )

    wrapper.unmount()
  })
})
