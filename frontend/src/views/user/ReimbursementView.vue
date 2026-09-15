<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="mx-auto max-w-3xl space-y-6">
        <!-- Draft restored notice -->
        <transition name="fade">
          <div
            v-if="draftRestored"
            class="card border-primary-200 bg-primary-50 dark:border-primary-800/50 dark:bg-primary-900/20"
            data-test="draft-restored"
          >
            <div class="flex items-center gap-3 p-4">
              <Icon name="infoCircle" size="md" class="shrink-0 text-primary-600 dark:text-primary-400" />
              <p class="text-sm text-primary-800 dark:text-primary-300">{{ t('reimbursement.draftRestored') }}</p>
            </div>
          </div>
        </transition>

        <!-- Card 1: paste text -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('reimbursement.pasteTitle') }}
            </h2>
          </div>
          <div class="space-y-4 p-6">
            <TextArea
              id="reimbursement-text"
              v-model="text"
              :rows="8"
              :placeholder="t('reimbursement.pastePlaceholder')"
              :hint="t('reimbursement.pasteHint')"
              :disabled="busy"
              data-test="paste-input"
            />
            <div class="flex flex-wrap items-center justify-end gap-3">
              <button
                v-if="parseResult"
                type="button"
                class="btn btn-secondary"
                :disabled="busy"
                data-test="restart-button"
                @click="handleRestart"
              >
                {{ t('reimbursement.restartButton') }}
              </button>
              <button
                type="button"
                class="btn btn-primary"
                :disabled="!text.trim() || busy"
                data-test="parse-button"
                @click="handleParse"
              >
                <svg v-if="parsing" class="-ml-1 mr-2 h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
                <Icon v-else name="sparkles" size="md" class="mr-1" />
                {{ parsing ? t('reimbursement.parsing') : t('reimbursement.parseButton') }}
              </button>
            </div>
          </div>
        </div>

        <!-- Card 2: parsed result -->
        <div v-if="parseResult" class="card" data-test="result-card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('reimbursement.resultTitle') }}
            </h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('reimbursement.resultHint') }}</p>
          </div>
          <div class="space-y-5 p-6">
            <!-- Summary banner -->
            <div
              v-if="complete"
              class="flex items-center gap-3 rounded-xl border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-800/50 dark:bg-emerald-900/20"
              data-test="complete-banner"
            >
              <Icon name="checkCircle" size="md" class="shrink-0 text-emerald-600 dark:text-emerald-400" />
              <p class="text-sm font-medium text-emerald-800 dark:text-emerald-300">
                {{ t('reimbursement.completeBanner') }}
              </p>
            </div>
            <div
              v-else
              class="flex items-center gap-3 rounded-xl border border-amber-200 bg-amber-50 p-4 dark:border-amber-800/50 dark:bg-amber-900/20"
              data-test="incomplete-banner"
            >
              <Icon name="exclamationTriangle" size="md" class="shrink-0 text-amber-600 dark:text-amber-400" />
              <p class="text-sm font-medium text-amber-800 dark:text-amber-300">
                {{ t('reimbursement.incompleteBanner', { fields: missingLabels }) }}
              </p>
            </div>

            <!-- Read-only fields -->
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div
                v-for="key in fieldKeys"
                :key="key"
                :class="key === 'address' ? 'sm:col-span-2' : ''"
                data-test="field-row"
                :data-field="key"
              >
                <label class="input-label">{{ t(`reimbursement.fields.${key}`) }}</label>
                <div
                  :class="[
                    'input flex items-center justify-between gap-2',
                    isMissing(key) ? 'input-error ring-2 ring-red-500/20' : ''
                  ]"
                  :title="displayValue(key)"
                >
                  <span
                    class="truncate"
                    :class="isMissing(key) ? 'text-gray-400 dark:text-dark-500' : 'text-gray-900 dark:text-gray-100'"
                  >
                    {{ displayValue(key) || '—' }}
                  </span>
                  <Icon
                    v-if="!isMissing(key)"
                    name="checkCircle"
                    size="sm"
                    class="shrink-0 text-emerald-500"
                    :title="t('reimbursement.fieldParsed')"
                  />
                  <Icon v-else name="exclamationCircle" size="sm" class="shrink-0 text-red-500" />
                </div>
                <p v-if="isMissing(key)" class="input-error-text" data-test="field-missing">
                  {{ t('reimbursement.fieldMissing') }}
                </p>
              </div>
            </div>

            <p v-if="parseResult.notes" class="text-xs text-gray-500 dark:text-dark-400" data-test="parse-notes">
              {{ t('reimbursement.notesLabel') }}: {{ parseResult.notes }}
            </p>

            <!-- Supplement (only when incomplete) -->
            <div v-if="!complete" class="space-y-3 border-t border-gray-100 pt-5 dark:border-dark-700">
              <TextArea
                id="reimbursement-supplement"
                v-model="supplementText"
                :rows="3"
                :label="t('reimbursement.supplementLabel')"
                :placeholder="t('reimbursement.supplementPlaceholder')"
                :disabled="supplementing"
                data-test="supplement-input"
              />
              <div class="flex justify-end">
                <button
                  type="button"
                  class="btn btn-primary"
                  :disabled="!supplementText.trim() || busy"
                  data-test="supplement-button"
                  @click="handleSupplement"
                >
                  <Icon name="sparkles" size="md" class="mr-1" />
                  {{ supplementing ? t('reimbursement.supplementing') : t('reimbursement.supplementButton') }}
                </button>
              </div>
            </div>

            <!-- Submit -->
            <div class="flex justify-end border-t border-gray-100 pt-5 dark:border-dark-700">
              <button
                type="button"
                class="btn btn-primary"
                :disabled="!complete || busy"
                data-test="submit-button"
                @click="handleSubmit"
              >
                <svg v-if="submitting" class="-ml-1 mr-2 h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
                <Icon v-else name="checkCircle" size="md" class="mr-1" />
                {{ submitting ? t('reimbursement.submitting') : t('reimbursement.submitButton') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Card 3: my requests -->
      <div ref="listSection" class="card" data-test="request-list">
        <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('reimbursement.myRequestsTitle') }}
          </h2>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="loadingList"
            :title="t('common.refresh')"
            @click="loadRequests"
          >
            <Icon name="refresh" size="sm" :class="loadingList ? 'animate-spin' : ''" />
          </button>
        </div>
        <DataTable :columns="columns" :data="requests" :loading="loadingList" row-key="id">
          <template #cell-index="{ row }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ rowIndex(row) }}</span>
          </template>
          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>
          <template #cell-company_name="{ value }">
            <span class="text-sm text-gray-900 dark:text-white">{{ value }}</span>
          </template>
          <template #cell-tax_id="{ value }">
            <span class="font-mono text-sm text-gray-700 dark:text-gray-300">{{ value }}</span>
          </template>
          <template #cell-amount="{ value }">
            <span class="text-sm font-medium text-gray-900 dark:text-white">{{ formatAmount(value) }}</span>
          </template>
          <template #cell-status="{ value }">
            <span :class="['badge', value === 'completed' ? 'badge-success' : 'badge-warning']" data-test="status-badge">
              {{ t(`reimbursement.status.${value}`) }}
            </span>
          </template>
          <template #cell-actions="{ row }">
            <button
              v-if="row.status === 'completed' && row.pdf_available"
              type="button"
              class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-primary-600 hover:bg-primary-50 disabled:opacity-50 dark:text-primary-400 dark:hover:bg-primary-900/20"
              :disabled="downloadingId === row.id"
              data-test="download-button"
              @click="handleDownload(row)"
            >
              <Icon name="download" size="sm" />
              {{ t('reimbursement.downloadPdf') }}
            </button>
            <span v-else class="text-xs text-gray-400 dark:text-dark-500">—</span>
          </template>
          <template #empty>
            <EmptyState :title="t('reimbursement.noRequests')" :description="t('reimbursement.noRequestsDescription')">
              <template #icon>
                <Icon name="document" class="empty-state-icon h-10 w-10" />
              </template>
            </EmptyState>
          </template>
        </DataTable>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import {
  reimbursementAPI,
  REIMBURSEMENT_FIELD_KEYS,
  type ReimbursementFieldKey,
  type ReimbursementFields,
  type ReimbursementParseResult,
  type ReimbursementRequest
} from '@/api/reimbursement'
import { saveBlob } from '@/api/batchImage'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import TextArea from '@/components/common/TextArea.vue'
import Icon from '@/components/icons/Icon.vue'

