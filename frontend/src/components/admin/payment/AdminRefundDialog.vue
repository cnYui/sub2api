<template>
  <BaseDialog
    :show="show"
    :title="t('payment.admin.refundOrder')"
    width="normal"
    @close="emit('cancel')"
  >
    <form id="refund-form" @submit.prevent="handleSubmit" class="space-y-4">
      <!-- Refund Request Info -->
      <div
        v-if="order?.refund_requested_at || order?.refund_request_reason"
        class="rounded-lg border border-violet-200 bg-violet-50 p-3 dark:border-violet-800 dark:bg-violet-900/20"
      >
        <div class="flex items-center gap-2 text-sm font-medium text-violet-700 dark:text-violet-300">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          {{ t('payment.admin.refundRequestInfo') }}
        </div>
        <div v-if="order?.refund_requested_at" class="mt-2 flex justify-between text-sm">
          <span class="text-violet-600 dark:text-violet-400">{{ t('payment.admin.refundRequestedAt') }}</span>
          <span class="text-violet-800 dark:text-violet-200">{{ formatDateTime(order.refund_requested_at) }}</span>
        </div>
        <div v-if="order?.refund_request_reason" class="mt-1 text-sm">
          <span class="text-violet-600 dark:text-violet-400">{{ t('payment.admin.refundRequestReason') }}:</span>
          <span class="ml-1 text-violet-800 dark:text-violet-200">{{ order.refund_request_reason }}</span>
        </div>
      </div>

      <!-- Order Info -->
      <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
        <div class="flex justify-between text-sm">
          <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</span>
          <span class="font-mono text-gray-900 dark:text-white">#{{ order?.id }}</span>
        </div>
        <div class="mt-1 flex justify-between text-sm">
          <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.creditedAmount') }}</span>
          <span class="font-medium text-gray-900 dark:text-white">{{ creditedAmountSymbol }}{{ order?.amount?.toFixed(2) }}</span>
        </div>
        <div class="mt-1 flex justify-between text-sm">
          <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.payAmount') }}</span>
          <span class="font-medium text-gray-900 dark:text-white">{{ paymentAmountSymbol }}{{ order?.pay_amount?.toFixed(2) }}</span>
        </div>
      </div>

      <!-- Live Refund Quote: the server recalculates it again on confirm -->
      <div class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-700">
        <p v-if="quoteLoading" class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.refundQuote.loading') }}</p>
        <p v-else-if="quoteError" class="text-xs text-red-600 dark:text-red-400">{{ quoteError }}</p>
        <template v-else-if="quote">
          <div v-if="!quote.manual_review_required" class="space-y-1">
            <div class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.refundQuote.periodQuotaUsage') }}</span>
              <span class="text-gray-900 dark:text-white">{{ creditedAmountSymbol }}{{ quote.used_quota_usd.toFixed(2) }} / {{ creditedAmountSymbol }}{{ quote.period_total_quota_usd.toFixed(2) }}</span>
            </div>
            <div v-if="retainedQuota > 0" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.refundQuote.retainedQuota') }}</span>
              <span class="text-gray-900 dark:text-white">{{ creditedAmountSymbol }}{{ retainedQuota.toFixed(2) }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.refundQuote.usageRatio') }}</span>
              <span class="text-gray-900 dark:text-white">{{ Math.round(quote.usage_ratio * 100) }}%</span>
            </div>
            <div class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.refundQuote.timeRatio') }}</span>
              <span class="text-gray-900 dark:text-white">{{ Math.round(quote.time_ratio * 100) }}%</span>
            </div>
            <div class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.refundQuote.consumptionRatio') }}</span>
              <span class="text-gray-900 dark:text-white">{{ Math.round(quote.consumption_ratio * 100) }}%</span>
            </div>
            <div v-if="reclaimQuota > 0" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.refundQuote.reclaimQuota') }}</span>
              <span class="text-gray-900 dark:text-white">{{ creditedAmountSymbol }}{{ reclaimQuota.toFixed(2) }}</span>
            </div>
          </div>
          <div class="mt-2 flex justify-between font-medium">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.admin.refundAmount') }}</span>
            <span class="text-gray-900 dark:text-white">{{ creditedAmountSymbol }}{{ quote.estimated_refund_amount.toFixed(2) }}</span>
          </div>
          <p v-if="quote.manual_review_required" class="mt-2 text-xs text-amber-600 dark:text-amber-400">{{ t('payment.refundQuote.manualReviewRequired') }}</p>
          <template v-else>
            <p v-if="quote.estimated_refund_amount <= 0" class="mt-2 text-xs text-amber-600 dark:text-amber-400">{{ t('payment.refundQuote.zeroRefundCancelsPackage') }}</p>
            <p v-if="reclaimQuota > 0" class="mt-2 text-xs text-amber-600 dark:text-amber-400">{{ t('payment.admin.refundReclaimNotice', { amount: creditedAmountSymbol + reclaimQuota.toFixed(2) }) }}</p>
            <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.refundQuoteLive') }}</p>
          </template>
        </template>
        <div v-if="lastAttemptedRefund > 0" class="mt-2 flex justify-between text-xs text-gray-500 dark:text-gray-400">
          <span>{{ t('payment.admin.lastAttemptedRefund') }}</span>
          <span>{{ creditedAmountSymbol }}{{ lastAttemptedRefund.toFixed(2) }}</span>
        </div>
      </div>

      <!-- Reason -->
      <div>
        <label class="input-label">{{ t('payment.admin.refundReason') }}</label>
        <textarea
          v-model="form.reason"
          rows="3"
          class="input"
          :placeholder="t('payment.admin.refundReasonPlaceholder')"
          required
        ></textarea>
      </div>

    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" @click="emit('cancel')" class="btn btn-secondary">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="refund-form"
          :disabled="submitting || !canConfirm"
          class="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 disabled:opacity-50 dark:focus:ring-offset-dark-800"
        >
          {{ submitting ? t('common.processing') : t('payment.admin.confirmRefund') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { BalancePackageRefundQuote, PaymentOrder } from '@/types/payment'
import { formatOrderDateTime } from '@/components/payment/orderUtils'
import { currencySymbol } from '@/components/payment/currency'

const { t } = useI18n()

const props = defineProps<{
  show: boolean
  order: PaymentOrder | null
  submitting?: boolean
}>()

const emit = defineEmits<{
  (e: 'confirm', data: { reason: string }): void
  (e: 'cancel'): void
}>()

const creditedAmountSymbol = currencySymbol('USD')

const paymentAmountSymbol = computed(() => currencySymbol(props.order?.currency))

const form = reactive({
  reason: '',
})

// 订单上存的是上次尝试（或用户申请时）的金额；真正退多少以实时报价为准。
const lastAttemptedRefund = computed(() => props.order?.refund_amount || 0)

const quote = ref<BalancePackageRefundQuote | null>(null)
const quoteLoading = ref(false)
const quoteError = ref('')
let quoteRequestSeq = 0

const retainedQuota = computed(() => quote.value?.retained_quota_usd ?? 0)
const reclaimQuota = computed(() => quote.value?.reclaim_quota_usd ?? 0)
const canConfirm = computed(() => !quoteLoading.value && !!quote.value && !quote.value.manual_review_required)

async function loadQuote(orderId: number) {
  const seq = ++quoteRequestSeq
  quote.value = null
  quoteError.value = ''
  quoteLoading.value = true
  try {
    const res = await adminPaymentAPI.getRefundQuote(orderId)
    if (seq === quoteRequestSeq) quote.value = res.data
  } catch (err: unknown) {
    if (seq === quoteRequestSeq) {
      quoteError.value = extractI18nErrorMessage(err, t, 'payment.errors', t('payment.admin.refundQuoteFailed'))
    }
  } finally {
    if (seq === quoteRequestSeq) quoteLoading.value = false
  }
}

watch(
  () => [props.show, props.order?.id] as const,
  ([show]) => {
    if (!show || !props.order) return
    form.reason = props.order.refund_request_reason || ''
    void loadQuote(props.order.id)
  },
  { immediate: true },
)

function formatDateTime(dateStr: string): string {
  return formatOrderDateTime(dateStr)
}

function handleSubmit() {
  if (props.submitting || !canConfirm.value) return
  emit('confirm', { ...form })
}
</script>
