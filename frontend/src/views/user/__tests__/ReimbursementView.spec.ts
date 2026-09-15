import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ReimbursementView from '../ReimbursementView.vue'

const { parse, submit, listMine, downloadPdf, saveBlob, showSuccess, showError, showInfo, showWarning } =
  vi.hoisted(() => ({
    parse: vi.fn(),
    submit: vi.fn(),
    listMine: vi.fn(),
    downloadPdf: vi.fn(),
    saveBlob: vi.fn(),
    showSuccess: vi.fn(),
    showError: vi.fn(),
    showInfo: vi.fn(),
    showWarning: vi.fn()
  }))

vi.mock('@/api/reimbursement', () => {
  const REIMBURSEMENT_FIELD_KEYS = ['company_name', 'tax_id', 'bank_account', 'bank_name', 'address', 'amount']
  return {
    REIMBURSEMENT_FIELD_KEYS,
    reimbursementAPI: { parse, submit, listMine, downloadPdf, getById: vi.fn() }
  }
})

vi.mock('@/api/batchImage', () => ({
  saveBlob
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
    showInfo,
    showWarning
  })
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

const TextAreaStub = {
  props: ['modelValue', 'placeholder', 'disabled', 'label', 'hint', 'rows', 'id'],
  emits: ['update:modelValue'],
  template: `<textarea :id="id" :value="modelValue ?? ''" :disabled="disabled" @input="$emit('update:modelValue', $event.target.value)"></textarea>`
}

const case2Fields = {
  company_name: '上海熠视智能科技有限公司',
  tax_id: '91310115MAKFGD5NXY',
  bank_account: '121995771710001',
  bank_name: '招商银行股份有限公司上海张杨支行',
  address: null,
  amount: 414.1
}

const case2Incomplete = {
  fields: case2Fields,
  missing: ['address'],
  complete: false,
  notes: null
}

const case2Complete = {
  fields: { ...case2Fields, address: '上海市浦东新区张杨路500号华润时代广场12楼' },
  missing: [],
  complete: true,
  notes: null
}

function mountView() {
  return mount(ReimbursementView, {
    attachTo: document.body,
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        DataTable: DataTableStub,
        TextArea: TextAreaStub,
        Pagination: true,
        EmptyState: true,
        Icon: true,
        transition: false
      }
    }
  })
}