const DRAFT_STORAGE_KEY = 'reimbursement_draft'

interface ReimbursementDraft {
  text: string
  rawText: string
  fields: ReimbursementFields
  missing: ReimbursementFieldKey[]
  complete: boolean
  updatedAt: string
}

const { t } = useI18n()
const appStore = useAppStore()

const fieldKeys = REIMBURSEMENT_FIELD_KEYS

// ===== Parse state =====
const text = ref('')
const rawText = ref('')
const parseResult = ref<ReimbursementParseResult | null>(null)
const parsing = ref(false)
const supplementText = ref('')
const supplementing = ref(false)
const submitting = ref(false)
const draftRestored = ref(false)
// 三个动作共用同一份解析结果，任一进行中都要锁住其它入口，否则并发返回会互相覆盖
const busy = computed(() => parsing.value || supplementing.value || submitting.value)

const fields = computed<ReimbursementFields | null>(() => parseResult.value?.fields ?? null)
const missing = computed<ReimbursementFieldKey[]>(() => parseResult.value?.missing ?? [])
const complete = computed(() => !!parseResult.value && parseResult.value.complete && missing.value.length === 0)
const missingLabels = computed(() => missing.value.map((key) => t(`reimbursement.fields.${key}`)).join('、'))

function isMissing(key: ReimbursementFieldKey): boolean {
  return missing.value.includes(key)
}

