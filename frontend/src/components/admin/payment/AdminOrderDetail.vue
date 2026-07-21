<template>
  <BaseDialog
    :show="show"
    :title="t('payment.admin.orderDetail')"
    width="wide"
    @close="emit('close')"
  >
    <div v-if="order" class="space-y-4">
      <div class="grid grid-cols-2 gap-4">
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</p>
          <p class="font-mono text-sm font-medium text-gray-900 dark:text-white">#{{ order.id }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.status') }}</p>
          <span :class="['badge', statusBadgeClass(order.status)]">
            {{ t('payment.status.' + order.status.toLowerCase(), order.status) }}
          </span>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.baseAmount') }}</p>
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ paymentAmountSymbol }}{{ baseAmount.toFixed(2) }}</p>
        </div>
        <div v-if="order.fee_rate > 0">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.fee') }} ({{ order.fee_rate }}%)</p>
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ paymentAmountSymbol }}{{ feeAmount.toFixed(2) }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.payAmount') }}</p>
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ paymentAmountSymbol }}{{ order.pay_amount.toFixed(2) }}</p>
        </div>
        <div v-if="order.amount !== order.pay_amount">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.creditedAmount') }}</p>
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ creditedAmountSymbol }}{{ order.amount.toFixed(2) }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.paymentMethod') }}</p>
          <p class="text-sm text-gray-700 dark:text-gray-300">
            {{ t('payment.methods.' + order.payment_type, order.payment_type) }}
          </p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.orderType') }}</p>
          <p class="text-sm text-gray-700 dark:text-gray-300">
            {{ t('payment.admin.' + order.order_type + 'Order', order.order_type) }}
          </p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.userId') }}</p>
          <p class="text-sm text-gray-700 dark:text-gray-300">#{{ order.user_id }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.createdAt') }}</p>
          <p class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(order.created_at) }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.expiresAt') }}</p>
          <p class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(order.expires_at) }}</p>
        </div>
        <div v-if="order.paid_at">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.paidAt') }}</p>
          <p class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(order.paid_at) }}</p>
        </div>
        <div v-if="order.completed_at">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.completedAt') }}</p>
          <p class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(order.completed_at) }}</p>
        </div>
      </div>

      <div
        v-if="order.refund_amount"
        class="rounded-lg border border-red-200 bg-red-50 p-3 dark:border-red-800 dark:bg-red-900/20"
      >
        <h4 class="mb-2 text-sm font-semibold text-red-700 dark:text-red-400">
          {{ t('payment.admin.refundInfo') }}
        </h4>
        <div class="grid grid-cols-2 gap-2 text-sm">
          <div>
            <span class="text-red-600 dark:text-red-400">{{ t('payment.admin.refundAmount') }}:</span>
            <span class="ml-1 font-medium text-red-700 dark:text-red-300">{{ creditedAmountSymbol }}{{ order.refund_amount.toFixed(2) }}</span>
          </div>
          <div v-if="order.refund_reason" class="col-span-2">
            <span class="text-red-600 dark:text-red-400">{{ t('payment.admin.refundReason') }}:</span>
            <span class="ml-1 text-red-700 dark:text-red-300">{{ order.refund_reason }}</span>
          </div>
        </div>
      </div>

      <div
        v-if="hasSubscriptionSnapshot"
        class="rounded-lg border border-blue-200 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-900/20"
      >
        <h4 class="mb-2 text-sm font-semibold text-blue-700 dark:text-blue-300">
          {{ t('payment.admin.subscriptionSnapshot') }}
        </h4>
        <div class="grid grid-cols-2 gap-2 text-sm">
          <div v-if="snapshotString('plan_name')">
            <span class="text-blue-600 dark:text-blue-400">{{ t('payment.admin.planName') }}:</span>
            <span class="ml-1 text-blue-800 dark:text-blue-200">{{ snapshotString('plan_name') }}</span>
          </div>
          <div v-if="snapshotNumber('group_id')">
            <span class="text-blue-600 dark:text-blue-400">{{ t('payment.admin.groupId') }}:</span>
            <span class="ml-1 text-blue-800 dark:text-blue-200">#{{ snapshotNumber('group_id') }}</span>
          </div>
          <div v-if="snapshotNumber('validity_days')">
            <span class="text-blue-600 dark:text-blue-400">{{ t('payment.admin.validityDays') }}:</span>
            <span class="ml-1 text-blue-800 dark:text-blue-200">{{ snapshotNumber('validity_days') }}</span>
          </div>
          <div v-if="snapshotNumber('weekly_limit_usd')">
            <span class="text-blue-600 dark:text-blue-400">{{ t('payment.planCard.weeklyLimit') }}:</span>
            <span class="ml-1 text-blue-800 dark:text-blue-200">{{ formatSubscriptionQuotaUSD(snapshotNumber('weekly_limit_usd')) }}</span>
          </div>
          <div v-if="snapshotNumber('period_total_quota_usd')">
            <span class="text-blue-600 dark:text-blue-400">{{ t('payment.admin.periodTotalQuota') }}:</span>
            <span class="ml-1 text-blue-800 dark:text-blue-200">{{ formatSubscriptionQuotaUSD(snapshotNumber('period_total_quota_usd')) }}</span>
          </div>
          <div v-if="snapshotString('quota_window_unit')">
            <span class="text-blue-600 dark:text-blue-400">{{ t('payment.admin.quotaWindow') }}:</span>
            <span class="ml-1 text-blue-800 dark:text-blue-200">{{ snapshotString('quota_window_unit') }} / {{ snapshotNumber('quota_window_days') || '-' }}</span>
          </div>
        </div>
      </div>

      <div
        v-if="hasRefundBasis"
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-900/20"
      >
        <h4 class="mb-2 text-sm font-semibold text-amber-700 dark:text-amber-300">
          {{ t('payment.admin.refundBasis') }}
        </h4>
        <div class="grid grid-cols-2 gap-2 text-sm">
          <div v-if="basisNumber('entitlement_period_id')">
            <span class="text-amber-600 dark:text-amber-400">{{ t('payment.admin.entitlementPeriod') }}:</span>
            <span class="ml-1 text-amber-800 dark:text-amber-200">#{{ basisNumber('entitlement_period_id') }}</span>
          </div>
          <div>
            <span class="text-amber-600 dark:text-amber-400">{{ t('payment.refundQuote.purchaseBase') }}:</span>
            <span class="ml-1 text-amber-800 dark:text-amber-200">{{ creditedAmountSymbol }}{{ basisNumber('purchase_base_amount').toFixed(2) }}</span>
          </div>
          <div>
            <span class="text-amber-600 dark:text-amber-400">{{ t('payment.refundQuote.nonRefundableFee') }}:</span>
            <span class="ml-1 text-amber-800 dark:text-amber-200">{{ paymentAmountSymbol }}{{ basisNumber('non_refundable_fee').toFixed(2) }}</span>
          </div>
          <div>
            <span class="text-amber-600 dark:text-amber-400">{{ t('payment.refundQuote.periodQuotaUsage') }}:</span>
            <span class="ml-1 text-amber-800 dark:text-amber-200">{{ formatSubscriptionQuotaUSD(basisNumber('used_quota_usd')) }} / {{ formatSubscriptionQuotaUSD(basisNumber('period_total_quota_usd')) }}</span>
          </div>
          <div>
            <span class="text-amber-600 dark:text-amber-400">{{ t('payment.refundQuote.usageRatio') }}:</span>
            <span class="ml-1 text-amber-800 dark:text-amber-200">{{ Math.round(basisNumber('usage_ratio') * 100) }}%</span>
          </div>
          <div v-if="basisString('calculated_at')" class="col-span-2">
            <span class="text-amber-600 dark:text-amber-400">{{ t('payment.admin.calculatedAt') }}:</span>
            <span class="ml-1 text-amber-800 dark:text-amber-200">{{ formatDateTime(basisString('calculated_at')) }}</span>
          </div>
          <div v-if="order.force_refund" class="col-span-2 text-red-700 dark:text-red-300">
            {{ t('payment.admin.forceRefundAudit') }}
          </div>
        </div>
      </div>

      <div class="flex items-center justify-end gap-2 border-t border-gray-200 pt-4 dark:border-dark-700">
        <button
          v-if="order.status === 'PENDING'"
          @click="emit('cancel', order)"
          class="btn btn-sm rounded-md bg-yellow-50 px-3 py-1.5 text-sm text-yellow-600 hover:bg-yellow-100 dark:bg-yellow-900/20 dark:text-yellow-400 dark:hover:bg-yellow-900/30"
        >
          {{ t('payment.orders.cancel') }}
        </button>
        <button
          v-if="order.status === 'FAILED'"
          @click="emit('retry', order)"
          class="btn btn-sm btn-secondary"
        >
          {{ t('payment.admin.retry') }}
        </button>
        <button
          v-if="canRefund(order)"
          @click="emit('refund', order)"
          class="btn btn-sm rounded-md bg-red-50 px-3 py-1.5 text-sm text-red-600 hover:bg-red-100 dark:bg-red-900/20 dark:text-red-400 dark:hover:bg-red-900/30"
        >
          {{ t('payment.admin.refund') }}
        </button>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { PaymentOrder } from '@/types/payment'