describe('user ReimbursementView', () => {
  beforeEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''
    parse.mockReset()
    submit.mockReset()
    listMine.mockReset()
    downloadPdf.mockReset()
    saveBlob.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    showInfo.mockReset()
    showWarning.mockReset()

    listMine.mockResolvedValue({
      items: [
        {
          id: 1,
          company_name: '福州斯摩尔贸易有限公司',
          tax_id: '91350102068793190Q',
          bank_account: '406565458594',
          bank_name: '中国银行福州东区支行',
          address: '福州市鼓楼区',
          amount: 45,
          status: 'pending',
          pdf_available: false,
          pdf_file_name: '',
          created_at: '2026-09-15T04:00:00Z',
          updated_at: '2026-09-15T04:00:00Z',
          completed_at: null
        },
        {
          id: 2,
          company_name: '北京和讯在线信息咨询服务有限公司',
          tax_id: '91110105723558454P',
          bank_account: '0200080709024530517',
          bank_name: '中国工商银行股份有限公司北京东城支行',
          address: '北京市朝阳区朝外大街22号10层A1-3',
          amount: 49,
          status: 'completed',
          pdf_available: true,
          pdf_file_name: 'invoice-2.pdf',
          created_at: '2026-09-14T04:00:00Z',
          updated_at: '2026-09-15T04:00:00Z',
          completed_at: '2026-09-15T04:00:00Z'
        }
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
  })

  it('highlights missing fields, disables submit, and enables it after supplement parse; submit clears the draft', async () => {
    parse.mockResolvedValueOnce(case2Incomplete)
    submit.mockResolvedValue({ id: 3, status: 'pending' })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('#reimbursement-text').setValue('公司名称：上海熠视智能科技有限公司 金额414.1')
    await wrapper.get('[data-test="parse-button"]').trigger('click')
    await flushPromises()

    expect(parse).toHaveBeenCalledWith('公司名称：上海熠视智能科技有限公司 金额414.1')
    expect(wrapper.find('[data-test="result-card"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="incomplete-banner"]').exists()).toBe(true)

    const missingHints = wrapper.findAll('[data-test="field-missing"]')
    expect(missingHints).toHaveLength(1)
    const addressRow = wrapper.find('[data-test="field-row"][data-field="address"]')
    expect(addressRow.find('[data-test="field-missing"]').exists()).toBe(true)
    expect(addressRow.find('.input-error').exists()).toBe(true)

    const submitButton = wrapper.get('[data-test="submit-button"]')
    expect((submitButton.element as HTMLButtonElement).disabled).toBe(true)
    expect(wrapper.find('[data-test="supplement-input"]').exists()).toBe(true)

    // 草稿已写入
    const draftRaw = localStorage.getItem('reimbursement_draft')
    expect(draftRaw).toBeTruthy()
    expect(JSON.parse(draftRaw as string).missing).toEqual(['address'])

    // 补充后带 previous 重新解析
    parse.mockResolvedValueOnce(case2Complete)
    await wrapper.get('#reimbursement-supplement').setValue('地址是上海市浦东新区张杨路500号华润时代广场12楼')
    await wrapper.get('[data-test="supplement-button"]').trigger('click')
    await flushPromises()

    expect(parse).toHaveBeenLastCalledWith('地址是上海市浦东新区张杨路500号华润时代广场12楼', case2Fields)
    expect(wrapper.find('[data-test="complete-banner"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-test="field-missing"]')).toHaveLength(0)
    expect(wrapper.find('[data-test="supplement-input"]').exists()).toBe(false)
    expect((wrapper.get('[data-test="submit-button"]').element as HTMLButtonElement).disabled).toBe(false)

    // 提交
    await wrapper.get('[data-test="submit-button"]').trigger('click')
    await flushPromises()

    expect(submit).toHaveBeenCalledTimes(1)
    expect(submit).toHaveBeenCalledWith({
      ...case2Complete.fields,
      amount: 414.1,
      raw_text: '公司名称：上海熠视智能科技有限公司 金额414.1\n地址是上海市浦东新区张杨路500号华润时代广场12楼'
    })
    expect(showSuccess).toHaveBeenCalledWith('reimbursement.submitSuccess')
    expect(localStorage.getItem('reimbursement_draft')).toBeNull()
    expect(wrapper.find('[data-test="result-card"]').exists()).toBe(false)
    // 提交后刷新列表
    expect(listMine).toHaveBeenCalledTimes(2)
  })

  it('disables parse, restart, submit and the paste box while a supplement parse is in flight', async () => {
    parse.mockResolvedValueOnce(case2Incomplete)

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('#reimbursement-text').setValue('公司名称：上海熠视智能科技有限公司 金额414.1')
    await wrapper.get('[data-test="parse-button"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="incomplete-banner"]').exists()).toBe(true)

    // 补充解析挂起，期间其它入口都必须锁住
    let resolveSupplement: (value: typeof case2Complete) => void = () => {}
    parse.mockImplementationOnce(
      () =>
        new Promise<typeof case2Complete>((resolve) => {
          resolveSupplement = resolve
        })
    )
    await wrapper.get('#reimbursement-supplement').setValue('地址是上海市浦东新区张杨路500号华润时代广场12楼')
    await wrapper.get('[data-test="supplement-button"]').trigger('click')
    await flushPromises()

    const disabled = (selector: string) => (wrapper.get(selector).element as HTMLButtonElement).disabled
    expect(disabled('[data-test="parse-button"]')).toBe(true)
    expect(disabled('[data-test="restart-button"]')).toBe(true)
    expect(disabled('[data-test="submit-button"]')).toBe(true)
    expect(disabled('[data-test="supplement-button"]')).toBe(true)
    expect((wrapper.get('#reimbursement-text').element as HTMLTextAreaElement).disabled).toBe(true)

    resolveSupplement(case2Complete)
    await flushPromises()

    expect(disabled('[data-test="parse-button"]')).toBe(false)
    expect(disabled('[data-test="restart-button"]')).toBe(false)
    expect(disabled('[data-test="submit-button"]')).toBe(false)
    expect((wrapper.get('#reimbursement-text').element as HTMLTextAreaElement).disabled).toBe(false)
  })

  it('falls back to the i18n message when the download rejects with the raw axios status text', async () => {
    downloadPdf.mockRejectedValue({ status: 404, message: 'Request failed with status code 404' })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('[data-test="download-button"]')[0].trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('reimbursement.downloadFailed')
    expect(saveBlob).not.toHaveBeenCalled()
  })

  it('renders request list with status badges and download button, and saves the PDF blob', async () => {
    const pdfBlob = new Blob(['%PDF-1.4'], { type: 'application/pdf' })
    downloadPdf.mockResolvedValue(pdfBlob)

    const wrapper = mountView()
    await flushPromises()

    expect(listMine).toHaveBeenCalledWith(1, 20)
    const badges = wrapper.findAll('[data-test="status-badge"]')
    expect(badges).toHaveLength(2)
    expect(badges[0].text()).toBe('reimbursement.status.pending')
    expect(badges[0].classes()).toContain('badge-warning')
    expect(badges[1].text()).toBe('reimbursement.status.completed')
    expect(badges[1].classes()).toContain('badge-success')

    const downloadButtons = wrapper.findAll('[data-test="download-button"]')
    expect(downloadButtons).toHaveLength(1)
    await downloadButtons[0].trigger('click')
    await flushPromises()

    expect(downloadPdf).toHaveBeenCalledWith(2)
    expect(saveBlob).toHaveBeenCalledWith(pdfBlob, 'invoice-2.pdf')
    expect(showError).not.toHaveBeenCalled()
  })

  it('shows the backend error when the PDF download returns a JSON error body', async () => {
    const errorBlob = new Blob([JSON.stringify({ code: 404, message: 'pdf not ready' })], {
      type: 'application/json'
    })
    downloadPdf.mockResolvedValue(errorBlob)

    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('[data-test="download-button"]')[0].trigger('click')
    await flushPromises()

    // jsdom 的 Blob 没有 text()，组件退回 FileReader（异步宏任务），需要等待
    await vi.waitFor(() => expect(showError).toHaveBeenCalledWith('pdf not ready'))
    expect(saveBlob).not.toHaveBeenCalled()
  })

  it('restores an unsubmitted draft from localStorage on mount', async () => {
    localStorage.setItem(
      'reimbursement_draft',
      JSON.stringify({
        text: '公司名称：上海熠视智能科技有限公司',
        rawText: '公司名称：上海熠视智能科技有限公司',
        fields: case2Fields,
        missing: ['address'],
        complete: false,
        updatedAt: '2026-09-15T00:00:00Z'
      })
    )

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-test="draft-restored"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="result-card"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-test="field-missing"]')).toHaveLength(1)
    expect((wrapper.get('[data-test="submit-button"]').element as HTMLButtonElement).disabled).toBe(true)
    expect(showInfo).toHaveBeenCalledWith('reimbursement.draftRestored')

    await wrapper.get('[data-test="restart-button"]').trigger('click')
    expect(localStorage.getItem('reimbursement_draft')).toBeNull()
    expect(wrapper.find('[data-test="result-card"]').exists()).toBe(false)
  })
})