function displayValue(key: ReimbursementFieldKey): string {
  const value = fields.value?.[key]
  if (value === null || value === undefined || value === '') return ''
  if (key === 'amount') return formatAmount(value as number)
  return String(value)
}

function formatAmount(value: number | string | null | undefined): string {
  const num = Number(value)
  if (!Number.isFinite(num)) return ''
  return `¥${num.toFixed(2)}`
}

// ===== Draft persistence =====
// 隐私模式下 localStorage 会抛异常，所有读写都要吞掉
function saveDraft() {
  if (!parseResult.value) return
  try {
    const draft: ReimbursementDraft = {
      text: text.value,
      rawText: rawText.value,
      fields: parseResult.value.fields,
      missing: parseResult.value.missing,
      complete: parseResult.value.complete,
      updatedAt: new Date().toISOString()
    }
    window.localStorage.setItem(DRAFT_STORAGE_KEY, JSON.stringify(draft))
  } catch (error) {
    console.warn('Failed to persist reimbursement draft:', error)
  }
}

function clearDraft() {
  try {
    window.localStorage.removeItem(DRAFT_STORAGE_KEY)
  } catch (error) {
    console.warn('Failed to clear reimbursement draft:', error)
  }
}

function restoreDraft() {
  try {
    const stored = window.localStorage.getItem(DRAFT_STORAGE_KEY)
    if (!stored) return
    const draft = JSON.parse(stored) as Partial<ReimbursementDraft>
    if (!draft || typeof draft !== 'object' || !draft.fields) return
    text.value = typeof draft.text === 'string' ? draft.text : ''
    rawText.value = typeof draft.rawText === 'string' ? draft.rawText : text.value
    const missingKeys = Array.isArray(draft.missing)
      ? draft.missing.filter((key): key is ReimbursementFieldKey => fieldKeys.includes(key as ReimbursementFieldKey))
      : []
    parseResult.value = {
      fields: draft.fields,
      missing: missingKeys,
      complete: !!draft.complete && missingKeys.length === 0,
      notes: null
    }
    draftRestored.value = true
    appStore.showInfo(t('reimbursement.draftRestored'))
  } catch (error) {
    console.warn('Failed to restore reimbursement draft:', error)
  }
}

// ===== Actions =====
async function handleParse() {
  const input = text.value.trim()
  if (!input) {
    appStore.showWarning(t('reimbursement.textRequired'))
    return
  }
  parsing.value = true
  try {
    const result = await reimbursementAPI.parse(input)
    parseResult.value = result
    rawText.value = input
    supplementText.value = ''
    draftRestored.value = false
    saveDraft()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('reimbursement.parseFailed')))
  } finally {
    parsing.value = false
  }
}

