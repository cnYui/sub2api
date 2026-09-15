<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full md:w-72">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.reimbursements.searchPlaceholder')"
              class="input pl-10"
              data-test="search-input"
              @input="handleSearch"
            />
          </div>
          <Select
            v-model="filters.status"
            :options="statusFilterOptions"
            class="w-40"
            data-test="status-select"
            @change="handleStatusChange"
          />

          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="loading"
              :title="t('common.refresh')"
              data-test="refresh-button"
              @click="loadRequests"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button type="button" class="btn btn-primary" data-test="config-open" @click="openConfigDialog">
              <Icon name="cog" size="md" class="mr-1" />
              {{ t('admin.reimbursements.parserSettings') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="requests"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="asc"
          row-key="id"
          @sort="handleSort"
        >
          <template #cell-index="{ row }">
            <span class="text-sm text-gray-500 dark:text-dark-400" data-test="row-index">{{ rowIndex(row) }}</span>
          </template>
          <template #cell-user_email="{ value, row }">
            <span class="text-sm text-gray-900 dark:text-white">{{ value || row.username || '#' + row.user_id }}</span>
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
          <template #cell-bank_account="{ value }">
            <span class="font-mono text-sm text-gray-700 dark:text-gray-300">{{ value }}</span>
          </template>
          <template #cell-bank_name="{ value }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ value }}</span>
          </template>
          <template #cell-address="{ value }">
            <span class="block max-w-xs truncate text-sm text-gray-700 dark:text-gray-300" :title="value">{{ value }}</span>
          </template>
          <template #cell-amount="{ value }">
            <span class="text-sm font-medium text-gray-900 dark:text-white">{{ formatAmount(value) }}</span>
          </template>
          <template #cell-status="{ value }">
            <span :class="['badge', value === 'completed' ? 'badge-success' : 'badge-warning']">
              {{ t(`admin.reimbursements.statusLabels.${value}`) }}
            </span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center space-x-1">
              <button
                type="button"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
                :title="t('admin.reimbursements.viewRaw')"
                data-test="view-raw"
                @click="openRawDialog(row)"
              >
                <Icon name="eye" size="sm" />
              </button>
              <button
                type="button"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-emerald-50 hover:text-emerald-600 dark:hover:bg-emerald-900/20 dark:hover:text-emerald-400"
                :title="row.status === 'completed' ? t('admin.reimbursements.reuploadPdf') : t('admin.reimbursements.uploadPdf')"
                data-test="upload-open"
                @click="openUploadDialog(row)"
              >
                <Icon name="upload" size="sm" />
              </button>
              <button
                v-if="row.pdf_available"
                type="button"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-primary-900/20 dark:hover:text-primary-400"
                :title="t('admin.reimbursements.downloadPdf')"
                :disabled="downloadingId === row.id"
                data-test="download-pdf"
                @click="handleDownload(row)"
              >
                <Icon name="download" size="sm" />
              </button>
            </div>
          </template>
          <template #empty>
            <EmptyState :title="t('admin.reimbursements.noData')">
              <template #icon>
                <Icon name="document" class="empty-state-icon h-10 w-10" />
              </template>
            </EmptyState>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Raw text dialog -->
    <BaseDialog
      :show="showRawDialog"
      :title="t('admin.reimbursements.rawDialogTitle')"
      width="wide"
      @close="showRawDialog = false"
    >
      <div v-if="rawTarget" class="space-y-5">
        <dl class="grid grid-cols-1 gap-x-6 gap-y-3 text-sm sm:grid-cols-2">
          <div>
            <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.reimbursements.userLabel') }}</dt>
            <dd class="text-gray-900 dark:text-white">{{ userLabel(rawTarget) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.reimbursements.submittedAtLabel') }}</dt>
            <dd class="text-gray-900 dark:text-white">{{ formatDateTime(rawTarget.created_at) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.reimbursements.completedAtLabel') }}</dt>
            <dd class="text-gray-900 dark:text-white">
              {{ rawTarget.completed_at ? formatDateTime(rawTarget.completed_at) : t('admin.reimbursements.none') }}
            </dd>
          </div>
          <div>
            <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.reimbursements.pdfFileLabel') }}</dt>
            <dd class="text-gray-900 dark:text-white">
              {{ rawTarget.pdf_available ? `${rawTarget.pdf_file_name} (${formatFileSize(rawTarget.pdf_size)})` : t('admin.reimbursements.none') }}
            </dd>
          </div>
          <div>
            <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.reimbursements.pdfUploadedAtLabel') }}</dt>
            <dd class="text-gray-900 dark:text-white">
              {{ rawTarget.pdf_uploaded_at ? formatDateTime(rawTarget.pdf_uploaded_at) : t('admin.reimbursements.none') }}
            </dd>
          </div>
          <div>
            <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.reimbursements.notifiedAtLabel') }}</dt>
            <dd class="text-gray-900 dark:text-white">
              {{ rawTarget.notified_at ? formatDateTime(rawTarget.notified_at) : t('admin.reimbursements.notNotified') }}
            </dd>
          </div>
        </dl>

        <div>
          <h4 class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.reimbursements.fieldsLabel') }}
          </h4>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div v-for="key in fieldKeys" :key="key" :class="key === 'address' ? 'sm:col-span-2' : ''">
              <label class="input-label">{{ t(`admin.reimbursements.fields.${key}`) }}</label>
              <div class="input bg-gray-50 dark:bg-dark-800" :class="key === 'tax_id' || key === 'bank_account' ? 'font-mono' : ''">
                {{ key === 'amount' ? formatAmount(rawTarget.amount) : rawTarget[key] }}
              </div>
            </div>
          </div>
        </div>

        <div>
          <h4 class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.reimbursements.rawTextLabel') }}
          </h4>
          <pre
            class="max-h-72 overflow-auto whitespace-pre-wrap rounded-xl border border-gray-200 bg-gray-50 p-4 font-sans text-sm text-gray-800 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200"
            data-test="raw-text"
          >{{ rawTarget.raw_text || t('admin.reimbursements.rawTextEmpty') }}</pre>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button type="button" class="btn btn-secondary" @click="showRawDialog = false">
            {{ t('common.close') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Upload PDF dialog -->
    <BaseDialog
      :show="showUploadDialog"
      :title="t('admin.reimbursements.uploadDialogTitle')"
      width="narrow"
      @close="closeUploadDialog"
    >
      <div v-if="uploadTarget" class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-dark-300">
          {{ t('admin.reimbursements.uploadTarget', { id: uploadTarget.id, company: uploadTarget.company_name }) }}
        </p>
        <div
          class="flex cursor-pointer items-center justify-between gap-3 rounded-lg border border-dashed px-4 py-4 transition-colors"
          :class="dragActive
            ? 'border-primary-400 bg-primary-50/70 dark:border-primary-500 dark:bg-primary-900/20'
            : 'border-gray-300 bg-gray-50 dark:border-dark-600 dark:bg-dark-800'"
          data-test="upload-dropzone"
          @click="openFilePicker"
          @dragenter.prevent="handleDragEnter"
          @dragover.prevent
          @dragleave.prevent="handleDragLeave"
          @drop.prevent="handleDrop"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-gray-700 dark:text-dark-200" data-test="upload-file-name">
              {{ selectedFile ? selectedFile.name : t('admin.reimbursements.dropHint') }}
            </div>
            <div class="text-xs text-gray-500 dark:text-dark-400">
              <template v-if="selectedFile">{{ formatFileSize(selectedFile.size) }}</template>
              <template v-else>{{ t('admin.reimbursements.pdfOnly', { size: MAX_PDF_MB }) }}</template>
            </div>
          </div>
          <button type="button" class="btn btn-secondary shrink-0" :disabled="uploading" @click.stop="openFilePicker">
            {{ t('common.chooseFile') }}
          </button>
        </div>
        <input
          ref="fileInput"
          type="file"
          class="hidden"
          accept="application/pdf,.pdf"
          data-test="upload-file-input"
          @change="handleFileChange"
        />
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" :disabled="uploading" @click="closeUploadDialog">
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="!selectedFile || uploading"
            data-test="upload-submit"
            @click="handleUpload"
          >
            <Icon name="upload" size="md" class="mr-1" />
            {{ uploading ? t('admin.reimbursements.uploading') : t('admin.reimbursements.upload') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Parser config dialog -->
    <BaseDialog
      :show="showConfigDialog"
      :title="t('admin.reimbursements.config.title')"
      width="normal"
      @close="closeConfigDialog"
    >
      <form id="reimbursement-config-form" class="space-y-4" @submit.prevent="handleSaveConfig">
        <!-- 配置还没回来时表单里是上一次的值，先锁住避免管理员编辑到一半被 applyConfigView 覆盖 -->
        <p v-if="loadingConfig" class="text-sm text-gray-500 dark:text-dark-400" data-test="config-loading">
          {{ t('admin.reimbursements.config.loading') }}
        </p>
        <div>
          <label class="input-label">{{ t('admin.reimbursements.config.baseUrl') }}</label>
          <input
            v-model="configForm.base_url"
            type="text"
            class="input"
            :disabled="loadingConfig"
            data-test="config-base-url"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.reimbursements.config.model') }}</label>
          <input v-model="configForm.model" type="text" class="input" :disabled="loadingConfig" data-test="config-model" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.reimbursements.config.timeoutMs') }}</label>
          <input
            v-model.number="configForm.timeout_ms"
            type="number"
            min="5000"
            max="120000"
            step="1000"
            class="input"
            :disabled="loadingConfig"
            data-test="config-timeout-ms"
          />
          <p class="input-hint">{{ t('admin.reimbursements.config.timeoutHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.reimbursements.config.apiKey') }}</label>
          <input
            v-model="configForm.api_key"
            type="password"
            class="input"
            autocomplete="new-password"
            :placeholder="apiKeyPlaceholder"
            :disabled="configForm.clear_api_key || loadingConfig"
            data-test="config-api-key"
          />
          <p class="input-hint">
            {{ t('admin.reimbursements.config.apiKeyHint') }}
            <span v-if="configView">
              · {{ t('admin.reimbursements.config.apiKeySource') }}: {{ apiKeySourceLabel }}
            </span>
          </p>
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input
            v-model="configForm.clear_api_key"
            type="checkbox"
            class="rounded border-gray-300 dark:border-dark-600"
            :disabled="loadingConfig"
            data-test="config-clear-api-key"
          />
          {{ t('admin.reimbursements.config.clearApiKey') }}
        </label>

        <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
          <h4 class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.reimbursements.config.testTitle') }}
          </h4>
          <textarea
            v-model="testText"
            rows="4"
            class="input"
            :placeholder="t('admin.reimbursements.config.testTextPlaceholder')"
            data-test="config-test-text"
          ></textarea>
          <div class="mt-2 flex justify-end">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="testing || savingConfig"
              data-test="config-test"
              @click="handleTestConfig"
            >
              <Icon name="beaker" size="md" class="mr-1" />
              {{ testing ? t('admin.reimbursements.config.testing') : t('admin.reimbursements.config.test') }}
            </button>
          </div>
          <div
            v-if="testResult"
            class="mt-3 space-y-2 rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm dark:border-dark-600 dark:bg-dark-800"
            data-test="config-test-result"
          >
            <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
              <span>{{ t('admin.reimbursements.config.latency', { ms: testResult.latency_ms }) }}</span>
              <span>{{ t('admin.reimbursements.config.testModel', { model: testResult.model }) }}</span>
            </div>
            <p
              v-if="testResult.result.complete"
              class="text-emerald-700 dark:text-emerald-400"
            >
              {{ t('admin.reimbursements.config.complete') }}
            </p>
            <p v-else class="text-amber-700 dark:text-amber-400">
              {{ t('admin.reimbursements.config.missingFields', { fields: testMissingLabels }) }}
            </p>
            <dl class="grid grid-cols-1 gap-x-4 gap-y-1 sm:grid-cols-2">
              <div v-for="key in fieldKeys" :key="key" class="flex gap-2">
                <dt class="shrink-0 text-gray-500 dark:text-dark-400">{{ t(`admin.reimbursements.fields.${key}`) }}:</dt>
                <dd class="min-w-0 truncate text-gray-900 dark:text-white">
                  {{ testResult.result.fields[key] ?? t('admin.reimbursements.none') }}
                </dd>
              </div>
            </dl>
            <p v-if="testResult.result.notes" class="text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.reimbursements.config.notes', { notes: testResult.result.notes }) }}
            </p>
          </div>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" :disabled="savingConfig" @click="closeConfigDialog">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="reimbursement-config-form"
            class="btn btn-primary"
            :disabled="savingConfig || loadingConfig"
            data-test="config-save"
          >
            {{ savingConfig ? t('admin.reimbursements.config.saving') : t('admin.reimbursements.config.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import type {
  AdminReimbursementRequest,
  ReimbursementConfigTestResult,
  ReimbursementLLMConfigView
} from '@/api/admin/reimbursements'
import { REIMBURSEMENT_FIELD_KEYS, type ReimbursementStatus } from '@/api/reimbursement'
import { saveBlob } from '@/api/batchImage'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import type { Column } from '@/components/common/types'

import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'

const MAX_PDF_MB = 20
const MAX_PDF_BYTES = MAX_PDF_MB * 1024 * 1024

const { t } = useI18n()
const appStore = useAppStore()

const fieldKeys = REIMBURSEMENT_FIELD_KEYS

// ===== List state =====
const requests = ref<AdminReimbursementRequest[]>([])
const loading = ref(false)
const searchQuery = ref('')
const filters = reactive<{ status: ReimbursementStatus | '' }>({ status: '' })
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
// 需求：按提交时间从早到晚排列，默认升序
const sortState = reactive<{ sort_by: string; sort_order: 'asc' | 'desc' }>({
  sort_by: 'created_at',
  sort_order: 'asc'
})

const statusFilterOptions = computed(() => [
  { value: '', label: t('admin.reimbursements.allStatus') },
  { value: 'pending', label: t('admin.reimbursements.statusLabels.pending') },
  { value: 'completed', label: t('admin.reimbursements.statusLabels.completed') }
])

const columns = computed<Column[]>(() => [
  { key: 'index', label: t('admin.reimbursements.columns.index') },
  { key: 'user_email', label: t('admin.reimbursements.columns.user') },
  { key: 'created_at', label: t('admin.reimbursements.columns.submittedAt'), sortable: true },
  { key: 'company_name', label: t('admin.reimbursements.columns.companyName') },
  { key: 'tax_id', label: t('admin.reimbursements.columns.taxId') },
  { key: 'bank_account', label: t('admin.reimbursements.columns.bankAccount') },
  { key: 'bank_name', label: t('admin.reimbursements.columns.bankName') },
  { key: 'address', label: t('admin.reimbursements.columns.address') },
  { key: 'amount', label: t('admin.reimbursements.columns.amount'), sortable: true },
  { key: 'status', label: t('admin.reimbursements.columns.status'), sortable: true },
  { key: 'actions', label: t('admin.reimbursements.columns.actions') }
])

function rowIndex(row: AdminReimbursementRequest): number {
  return (pagination.page - 1) * pagination.page_size + requests.value.indexOf(row) + 1
}

function userLabel(row: AdminReimbursementRequest): string {
  return row.user_email || row.username || `#${row.user_id}`
}

function formatAmount(value: number | string | null | undefined): string {
  const num = Number(value)
  if (!Number.isFinite(num)) return ''
  return `¥${num.toFixed(2)}`
}

function formatFileSize(bytes: number | null | undefined): string {
  const size = Number(bytes)
  if (!Number.isFinite(size) || size <= 0) return '0 B'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / (1024 * 1024)).toFixed(2)} MB`
}

let currentController: AbortController | null = null

async function loadRequests() {
  currentController?.abort()
  const requestController = new AbortController()
  currentController = requestController
  const { signal } = requestController

  try {
    loading.value = true
    const res = await adminAPI.reimbursements.list(
      pagination.page,
      pagination.page_size,
      {
        status: filters.status || undefined,
        search: searchQuery.value.trim() || undefined,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      { signal }
    )

    if (signal.aborted || currentController !== requestController) return

    requests.value = res.items || []
    pagination.total = res.total
    pagination.pages = res.pages
    pagination.page = res.page
    pagination.page_size = res.page_size
  } catch (error: unknown) {
    const err = error as { name?: string; code?: string } | null
    if (
      signal.aborted ||
      currentController !== requestController ||
      err?.name === 'AbortError' ||
      err?.code === 'ERR_CANCELED'
    ) {
      return
    }
    appStore.showError(extractApiErrorMessage(error, t('admin.reimbursements.failedToLoad')))
  } finally {
    if (currentController === requestController) {
      loading.value = false
      currentController = null
    }
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  loadRequests()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadRequests()
}

function handleStatusChange() {
  pagination.page = 1
  loadRequests()
}

function handleSort(key: string, order: 'asc' | 'desc') {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadRequests()
}

let searchDebounceTimer: number | null = null
function handleSearch() {
  if (searchDebounceTimer) window.clearTimeout(searchDebounceTimer)
  searchDebounceTimer = window.setTimeout(() => {
    pagination.page = 1
    loadRequests()
  }, 300)
}

// ===== Raw text dialog =====
const showRawDialog = ref(false)
const rawTarget = ref<AdminReimbursementRequest | null>(null)

function openRawDialog(row: AdminReimbursementRequest) {
  rawTarget.value = row
  showRawDialog.value = true
}

// ===== Upload dialog =====
const showUploadDialog = ref(false)
const uploadTarget = ref<AdminReimbursementRequest | null>(null)
const selectedFile = ref<File | null>(null)
const uploading = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const dragDepth = ref(0)
const dragActive = computed(() => dragDepth.value > 0)

function openUploadDialog(row: AdminReimbursementRequest) {
  uploadTarget.value = row
  selectedFile.value = null
  dragDepth.value = 0
  showUploadDialog.value = true
}

// 上传进行中禁止关闭，避免中断后状态不一致
function closeUploadDialog() {
  if (uploading.value) return
  showUploadDialog.value = false
  uploadTarget.value = null
  selectedFile.value = null
}

function openFilePicker() {
  if (uploading.value) return
  fileInput.value?.click()
}

function isPdfFile(file: File): boolean {
  return file.name.toLowerCase().endsWith('.pdf')
}

function setSelectedFile(sourceFiles: FileList | File[] | null | undefined) {
  if (uploading.value) return
  const incoming = Array.from(sourceFiles || [])
  const file = incoming[0]
  if (!file) return
  if (!isPdfFile(file)) {
    appStore.showError(t('admin.reimbursements.invalidFile'))
    return
  }
  if (file.size > MAX_PDF_BYTES) {
    appStore.showError(t('admin.reimbursements.fileTooLarge', { size: MAX_PDF_MB }))
    return
  }
  selectedFile.value = file
}

function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  setSelectedFile(target.files)
  target.value = ''
}

function handleDragEnter() {
  if (uploading.value) return
  dragDepth.value += 1
}

function handleDragLeave() {
  dragDepth.value = Math.max(0, dragDepth.value - 1)
}

function handleDrop(event: DragEvent) {
  dragDepth.value = 0
  if (uploading.value) return
  setSelectedFile(event.dataTransfer?.files)
}

async function handleUpload() {
  const target = uploadTarget.value
  const file = selectedFile.value
  if (!target || !file || uploading.value) return
  uploading.value = true
  try {
    await adminAPI.reimbursements.uploadPdf(target.id, file)
    appStore.showSuccess(t('admin.reimbursements.uploadSuccess'))
    uploading.value = false
    closeUploadDialog()
    await loadRequests()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.reimbursements.failedToUpload')))
  } finally {
    uploading.value = false
  }
}

// ===== Download =====
const downloadingId = ref<number | null>(null)

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

async function handleDownload(row: AdminReimbursementRequest) {
  downloadingId.value = row.id
  try {
    const blob = await adminAPI.reimbursements.downloadPdf(row.id)
    // 后端出错时 JSON 错误体也会被包成 Blob，不能直接当 PDF 保存
    if (blob.type === 'application/json') {
      let message = t('admin.reimbursements.downloadFailed')
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
    let message = extractApiErrorMessage(error, t('admin.reimbursements.downloadFailed'))
    // 拦截器还原不了 Blob 错误体时会落到 axios 原文，对用户无意义，换成兜底文案
    if (message.startsWith('Request failed with status code')) message = t('admin.reimbursements.downloadFailed')
    appStore.showError(message)
  } finally {
    downloadingId.value = null
  }
}

// ===== Parser config dialog =====
const showConfigDialog = ref(false)
const loadingConfig = ref(false)
const savingConfig = ref(false)
const testing = ref(false)
const configView = ref<ReimbursementLLMConfigView | null>(null)
const configForm = reactive({
  base_url: '',
  model: '',
  timeout_ms: 60000,
  api_key: '',
  clear_api_key: false
})
const testText = ref('')
const testResult = ref<ReimbursementConfigTestResult | null>(null)

const apiKeyPlaceholder = computed(() => {
  const view = configView.value
  if (view?.api_key_configured && view.api_key_masked) return view.api_key_masked
  return t('admin.reimbursements.config.apiKeyNotConfigured')
})

const apiKeySourceLabel = computed(() => {
  const source = configView.value?.api_key_source
  if (source === 'settings') return t('admin.reimbursements.config.apiKeySources.settings')
  if (source === 'env') return t('admin.reimbursements.config.apiKeySources.env')
  return t('admin.reimbursements.config.apiKeySources.none')
})

const testMissingLabels = computed(() =>
  (testResult.value?.result.missing ?? []).map((key) => t(`admin.reimbursements.fields.${key}`)).join('、')
)

function applyConfigView(view: ReimbursementLLMConfigView) {
  configView.value = view
  configForm.base_url = view.base_url
  configForm.model = view.model
  configForm.timeout_ms = view.timeout_ms
  configForm.api_key = ''
  configForm.clear_api_key = false
}

async function openConfigDialog() {
  showConfigDialog.value = true
  testResult.value = null
  loadingConfig.value = true
  try {
    applyConfigView(await adminAPI.reimbursements.getConfig())
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.reimbursements.config.loadFailed')))
  } finally {
    loadingConfig.value = false
  }
}

function closeConfigDialog() {
  if (savingConfig.value) return
  showConfigDialog.value = false
}

async function handleSaveConfig() {
  if (savingConfig.value) return
  savingConfig.value = true
  try {
    const payload: Parameters<typeof adminAPI.reimbursements.updateConfig>[0] = {
      base_url: configForm.base_url.trim(),
      model: configForm.model.trim(),
      timeout_ms: Number(configForm.timeout_ms)
    }
    if (configForm.clear_api_key) {
      payload.clear_api_key = true
    } else if (configForm.api_key.trim()) {
      payload.api_key = configForm.api_key.trim()
    }
    applyConfigView(await adminAPI.reimbursements.updateConfig(payload))
    appStore.showSuccess(t('admin.reimbursements.config.saveSuccess'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.reimbursements.config.saveFailed')))
  } finally {
    savingConfig.value = false
  }
}

async function handleTestConfig() {
  if (testing.value) return
  testing.value = true
  testResult.value = null
  try {
    testResult.value = await adminAPI.reimbursements.testConfig(testText.value.trim() || undefined)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.reimbursements.config.testFailed')))
  } finally {
    testing.value = false
  }
}

onMounted(() => {
  loadRequests()
})

onUnmounted(() => {
  if (searchDebounceTimer) window.clearTimeout(searchDebounceTimer)
  currentController?.abort()
})
</script>
