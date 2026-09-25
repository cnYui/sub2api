<template>
  <AppLayout>
    <div class="space-y-6">
      <section ref="editorRef" class="card">
        <div class="card-header flex flex-wrap items-center justify-between gap-3">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('admin.redeemCards.editor.title') }}
          </h2>
          <button type="button" class="btn btn-secondary btn-sm" data-test="open-profile" @click="openProfileDialog">
            <Icon name="edit" size="sm" />
            {{ t('admin.redeemCards.profile.button') }}
          </button>
        </div>

        <div class="card-body grid gap-8 xl:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
          <div class="min-w-0 space-y-5">
            <div>
              <label class="input-label">{{ t('admin.redeemCards.editor.codeLabel') }}</label>
              <div
                v-if="selectedCode"
                class="flex items-center gap-3 rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-800"
              >
                <div class="min-w-0 flex-1">
                  <code class="block truncate font-mono text-sm text-gray-900 dark:text-gray-100" data-test="selected-code">
                    {{ selectedCode.code }}
                  </code>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ describeCode(selectedCode) }}</p>
                </div>
                <button type="button" class="btn btn-secondary btn-sm flex-none" @click="clearSelection">
                  {{ t('admin.redeemCards.editor.change') }}
                </button>
              </div>
              <div v-else class="relative">
                <div class="flex flex-col gap-2 sm:flex-row">
                  <input
                    ref="codeInputRef"
                    v-model="codeQuery"
                    type="text"
                    class="input font-mono"
                    data-test="code-search"
                    :placeholder="t('admin.redeemCards.editor.codeSearchPlaceholder')"
                    @focus="openPicker"
                    @blur="closePickerSoon"
                    @input="onCodeQueryInput"
                  />
                  <button type="button" class="btn btn-secondary whitespace-nowrap" data-test="open-generate" @click="openGenerateDialog">
                    <Icon name="plus" size="sm" />
                    {{ t('admin.redeemCards.editor.generate') }}
                  </button>
                </div>
                <div
                  v-if="pickerOpen"
                  class="absolute left-0 right-0 z-20 mt-2 max-h-72 overflow-y-auto rounded-xl border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800"
                >
                  <div v-if="codeLoading" class="flex justify-center py-4">
                    <div class="h-5 w-5 animate-spin rounded-full border-b-2 border-primary-600"></div>
                  </div>
                  <template v-else>
                    <button
                      v-for="code in codeOptions"
                      :key="code.id"
                      type="button"
                      class="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left transition-colors hover:bg-gray-50 dark:hover:bg-dark-700"
                      data-test="code-option"
                      @mousedown.prevent="selectCode(code)"
                    >
                      <code class="truncate font-mono text-sm text-gray-900 dark:text-gray-100">{{ code.code }}</code>
                      <span class="flex-none text-xs text-gray-500 dark:text-dark-400">{{ describeCode(code) }}</span>
                    </button>
                    <p v-if="codeOptions.length === 0" class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">
                      {{ t('admin.redeemCards.editor.noCodes') }}
                    </p>
                  </template>
                </div>
              </div>
              <p v-if="savedCard" class="input-hint">{{ t('admin.redeemCards.editor.existingCard') }}</p>
            </div>

            <template v-if="selectedCode">
              <div>
                <label class="input-label">{{ t('admin.redeemCards.editor.theme') }}</label>
                <div class="inline-flex rounded-xl border border-gray-200 p-1 dark:border-dark-600">
                  <button
                    v-for="option in THEMES"
                    :key="option"
                    type="button"
                    :data-test="`theme-${option}`"
                    :class="segmentClass(form.theme === option)"
                    @click="form.theme = option"
                  >
                    <span
                      class="h-3 w-3 rounded-full border border-gray-300 dark:border-dark-500"
                      :class="option === 'dark' ? 'bg-[#0d1117]' : 'bg-white'"
                    ></span>
                    {{ t(`admin.redeemCards.editor.themes.${option}`) }}
                  </button>
                </div>
              </div>

              <div class="grid gap-4 sm:grid-cols-2">
                <div v-for="field in TEXT_FIELDS" :key="field">
                  <label class="input-label">{{ t(`admin.redeemCards.editor.fields.${field}`) }}</label>
                  <input v-model="form.content[field]" type="text" maxlength="64" class="input" :data-test="`field-${field}`" />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.redeemCards.editor.fields.serial') }}</label>
                  <input v-model="form.content.serial" type="text" maxlength="16" class="input" />
                  <p class="input-hint">{{ t('admin.redeemCards.editor.serialHint') }}</p>
                </div>
                <div>
                  <label class="input-label">{{ t('admin.redeemCards.editor.fields.heatmap_seed') }}</label>
                  <div class="flex gap-2">
                    <input v-model.number="form.content.heatmap_seed" type="number" min="1" max="999" class="input" />
                    <button type="button" class="btn btn-secondary whitespace-nowrap" @click="reshuffle">
                      <Icon name="refresh" size="sm" />
                      {{ t('admin.redeemCards.editor.reshuffle') }}
                    </button>
                  </div>
                </div>
              </div>

              <div class="flex flex-wrap gap-2">
                <button type="button" class="btn btn-secondary" @click="autofill">
                  <Icon name="sparkles" size="sm" />
                  {{ t('admin.redeemCards.editor.autofill') }}
                </button>
                <button
                  type="button"
                  class="btn btn-primary"
                  data-test="save-card"
                  :disabled="saving || loadingCard || (!!savedCard && !dirty)"
                  @click="saveCard"
                >
                  {{ saveLabel }}
                </button>
              </div>

              <div
                v-if="savedCard"
                class="rounded-xl border border-emerald-200 bg-emerald-50/70 p-4 dark:border-emerald-800/60 dark:bg-emerald-900/10"
              >
                <label class="input-label">{{ t('admin.redeemCards.editor.shareLink') }}</label>
                <div class="flex flex-col gap-2 sm:flex-row">
                  <input
                    :value="shareLink"
                    readonly
                    class="input font-mono text-xs"
                    data-test="share-link"
                    @focus="selectInput"
                  />
                  <div class="flex flex-none gap-2">
                    <button type="button" class="btn btn-primary whitespace-nowrap" @click="copyLink(savedCard.token)">
                      <Icon name="copy" size="sm" />
                      {{ t('admin.redeemCards.editor.copyLink') }}
                    </button>
                    <a class="btn btn-secondary whitespace-nowrap" :href="shareLink" target="_blank" rel="noopener">
                      <Icon name="externalLink" size="sm" />
                      {{ t('admin.redeemCards.editor.openLink') }}
                    </a>
                  </div>
                </div>
              </div>
            </template>
          </div>

          <div class="min-w-0">
            <div class="mb-3 flex justify-end">
              <div class="inline-flex rounded-xl border border-gray-200 p-1 dark:border-dark-600">
                <button type="button" :class="segmentClass(previewMode === 'flat')" @click="previewMode = 'flat'">
                  {{ t('admin.redeemCards.editor.flat') }}
                </button>
                <button type="button" :class="segmentClass(previewMode === '3d')" data-test="preview-3d" @click="previewMode = '3d'">
                  <Icon name="cube" size="sm" />
                  {{ t('admin.redeemCards.editor.threeD') }}
                </button>
              </div>
            </div>
            <template v-if="previewData">
              <div v-if="previewMode === 'flat'" class="space-y-4">
                <RedeemCardScaled :data="previewData" side="front" />
                <RedeemCardScaled :data="previewData" side="back" :stamp="previewStamp" />
              </div>
              <div v-else class="aspect-[16/10] overflow-hidden rounded-2xl bg-[#e9e8e4]">
                <RedeemCard3D :data="previewData" :stamp="previewStamp" :zoomable="false" />
              </div>
            </template>
            <div
              v-else
              class="flex aspect-[1712/1080] items-center justify-center rounded-2xl border border-dashed border-gray-300 px-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400"
            >
              {{ t('admin.redeemCards.editor.emptyPreview') }}
            </div>
          </div>
        </div>
      </section>

      <section class="card">
        <div class="card-header flex flex-wrap items-center justify-between gap-3">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.redeemCards.list.title') }}</h2>
          <div class="flex items-center gap-2">
            <input
              v-model="listQuery"
              type="text"
              class="input w-56"
              :placeholder="t('admin.redeemCards.list.searchPlaceholder')"
              @input="onListSearch"
            />
            <button type="button" class="btn btn-secondary" :disabled="listLoading" :title="t('common.refresh')" @click="loadCards">
              <Icon name="refresh" size="md" :class="listLoading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>

        <DataTable :columns="columns" :data="cards" :loading="listLoading">
          <template #cell-code="{ value }">
            <code class="font-mono text-sm text-gray-900 dark:text-gray-100">{{ value }}</code>
          </template>
          <template #cell-value="{ row }">
            <span class="text-sm text-gray-900 dark:text-white">{{ cardValueText(row) }}</span>
          </template>
          <template #cell-theme="{ value }">
            <span class="badge" :class="value === 'dark' ? 'badge-gray' : 'badge-primary'">
              {{ t(`admin.redeemCards.editor.themes.${value}`) }}
            </span>
          </template>
          <template #cell-code_status="{ value }">
            <span class="badge" :class="statusBadgeClass(value)">{{ statusText(value) }}</span>
          </template>
          <template #cell-view_count="{ value, row }">
            <span class="text-sm text-gray-700 dark:text-gray-300" :title="row.last_viewed_at ? t('admin.redeemCards.list.lastViewed', { time: formatDateTime(row.last_viewed_at) }) : ''">
              {{ value }}
            </span>
          </template>
          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button type="button" :class="rowActionClass" :title="t('admin.redeemCards.editor.copyLink')" @click="copyLink(row.token)">
                <Icon name="copy" size="sm" />
                <span class="text-xs">{{ t('admin.redeemCards.editor.copyLink') }}</span>
              </button>
              <button type="button" :class="rowActionClass" @click="editCard(row)">
                <Icon name="edit" size="sm" />
                <span class="text-xs">{{ t('admin.redeemCards.list.edit') }}</span>
              </button>
              <button type="button" :class="[rowActionClass, 'hover:!bg-red-50 hover:!text-red-600 dark:hover:!bg-red-900/20 dark:hover:!text-red-400']" @click="revoking = row">
                <Icon name="trash" size="sm" />
                <span class="text-xs">{{ t('admin.redeemCards.list.revoke') }}</span>
              </button>
            </div>
          </template>
        </DataTable>

        <div v-if="pagination.total > 0" class="px-6 pb-5">
          <Pagination
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </div>
      </section>
    </div>

    <BaseDialog :show="showGenerate" :title="t('admin.redeemCards.generate.title')" width="narrow" @close="showGenerate = false">
      <form id="redeem-card-generate" class="space-y-4" @submit.prevent="submitGenerate">
        <div>
          <label class="input-label">{{ t('admin.redeemCards.generate.type') }}</label>
          <div class="inline-flex rounded-xl border border-gray-200 p-1 dark:border-dark-600">
            <button
              v-for="type in GENERATE_TYPES"
              :key="type"
              type="button"
              :class="segmentClass(generateForm.type === type)"
              @click="generateForm.type = type"
            >
              {{ t(`admin.redeemCards.generate.types.${type}`) }}
            </button>
          </div>
        </div>
        <div v-if="generateForm.type === 'balance'">
          <label class="input-label">{{ t('admin.redeemCards.generate.amount') }}</label>
          <input v-model.number="generateForm.amount" type="number" step="0.01" min="0.01" class="input" />
        </div>
        <div v-else>
          <label class="input-label">{{ t('admin.redeemCards.generate.plan') }}</label>
          <Select
            v-model="generateForm.planId"
            :options="planOptions"
            :placeholder="t('admin.redeemCards.generate.planPlaceholder')"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.redeemCards.generate.expiry') }}</label>
          <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
            <button
              v-for="option in expiryOptions"
              :key="option.value"
              type="button"
              :class="[
                'rounded-lg border px-3 py-2 text-sm transition-colors',
                generateForm.expiry === option.value
                  ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-400 dark:bg-primary-900/20 dark:text-primary-300'
                  : 'border-gray-200 text-gray-700 hover:bg-gray-50 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700'
              ]"
              @click="generateForm.expiry = option.value"
            >
              {{ option.label }}
            </button>
          </div>
          <input
            v-if="generateForm.expiry === 'custom'"
            v-model.number="generateForm.customDays"
            type="number"
            min="1"
            max="3650"
            class="input mt-2"
            :placeholder="t('admin.redeemCards.generate.customDays')"
          />
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="showGenerate = false">{{ t('common.cancel') }}</button>
          <button type="submit" form="redeem-card-generate" class="btn btn-primary" :disabled="generating">
            {{ t('admin.redeemCards.generate.submit') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="showProfile" :title="t('admin.redeemCards.profile.title')" width="extra-wide" @close="showProfile = false">
      <p class="mb-5 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.redeemCards.profile.hint') }}</p>
      <div class="grid gap-6 lg:grid-cols-2">
        <div class="space-y-4">
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.redeemCards.profile.ownerLabel') }}</label>
              <input v-model="profileDraft.owner_label" type="text" maxlength="64" class="input" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.redeemCards.profile.ownerName') }}</label>
              <input v-model="profileDraft.owner_name" type="text" maxlength="64" class="input" />
            </div>
          </div>
          <div>
            <label class="input-label">{{ t('admin.redeemCards.profile.ownerLines') }}</label>
            <textarea v-model="profileDraft.owner_lines_text" rows="3" class="input"></textarea>
          </div>
          <div>
            <label class="input-label">{{ t('admin.redeemCards.profile.steps') }}</label>
            <textarea v-model="profileDraft.steps_text" rows="4" class="input"></textarea>
          </div>
          <div v-for="side in QR_SIDES" :key="side" class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
            <label class="input-label">
              {{ t(side === 'left' ? 'admin.redeemCards.profile.leftQr' : 'admin.redeemCards.profile.rightQr') }}
            </label>
            <div class="flex items-start gap-4">
              <img
                :src="profileDraft[qrImageKey(side)] || DEFAULT_QR_IMAGES[side]"
                alt=""
                class="h-20 w-20 flex-none rounded-lg border border-gray-200 bg-white object-contain p-1 dark:border-dark-600"
              />
              <div class="min-w-0 flex-1 space-y-2">
                <div class="flex flex-wrap gap-2">
                  <label class="btn btn-secondary btn-sm cursor-pointer">
                    <Icon name="upload" size="sm" />
                    {{ t('admin.redeemCards.profile.upload') }}
                    <input type="file" accept="image/png,image/jpeg,image/webp" class="hidden" @change="onQrFile(side, $event)" />
                  </label>
                  <button
                    v-if="profileDraft[qrImageKey(side)]"
                    type="button"
                    class="btn btn-ghost btn-sm"
                    @click="profileDraft[qrImageKey(side)] = ''"
                  >
                    {{ t('admin.redeemCards.profile.useDefault') }}
                  </button>
                </div>
                <p class="input-hint !mt-0">{{ t('admin.redeemCards.profile.imageHint') }}</p>
                <input
                  v-model="profileDraft[qrCaptionKey(side)]"
                  type="text"
                  maxlength="64"
                  class="input"
                  :placeholder="t('admin.redeemCards.profile.caption')"
                />
              </div>
            </div>
          </div>
        </div>
        <div class="space-y-4">
          <RedeemCardScaled :data="profilePreviewData" side="front" />
          <RedeemCardScaled :data="profilePreviewData" side="back" />
        </div>
      </div>
      <template #footer>
        <div class="flex flex-wrap items-center justify-between gap-2">
          <button type="button" class="btn btn-ghost" @click="resetProfileDraft">
            {{ t('admin.redeemCards.profile.resetAll') }}
          </button>
          <div class="flex gap-2">
            <button type="button" class="btn btn-secondary" @click="showProfile = false">{{ t('common.cancel') }}</button>
            <button type="button" class="btn btn-primary" data-test="save-profile" :disabled="savingProfile" @click="saveProfile">
              {{ t('common.save') }}
            </button>
          </div>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="!!revoking"
      :title="t('admin.redeemCards.list.revoke')"
      :message="t('admin.redeemCards.list.revokeConfirm')"
      :confirm-text="t('admin.redeemCards.list.revoke')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmRevoke"
      @cancel="revoking = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import type { AdminRedeemCard } from '@/api/admin/redeemCards'
import { formatDateTime } from '@/utils/format'
import type { RedeemCode } from '@/types'
import type { BalancePackagePlan } from '@/types/payment'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import RedeemCardScaled from '@/components/redeemCard/RedeemCardScaled.vue'
import RedeemCard3D from '@/components/redeemCard/RedeemCard3D.vue'
import {
  DEFAULT_HEATMAP_SEED,
  DEFAULT_QR_IMAGES,
  codeStatusStamp,
  contentFromRedeemCode,
  defaultRedeemCardProfile,
  emptyRedeemCardContent,
  type RedeemCardContent,
  type RedeemCardData,
  type RedeemCardProfile,
  type RedeemCardTheme
} from '@/components/redeemCard/redeemCardModel'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const THEMES: RedeemCardTheme[] = ['dark', 'light']
const TEXT_FIELDS = ['amount', 'plan', 'tokens', 'valid_until'] as const
const GENERATE_TYPES = ['balance', 'balance_package'] as const
const QR_SIDES = ['left', 'right'] as const
const QR_IMAGE_MAX_BYTES = 512 * 1024
const QR_IMAGE_TYPES = ['image/png', 'image/jpeg', 'image/webp']
const PROFILE_MAX_OWNER_LINES = 4
const PROFILE_MAX_STEPS = 5

// 还没选兑换码时，卡面信息弹窗用这组示例值预览。
const SAMPLE_CODE = 'a3f9c2e17b4d58e0c6a1f93b2d7e4c85'
const SAMPLE_CONTENT: RedeemCardContent = {
  amount: '$100',
  plan: '余额充值',
  tokens: '',
  valid_until: '长期有效',
  serial: '0001',
  heatmap_seed: DEFAULT_HEATMAP_SEED
}

const rowActionClass =
  'flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200'

function segmentClass(active: boolean) {
  return [
    'inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium transition-colors',
    active
      ? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-dark-950'
      : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'
  ]
}

function randomSeed() {
  return Math.floor(Math.random() * 999) + 1
}

function errorMessage(error: unknown, fallback: string) {
  const message = (error as { message?: string } | null)?.message
  return message || fallback
}

// ---------------- 卡面信息 ----------------

const profile = ref<RedeemCardProfile>(defaultRedeemCardProfile())
const showProfile = ref(false)
const savingProfile = ref(false)
const profileDraft = reactive({
  owner_label: '',
  owner_name: '',
  owner_lines_text: '',
  steps_text: '',
  left_qr_image: '',
  left_qr_caption: '',
  right_qr_image: '',
  right_qr_caption: ''
})

function splitLines(text: string, max: number) {
  return text
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .slice(0, max)
}

function fillProfileDraft(source: RedeemCardProfile) {
  profileDraft.owner_label = source.owner_label
  profileDraft.owner_name = source.owner_name
  profileDraft.owner_lines_text = source.owner_lines.join('\n')
  profileDraft.steps_text = source.steps.join('\n')
  profileDraft.left_qr_image = source.left_qr_image
  profileDraft.left_qr_caption = source.left_qr_caption
  profileDraft.right_qr_image = source.right_qr_image
  profileDraft.right_qr_caption = source.right_qr_caption
}

const draftProfile = computed<RedeemCardProfile>(() => ({
  owner_label: profileDraft.owner_label.trim(),
  owner_name: profileDraft.owner_name.trim(),
  owner_lines: splitLines(profileDraft.owner_lines_text, PROFILE_MAX_OWNER_LINES),
  steps: splitLines(profileDraft.steps_text, PROFILE_MAX_STEPS),
  left_qr_image: profileDraft.left_qr_image,
  left_qr_caption: profileDraft.left_qr_caption.trim(),
  right_qr_image: profileDraft.right_qr_image,
  right_qr_caption: profileDraft.right_qr_caption.trim()
}))

function openProfileDialog() {
  fillProfileDraft(profile.value)
  showProfile.value = true
}

function resetProfileDraft() {
  fillProfileDraft(defaultRedeemCardProfile())
}

type QRSide = (typeof QR_SIDES)[number]

function qrImageKey(side: QRSide) {
  return side === 'left' ? ('left_qr_image' as const) : ('right_qr_image' as const)
}

function qrCaptionKey(side: QRSide) {
  return side === 'left' ? ('left_qr_caption' as const) : ('right_qr_caption' as const)
}

function onQrFile(side: QRSide, event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!QR_IMAGE_TYPES.includes(file.type)) {
    appStore.showError(t('admin.redeemCards.profile.imageInvalid'))
    return
  }
  if (file.size > QR_IMAGE_MAX_BYTES) {
    appStore.showError(t('admin.redeemCards.profile.imageTooLarge'))
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    if (typeof reader.result === 'string') profileDraft[qrImageKey(side)] = reader.result
  }
  reader.readAsDataURL(file)
}

async function loadProfile() {
  try {
    profile.value = await adminAPI.redeemCards.getProfile()
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.redeemCards.errors.loadProfile')))
  }
}

async function saveProfile() {
  savingProfile.value = true
  try {
    profile.value = await adminAPI.redeemCards.updateProfile(draftProfile.value)
    showProfile.value = false
    appStore.showSuccess(t('admin.redeemCards.profile.saved'))
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.redeemCards.errors.saveProfile')))
  } finally {
    savingProfile.value = false
  }
}

// ---------------- 编辑器 ----------------

// 余额套餐档位：生成兑换码时选档，自动填写卡面时补档位信息。
const plans = ref<BalancePackagePlan[]>([])

const editorRef = ref<HTMLElement | null>(null)
const codeInputRef = ref<HTMLInputElement | null>(null)
const selectedCode = ref<RedeemCode | null>(null)
const savedCard = ref<AdminRedeemCard | null>(null)
const loadingCard = ref(false)
const saving = ref(false)
const previewMode = ref<'flat' | '3d'>('flat')
const form = reactive<{ theme: RedeemCardTheme; content: RedeemCardContent }>({
  theme: 'dark',
  content: emptyRedeemCardContent()
})
const savedSnapshot = ref('')

const snapshotOf = (theme: RedeemCardTheme, content: RedeemCardContent) => JSON.stringify({ theme, content })
const dirty = computed(() => snapshotOf(form.theme, form.content) !== savedSnapshot.value)

const saveLabel = computed(() => {
  if (!savedCard.value) return t('admin.redeemCards.editor.save')
  return dirty.value ? t('admin.redeemCards.editor.update') : t('admin.redeemCards.editor.saved')
})

const shareLink = computed(() => (savedCard.value ? adminAPI.redeemCards.shareUrl(savedCard.value.token) : ''))

const previewData = computed<RedeemCardData | null>(() =>
  selectedCode.value
    ? { theme: form.theme, code: selectedCode.value.code, content: form.content, profile: profile.value }
    : null
)

const previewStamp = computed(() => codeStatusStamp(savedCard.value?.code_status ?? selectedCode.value?.status))

const profilePreviewData = computed<RedeemCardData>(() => ({
  theme: form.theme,
  code: selectedCode.value?.code ?? SAMPLE_CODE,
  content: selectedCode.value ? form.content : SAMPLE_CONTENT,
  profile: draftProfile.value
}))

// 列表接口只带档位 ID 时，从已加载的档位里补上名称和到账规则，自动填写才完整。
function withPlan(code: RedeemCode): RedeemCode {
  if (code.type !== 'balance_package' || code.balance_package_plan || !code.balance_package_plan_id) return code
  const plan = plans.value.find((item) => item.id === code.balance_package_plan_id)
  return plan ? { ...code, balance_package_plan: plan } : code
}

function describeCode(code: RedeemCode) {
  const type = t(`admin.redeem.types.${code.type}`)
  switch (code.type) {
    case 'balance':
      return `${type} $${Number(code.value).toFixed(2)}`
    case 'balance_package':
      return code.balance_package_plan ? `${type} · ${code.balance_package_plan.name}` : type
    case 'subscription':
      return code.group ? `${type} · ${code.group.name}` : type
    default:
      return `${type} ${code.value}`
  }
}

function applySavedCard(card: AdminRedeemCard) {
  savedCard.value = card
  form.theme = card.theme
  form.content = { ...emptyRedeemCardContent(), ...card.content }
  savedSnapshot.value = snapshotOf(form.theme, form.content)
}

async function selectCode(code: RedeemCode) {
  pickerOpen.value = false
  selectedCode.value = withPlan(code)
  savedCard.value = null
  savedSnapshot.value = ''
  form.content = contentFromRedeemCode(selectedCode.value, randomSeed())
  loadingCard.value = true
  try {
    const existing = await adminAPI.redeemCards.getByCode(code.id)
    if (existing && selectedCode.value?.id === code.id) applySavedCard(existing)
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.redeemCards.errors.load')))
  } finally {
    loadingCard.value = false
  }
}

async function clearSelection() {
  selectedCode.value = null
  savedCard.value = null
  savedSnapshot.value = ''
  codeQuery.value = ''
  await nextTick()
  codeInputRef.value?.focus()
}

function autofill() {
  if (!selectedCode.value) return
  form.content = contentFromRedeemCode(selectedCode.value, form.content.heatmap_seed || randomSeed())
}

function reshuffle() {
  form.content.heatmap_seed = randomSeed()
}

function normalizedContent(): RedeemCardContent {
  const seed = Math.round(Number(form.content.heatmap_seed) || DEFAULT_HEATMAP_SEED)
  return {
    amount: form.content.amount.trim(),
    plan: form.content.plan.trim(),
    tokens: form.content.tokens.trim(),
    valid_until: form.content.valid_until.trim(),
    serial: form.content.serial.trim(),
    heatmap_seed: Math.min(999, Math.max(1, seed))
  }
}

async function saveCard() {
  if (!selectedCode.value) return
  saving.value = true
  try {
    const card = await adminAPI.redeemCards.save({
      redeem_code_id: selectedCode.value.id,
      theme: form.theme,
      content: normalizedContent()
    })
    applySavedCard(card)
    appStore.showSuccess(t('admin.redeemCards.editor.saved'))
    void loadCards()
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.redeemCards.errors.save')))
  } finally {
    saving.value = false
  }
}

function selectInput(event: FocusEvent) {
  const input = event.target as HTMLInputElement
  input.select()
}

async function copyLink(token: string) {
  await copyToClipboard(adminAPI.redeemCards.shareUrl(token), t('admin.redeemCards.editor.linkCopied'))
}

// ---------------- 兑换码选择 ----------------

const codeQuery = ref('')
const codeOptions = ref<RedeemCode[]>([])
const codeLoading = ref(false)
const pickerOpen = ref(false)
let codeSearchTimer: ReturnType<typeof setTimeout> | undefined
let closePickerTimer: ReturnType<typeof setTimeout> | undefined
let codeRequestSeq = 0

async function loadCodeOptions() {
  const seq = ++codeRequestSeq
  codeLoading.value = true
  try {
    const result = await adminAPI.redeem.list(1, 20, {
      status: 'unused',
      search: codeQuery.value.trim() || undefined,
      sort_by: 'id',
      sort_order: 'desc'
    })
    if (seq !== codeRequestSeq) return
    // 邀请码不带额度，做成兑换卡没有意义。
    codeOptions.value = result.items.filter((code) => code.type !== 'invitation').map(withPlan)
  } catch (error) {
    if (seq === codeRequestSeq) appStore.showError(errorMessage(error, t('admin.redeemCards.errors.loadCodes')))
  } finally {
    if (seq === codeRequestSeq) codeLoading.value = false
  }
}

function openPicker() {
  clearTimeout(closePickerTimer)
  pickerOpen.value = true
  void loadCodeOptions()
}

function closePickerSoon() {
  closePickerTimer = setTimeout(() => {
    pickerOpen.value = false
  }, 150)
}

function onCodeQueryInput() {
  pickerOpen.value = true
  clearTimeout(codeSearchTimer)
  codeSearchTimer = setTimeout(() => void loadCodeOptions(), 250)
}

// ---------------- 生成兑换码 ----------------

type ExpiryOption = 'never' | '7' | '30' | 'custom'

const showGenerate = ref(false)
const generating = ref(false)
const generateForm = reactive({
  type: 'balance' as (typeof GENERATE_TYPES)[number],
  amount: 10,
  planId: null as number | null,
  expiry: 'never' as ExpiryOption,
  customDays: 30
})

const planOptions = computed(() =>
  plans.value.map((plan) => ({
    value: plan.id,
    label: `${plan.name} · ¥${plan.price_cny} · $${plan.weekly_credit_usd} × ${plan.refresh_count}`
  }))
)

const expiryOptions = computed<{ value: ExpiryOption; label: string }[]>(() => [
  { value: 'never', label: t('admin.redeemCards.generate.never') },
  { value: '7', label: t('admin.redeemCards.generate.days', { days: 7 }) },
  { value: '30', label: t('admin.redeemCards.generate.days', { days: 30 }) },
  { value: 'custom', label: t('admin.redeemCards.generate.custom') }
])

async function loadPlans() {
  try {
    const { data } = await adminAPI.payment.getBalancePackages()
    plans.value = data || []
  } catch {
    plans.value = []
  }
}

function openGenerateDialog() {
  showGenerate.value = true
  if (plans.value.length === 0) void loadPlans()
}

async function submitGenerate() {
  const { type } = generateForm
  if (type === 'balance' && !(Number(generateForm.amount) > 0)) {
    appStore.showError(t('admin.redeemCards.generate.amountRequired'))
    return
  }
  if (type === 'balance_package' && !generateForm.planId) {
    appStore.showError(t('admin.redeemCards.generate.planRequired'))
    return
  }
  let expiresInDays: number | undefined
  if (generateForm.expiry === 'custom') {
    const days = Number(generateForm.customDays)
    if (!Number.isInteger(days) || days < 1) {
      appStore.showError(t('admin.redeemCards.generate.expiryRequired'))
      return
    }
    expiresInDays = days
  } else if (generateForm.expiry !== 'never') {
    expiresInDays = Number(generateForm.expiry)
  }

  generating.value = true
  try {
    const generated = await adminAPI.redeem.generate(
      1,
      type,
      type === 'balance' ? Number(generateForm.amount) : 0,
      undefined,
      undefined,
      expiresInDays,
      type === 'balance_package' ? generateForm.planId : undefined
    )
    const created = generated[0]
    if (!created) throw new Error(t('admin.redeemCards.errors.generate'))
    const code = await adminAPI.redeem.getById(created.id).catch(() => created)
    showGenerate.value = false
    await selectCode(code)
    appStore.showSuccess(t('admin.redeemCards.generate.generated'))
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.redeemCards.errors.generate')))
  } finally {
    generating.value = false
  }
}

// ---------------- 已制作的卡片 ----------------

const cards = ref<AdminRedeemCard[]>([])
const listLoading = ref(false)
const listQuery = ref('')
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const revoking = ref<AdminRedeemCard | null>(null)
let listSearchTimer: ReturnType<typeof setTimeout> | undefined
let listRequestSeq = 0

const columns = computed<Column[]>(() => [
  { key: 'code', label: t('admin.redeemCards.list.columns.code') },
  { key: 'value', label: t('admin.redeemCards.list.columns.value') },
  { key: 'theme', label: t('admin.redeemCards.list.columns.theme') },
  { key: 'code_status', label: t('admin.redeemCards.list.columns.status') },
  { key: 'view_count', label: t('admin.redeemCards.list.columns.views') },
  { key: 'created_at', label: t('admin.redeemCards.list.columns.createdAt') },
  { key: 'actions', label: t('admin.redeemCards.list.columns.actions') }
])

function cardValueText(card: AdminRedeemCard) {
  return [card.content.amount, card.content.plan].filter(Boolean).join(' · ') || '-'
}

function statusText(status: string) {
  return t(`admin.redeemCards.codeStatus.${status}`, status)
}

function statusBadgeClass(status: string) {
  if (status === 'unused') return 'badge-success'
  if (status === 'used') return 'badge-gray'
  return 'badge-danger'
}

async function loadCards() {
  const seq = ++listRequestSeq
  listLoading.value = true
  try {
    const result = await adminAPI.redeemCards.list(pagination.page, pagination.page_size, listQuery.value.trim())
    if (seq !== listRequestSeq) return
    cards.value = result.items
    pagination.total = result.total
  } catch (error) {
    if (seq === listRequestSeq) appStore.showError(errorMessage(error, t('admin.redeemCards.errors.load')))
  } finally {
    if (seq === listRequestSeq) listLoading.value = false
  }
}

function onListSearch() {
  clearTimeout(listSearchTimer)
  listSearchTimer = setTimeout(() => {
    pagination.page = 1
    void loadCards()
  }, 300)
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadCards()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadCards()
}

async function editCard(card: AdminRedeemCard) {
  try {
    const code = await adminAPI.redeem.getById(card.redeem_code_id)
    await selectCode(code)
    editorRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.redeemCards.errors.load')))
  }
}

async function confirmRevoke() {
  const card = revoking.value
  if (!card) return
  try {
    await adminAPI.redeemCards.remove(card.id)
    revoking.value = null
    if (savedCard.value?.id === card.id) {
      savedCard.value = null
      savedSnapshot.value = ''
    }
    appStore.showSuccess(t('admin.redeemCards.list.revoked'))
    void loadCards()
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.redeemCards.errors.revoke')))
  }
}

onMounted(() => {
  void loadProfile()
  void loadCards()
  void loadPlans()
})

onUnmounted(() => {
  clearTimeout(codeSearchTimer)
  clearTimeout(closePickerTimer)
  clearTimeout(listSearchTimer)
})
</script>