async function handleSupplement() {
  const supplement = supplementText.value.trim()
  if (!supplement || !fields.value) return
  supplementing.value = true
  try {
    const result = await reimbursementAPI.parse(supplement, fields.value)
    parseResult.value = result
    rawText.value = rawText.value ? `${rawText.value}\n${supplement}` : supplement
    supplementText.value = ''
    saveDraft()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('reimbursement.parseFailed')))
  } finally {
    supplementing.value = false
  }
}

async function handleSubmit() {
  const current = fields.value
  if (!complete.value || !current) return
  submitting.value = true
  try {
    await reimbursementAPI.submit({
      company_name: current.company_name ?? '',
      tax_id: current.tax_id ?? '',
      bank_account: current.bank_account ?? '',
      bank_name: current.bank_name ?? '',
      address: current.address ?? '',
      amount: Number(current.amount ?? 0),
      raw_text: rawText.value || undefined
    })
    appStore.showSuccess(t('reimbursement.submitSuccess'))
    resetState()
    clearDraft()
    pagination.page = 1
    await loadRequests()
    scrollToList()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('reimbursement.submitFailed')))
  } finally {
    submitting.value = false
  }
}

function handleRestart() {
  resetState()
  clearDraft()
}

function resetState() {
  text.value = ''
  rawText.value = ''
  parseResult.value = null
  supplementText.value = ''
  draftRestored.value = false
}

const listSection = ref<HTMLElement | null>(null)
function scrollToList() {
  const el = listSection.value
  if (el && typeof el.scrollIntoView === 'function') {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
}

// ===== My requests =====
const requests = ref<ReimbursementRequest[]>([])
const loadingList = ref(false)
const downloadingId = ref<number | null>(null)
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const columns = computed<Column[]>(() => [
  { key: 'index', label: t('reimbursement.columns.index') },
  { key: 'created_at', label: t('reimbursement.columns.submittedAt') },
  { key: 'company_name', label: t('reimbursement.columns.companyName') },
  { key: 'tax_id', label: t('reimbursement.columns.taxId') },
  { key: 'amount', label: t('reimbursement.columns.amount') },
  { key: 'status', label: t('reimbursement.columns.status') },
  { key: 'actions', label: t('reimbursement.columns.actions') }
])

function rowIndex(row: ReimbursementRequest): number {
  return (pagination.page - 1) * pagination.page_size + requests.value.indexOf(row) + 1
}

async function loadRequests() {
  loadingList.value = true
  try {
    const res = await reimbursementAPI.listMine(pagination.page, pagination.page_size)
    requests.value = res.items || []
    pagination.total = res.total || 0
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('reimbursement.loadFailed')))
  } finally {
    loadingList.value = false
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  loadRequests()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  loadRequests()
}

// 旧版 jsdom / 部分浏览器的 Blob 没有 text()，退回 FileReader
function readBlobText(blob: Blob): Promise<string> {
  if (typeof blob.text === 'function') return blob.text()
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '')
    reader.onerror = () => reject(reader.error ?? new Error('blob_read_failed'))
    reader.readAsText(blob)
  })
}

async function handleDownload(row: ReimbursementRequest) {
  downloadingId.value = row.id
  try {
    const blob = await reimbursementAPI.downloadPdf(row.id)
    // 后端出错时 JSON 错误体也会被包成 Blob，不能直接当 PDF 保存
    if (blob.type === 'application/json') {
      let message = t('reimbursement.downloadFailed')
      try {
        const parsed = JSON.parse(await readBlobText(blob)) as { message?: string }
        if (parsed?.message) message = parsed.message
      } catch {
        // 错误体不是合法 JSON 时用默认文案
      }
      appStore.showError(message)
      return
    }
    saveBlob(blob, row.pdf_file_name || `invoice-${row.id}.pdf`)
  } catch (error) {
    let message = extractApiErrorMessage(error, t('reimbursement.downloadFailed'))
    // 拦截器还原不了 Blob 错误体时会落到 axios 原文，对用户无意义，换成兜底文案
    if (message.startsWith('Request failed with status code')) message = t('reimbursement.downloadFailed')
    appStore.showError(message)
  } finally {
    downloadingId.value = null
  }
}

onMounted(() => {
  restoreDraft()
  loadRequests()
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: all 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