import { statusBadgeClass, canRefund as canRefundStatus, isAutomaticRefundAllowed, formatOrderDateTime } from '@/components/payment/orderUtils'
import { currencySymbol } from '@/components/payment/currency'
import { formatSubscriptionQuotaUSD } from '@/utils/subscriptionQuota'

const { t } = useI18n()

const props = defineProps<{
  show: boolean
  order: PaymentOrder | null
}>()

const creditedAmountSymbol = computed(() => currencySymbol(props.order?.currency || 'CNY'))

const paymentAmountSymbol = computed(() => currencySymbol(props.order?.currency))

/** 充值金额 (base amount before fee) = pay_amount - fee = pay_amount / (1 + fee_rate/100) */
const baseAmount = computed(() => {
  if (!props.order) return 0
  const feeRate = Number(props.order.fee_rate) || 0
  if (feeRate <= 0) return props.order.pay_amount
  return props.order.pay_amount / (1 + feeRate / 100)
})

/** 手续费 = pay_amount - baseAmount */
const feeAmount = computed(() => {
  if (!props.order) return 0
  const feeRate = Number(props.order.fee_rate) || 0
  if (feeRate <= 0) return 0
  return props.order.pay_amount - baseAmount.value
})

const hasSubscriptionSnapshot = computed(() => !!props.order?.subscription_snapshot && Object.keys(props.order.subscription_snapshot).length > 0)

const hasRefundBasis = computed(() => !!props.order?.refund_basis && Object.keys(props.order.refund_basis).length > 0)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'cancel', order: PaymentOrder): void
  (e: 'retry', order: PaymentOrder): void
  (e: 'refund', order: PaymentOrder): void
}>()

function canRefund(order: PaymentOrder): boolean {
  return isAutomaticRefundAllowed(order.payment_type) && canRefundStatus(order.status, order.refund_retryable === true)
}

function formatDateTime(dateStr: string): string {
  return formatOrderDateTime(dateStr)
}

function snapshotString(key: string): string {
  const value = props.order?.subscription_snapshot?.[key]
  return typeof value === 'string' ? value : ''
}

function snapshotNumber(key: string): number {
  return toFiniteNumber(props.order?.subscription_snapshot?.[key])
}

function basisString(key: string): string {
  const value = props.order?.refund_basis?.[key]
  return typeof value === 'string' ? value : ''
}

function basisNumber(key: string): number {
  return toFiniteNumber(props.order?.refund_basis?.[key])
}

function toFiniteNumber(value: unknown): number {
  const num = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(num) ? num : 0
}
</script>
