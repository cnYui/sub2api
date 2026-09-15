import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ReimbursementsView from '../ReimbursementsView.vue'

const { list, uploadPdf, downloadPdf, getConfig, updateConfig, testConfig, showSuccess, showError } =
  vi.hoisted(() => ({
    list: vi.fn(),
    uploadPdf: vi.fn(),
    downloadPdf: vi.fn(),
    getConfig: vi.fn(),
    updateConfig: vi.fn(),
    testConfig: vi.fn(),
    showSuccess: vi.fn(),
    showError: vi.fn()
  }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    reimbursements: {
      list,
      getById: vi.fn(),
      uploadPdf,
      downloadPdf,
      getConfig,
      updateConfig,
      testConfig
    }
  }
}))

vi.mock('@/api/batchImage', () => ({
  saveBlob: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
    showInfo: vi.fn(),
    showWarning: vi.fn()
  })
}))

vi.mock('@/composables/usePersistedPageSize', () => ({
  getPersistedPageSize: () => 20
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <table>
      <thead>
        <tr>
          <th v-for="column in columns" :key="column.key">
            <slot :name="'header-' + column.key" :column="column">{{ column.label }}</slot>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in data" :key="row.id">
          <td v-for="column in columns" :key="column.key">
            <slot :name="'cell-' + column.key" :row="row" :value="row[column.key]">
              {{ row[column.key] }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
  `
}

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  template: `<select :value="modelValue ?? ''"><option v-for="option in options" :key="String(option.value ?? '')" :value="option.value ?? ''">{{ option.label }}</option></select>`
}

function makeRow(id: number, status: 'pending' | 'completed') {
  return {
    id,
    user_id: 100 + id,
    user_email: id === 1 ? '' : `user${id}@example.com`,
    username: id === 1 ? 'alice' : '',
    company_name: `公司${id}`,
    tax_id: `9131011${id}`,
    bank_account: `12199577171000${id}`,
    bank_name: '招商银行股份有限公司上海张杨支行',
    address: '上海市浦东新区张杨路500号',
    amount: 100 + id,
    status,
    pdf_available: status === 'completed',
    pdf_file_name: status === 'completed' ? `invoice-${id}.pdf` : '',
    raw_text: `原文${id}`,
    pdf_size: 0,
    pdf_uploaded_at: null,
    handled_by: null,
    notified_at: null,
    created_at: `2026-09-1${id}T00:00:00Z`,
    updated_at: `2026-09-1${id}T00:00:00Z`,
    completed_at: null
  }
}

function mountView() {
  return mount(ReimbursementsView, {
    attachTo: document.body,
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        Pagination: true,
        Select: SelectStub,
        EmptyState: true,
        Icon: true,
        Teleport: true
      }
    }
  })
}

describe('admin ReimbursementsView', () => {
  beforeEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''
    list.mockReset()
    uploadPdf.mockReset()
    downloadPdf.mockReset()
    getConfig.mockReset()
    updateConfig.mockReset()
    testConfig.mockReset()
    showSuccess.mockReset()
    showError.mockReset()

    list.mockResolvedValue({
      items: [makeRow(1, 'pending'), makeRow(2, 'pending'), makeRow(3, 'completed')],
      total: 3,
      page: 1,
      page_size: 20,
      pages: 1
    })
    uploadPdf.mockResolvedValue(makeRow(1, 'completed'))
  })

  it('loads the list sorted by created_at asc and renders sequence numbers 1..n', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(list).toHaveBeenCalledTimes(1)
    const [page, pageSize, filters] = list.mock.calls[0]
    expect(page).toBe(1)
    expect(pageSize).toBe(20)
    expect(filters).toMatchObject({ sort_by: 'created_at', sort_order: 'asc' })
    expect(filters.status).toBeUndefined()
    expect(filters.search).toBeUndefined()

    const indexes = wrapper.findAll('[data-test="row-index"]').map((el) => el.text())
    expect(indexes).toEqual(['1', '2', '3'])

    // 用户列兜底：无邮箱显示用户名
    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('alice')
    expect(rows[1].text()).toContain('user2@example.com')

    // 仅已完成且有 PDF 的行有下载按钮
    expect(wrapper.findAll('[data-test="download-pdf"]')).toHaveLength(1)
  })

  it('uploads the selected PDF via uploadPdf(id, file) and refreshes the list', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('[data-test="upload-open"]')[0].trigger('click')
    await flushPromises()

    const input = wrapper.get('[data-test="upload-file-input"]')
    const file = new File(['%PDF-1.4 test'], 'invoice.pdf', { type: 'application/pdf' })
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change')
    await flushPromises()

    expect(wrapper.get('[data-test="upload-file-name"]').text()).toBe('invoice.pdf')
    const submitButton = wrapper.get('[data-test="upload-submit"]')
    expect((submitButton.element as HTMLButtonElement).disabled).toBe(false)

    await submitButton.trigger('click')
    await flushPromises()

    expect(uploadPdf).toHaveBeenCalledTimes(1)
    expect(uploadPdf).toHaveBeenCalledWith(1, file)
    expect(showSuccess).toHaveBeenCalledWith('admin.reimbursements.uploadSuccess')
    expect(list).toHaveBeenCalledTimes(2)
  })

  it('rejects non-pdf files in the upload dialog', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('[data-test="upload-open"]')[0].trigger('click')
    await flushPromises()

    const input = wrapper.get('[data-test="upload-file-input"]')
    const file = new File(['hello'], 'notes.txt', { type: 'text/plain' })
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.reimbursements.invalidFile')
    expect((wrapper.get('[data-test="upload-submit"]').element as HTMLButtonElement).disabled).toBe(true)
  })

  it('locks the config inputs and shows a loading hint until getConfig resolves', async () => {
    let resolveConfig: (value: unknown) => void = () => {}
    getConfig.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveConfig = resolve
        })
    )

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="config-open"]').trigger('click')
    await flushPromises()

    const isDisabled = (selector: string) => (wrapper.get(selector).element as HTMLInputElement).disabled
    expect(wrapper.find('[data-test="config-loading"]').exists()).toBe(true)
    expect(isDisabled('[data-test="config-base-url"]')).toBe(true)
    expect(isDisabled('[data-test="config-model"]')).toBe(true)
    expect(isDisabled('[data-test="config-timeout-ms"]')).toBe(true)
    expect(isDisabled('[data-test="config-api-key"]')).toBe(true)
    expect(isDisabled('[data-test="config-clear-api-key"]')).toBe(true)
    expect((wrapper.get('[data-test="config-save"]').element as HTMLButtonElement).disabled).toBe(true)

    resolveConfig({
      base_url: 'https://api.deepseek.com',
      model: 'deepseek-flash',
      timeout_ms: 60000,
      api_key_configured: false,
      api_key_masked: '',
      api_key_source: ''
    })
    await flushPromises()

    expect(wrapper.find('[data-test="config-loading"]').exists()).toBe(false)
    expect(isDisabled('[data-test="config-base-url"]')).toBe(false)
    expect(isDisabled('[data-test="config-model"]')).toBe(false)
    expect(isDisabled('[data-test="config-timeout-ms"]')).toBe(false)
    expect(isDisabled('[data-test="config-api-key"]')).toBe(false)
    expect(isDisabled('[data-test="config-clear-api-key"]')).toBe(false)
  })

  it('falls back to the i18n message when the download rejects with the raw axios status text', async () => {
    downloadPdf.mockRejectedValue({ status: 404, message: 'Request failed with status code 404' })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="download-pdf"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.reimbursements.downloadFailed')
  })

  it('opens the parser config dialog, saves without an empty api_key, and runs a test parse', async () => {
    getConfig.mockResolvedValue({
      base_url: 'https://api.deepseek.com',
      model: 'deepseek-flash',
      timeout_ms: 60000,
      api_key_configured: true,
      api_key_masked: '********dde9',
      api_key_source: 'settings'
    })
    updateConfig.mockResolvedValue({
      base_url: 'https://api.deepseek.com',
      model: 'deepseek-v4-pro',
      timeout_ms: 60000,
      api_key_configured: true,
      api_key_masked: '********dde9',
      api_key_source: 'settings'
    })
    testConfig.mockResolvedValue({
      ok: true,
      latency_ms: 1830,
      model: 'deepseek-flash',
      result: {
        fields: {
          company_name: '北京和讯在线信息咨询服务有限公司',
          tax_id: '91110105723558454P',
          bank_account: '0200080709024530517',
          bank_name: '中国工商银行股份有限公司北京东城支行',
          address: '北京市朝阳区朝外大街22号10层A1-3',
          amount: 49
        },
        missing: [],
        complete: true,
        notes: null
      }
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="config-open"]').trigger('click')
    await flushPromises()

    expect(getConfig).toHaveBeenCalledTimes(1)
    const apiKeyInput = wrapper.get('[data-test="config-api-key"]')
    expect(apiKeyInput.attributes('placeholder')).toBe('********dde9')

    await wrapper.get('[data-test="config-model"]').setValue('deepseek-v4-pro')
    await wrapper.get('#reimbursement-config-form').trigger('submit')
    await flushPromises()

    expect(updateConfig).toHaveBeenCalledWith({
      base_url: 'https://api.deepseek.com',
      model: 'deepseek-v4-pro',
      timeout_ms: 60000
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.reimbursements.config.saveSuccess')

    await wrapper.get('[data-test="config-test"]').trigger('click')
    await flushPromises()

    expect(testConfig).toHaveBeenCalledWith(undefined)
    const result = wrapper.get('[data-test="config-test-result"]')
    expect(result.text()).toContain('admin.reimbursements.config.complete')
    expect(result.text()).toContain('北京和讯在线信息咨询服务有限公司')
  })
})
