<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-gray-900 border-t-transparent dark:border-gray-100 dark:border-t-transparent"
        ></div>
      </div>

      <!-- Empty State -->
      <div v-else-if="subscriptions.length === 0 && !hasTrafficPackContent" class="card p-12 text-center">
        <div
          class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
        >
          <Icon name="creditCard" size="xl" class="text-gray-400" />
        </div>
        <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('userSubscriptions.noActiveSubscriptions') }}
        </h3>
        <p class="text-gray-500 dark:text-dark-400">
          {{ t('userSubscriptions.noActiveSubscriptionsDesc') }}
        </p>
      </div>

      <template v-else>
        <section v-if="hasTrafficPackContent" class="space-y-4">
          <div class="grid gap-6 lg:grid-cols-2">
            <SubscriptionUsageCard
              title="GPT 流量包"
              platform="openai"
              description="套餐额度用完后自动消耗，剩余额度不会每日刷新。"
              status="active"
              :status-label="t('userSubscriptions.status.active')"
              expiration-label="到期时间"
              :expiration-value="trafficCreditExpiration"
              expiration-value-class="text-gray-700 dark:text-gray-300"
              :usage-rows="trafficCreditUsageRows"
              :unlimited-title="t('userSubscriptions.unlimited')"
              :unlimited-description="t('userSubscriptions.unlimitedDesc')"
            />
          </div>
        </section>

        <!-- Subscriptions Grid -->
        <div v-if="subscriptions.length > 0" class="grid gap-6 lg:grid-cols-2">
          <SubscriptionUsageCard
            v-for="subscription in subscriptions"
            :key="subscription.id"
            :title="subscription.group?.name || `Group #${subscription.group_id}`"
            :platform="subscription.group?.platform || ''"
            :description="subscription.group?.description || ''"
            :status="subscription.status"
            :status-label="t(`userSubscriptions.status.${subscription.status}`)"
            :expiration-label="t('userSubscriptions.expires')"
            :expiration-value="formatSubscriptionExpiration(subscription)"
            :expiration-value-class="getExpirationClass(subscription.expires_at)"
            :usage-rows="subscriptionUsageRows(subscription)"
            :unlimited-title="t('userSubscriptions.unlimited')"
            :unlimited-description="t('userSubscriptions.unlimitedDesc')"
          />
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import subscriptionsAPI from '@/api/subscriptions'
import { paymentAPI } from '@/api/payment'
import type { UserSubscription } from '@/types'
import type { CheckoutInfoResponse } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import SubscriptionUsageCard from '@/components/user/SubscriptionUsageCard.vue'
import { formatDateOnly } from '@/utils/format'
import { grayProgressBarClass } from '@/utils/grayTheme'
import { getRemainingDurationParts, isOneTimeDailyQuota, type RemainingDurationParts } from '@/utils/subscriptionQuota'

interface SubscriptionUsageRow {
  label: string
  value: string
  progressWidth: string
  progressClass: string
  footer?: string
  testId?: string
}

const { t } = useI18n()
const appStore = useAppStore()

const subscriptions = ref<UserSubscription[]>([])
const checkout = ref<Pick<CheckoutInfoResponse, 'traffic_packs' | 'traffic_credit_summary'>>({
  traffic_packs: [],
  traffic_credit_summary: null,
})
const loading = ref(true)

const trafficCreditSummary = computed(() => checkout.value.traffic_credit_summary ?? null)
const hasTrafficPackContent = computed(() =>
  (trafficCreditSummary.value?.total_remaining_usd ?? 0) > 0
)
const trafficCreditExpiration = computed(() => '剩余 365 天')
const trafficCreditUsageRows = computed<SubscriptionUsageRow[]>(() => {
  const remaining = trafficCreditSummary.value?.total_remaining_usd ?? 0
  return [{
    label: '总计',
    value: `$0.00 / $${remaining.toFixed(2)}`,
    progressWidth: getProgressWidth(0, remaining),
    progressClass: getProgressBarClass(0, remaining),
    testId: 'traffic-credit-progress',
  }]
})

async function loadSubscriptions() {
  try {
    loading.value = true
    const [subscriptionList, checkoutInfo] = await Promise.all([
      subscriptionsAPI.getMySubscriptions(),
      paymentAPI.getCheckoutInfo(),
    ])
    subscriptions.value = subscriptionList
    checkout.value = {
      traffic_packs: checkoutInfo.data.traffic_packs ?? [],
      traffic_credit_summary: checkoutInfo.data.traffic_credit_summary ?? null,
    }
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function getProgressWidth(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return '0%'
  const percentage = Math.min(((used || 0) / limit) * 100, 100)
  return `${percentage}%`
}

function getProgressBarClass(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return 'bg-gray-400'
  const percentage = ((used || 0) / limit) * 100
  return grayProgressBarClass(percentage)
}

function formatExpirationDate(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (days < 0) {
    return t('userSubscriptions.status.expired')
  }

  const dateStr = formatDateOnly(expires)

  if (days === 0) {
    return `${dateStr} (${t('common.today')})`
  }
  if (days === 1) {
    return `${dateStr} (${t('common.tomorrow')})`
  }

  return t('userSubscriptions.daysRemaining', { days }) + ` (${dateStr})`
}

function formatSubscriptionExpiration(subscription: UserSubscription): string {
  if (!subscription.expires_at) return t('userSubscriptions.noExpiration')
  return formatExpirationDate(subscription.expires_at)
}

function getExpirationClass(expiresAt: string | null | undefined): string {
  if (!expiresAt) return 'text-gray-700 dark:text-gray-300'
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (days <= 0) return 'text-red-600 dark:text-red-400 font-medium'
  if (days <= 3) return 'text-red-600 dark:text-red-400'
  if (days <= 7) return 'text-orange-600 dark:text-orange-400'
  return 'text-gray-700 dark:text-gray-300'
}

function buildUsageRow(
  label: string,
  used: number | undefined,
  limit: number,
  footer?: string,
): SubscriptionUsageRow {
  return {
    label,
    value: `$${(used || 0).toFixed(2)} / $${limit.toFixed(2)}`,
    progressWidth: getProgressWidth(used, limit),
    progressClass: getProgressBarClass(used, limit),
    footer,
  }
}

function subscriptionUsageRows(subscription: UserSubscription): SubscriptionUsageRow[] {
  const rows: SubscriptionUsageRow[] = []
  if (subscription.group?.daily_limit_usd) {
    rows.push(buildUsageRow(
      t('userSubscriptions.daily'),
      subscription.daily_usage_usd,
      subscription.group.daily_limit_usd,
      subscription.daily_window_start ? formatDailyUsageWindow(subscription) : undefined,
    ))
  }
  if (subscription.group?.weekly_limit_usd) {
    rows.push(buildUsageRow(
      t('userSubscriptions.weekly'),
      subscription.weekly_usage_usd,
      subscription.group.weekly_limit_usd,
      subscription.weekly_window_start
        ? t('userSubscriptions.resetIn', { time: formatResetTime(subscription.weekly_window_start, 168) })
        : undefined,
    ))
  }
  if (subscription.group?.monthly_limit_usd) {
    rows.push(buildUsageRow(
      t('userSubscriptions.monthly'),
      subscription.monthly_usage_usd,
      subscription.group.monthly_limit_usd,
      subscription.monthly_window_start
        ? t('userSubscriptions.resetIn', { time: formatResetTime(subscription.monthly_window_start, 720) })
        : undefined,
    ))
  }
  return rows
}

function formatDurationParts(parts: RemainingDurationParts): string {
  if (parts.days > 0) {
    return `${parts.days}d ${parts.hours}h`
  }

  if (parts.hours > 0) {
    return `${parts.hours}h ${parts.minutes}m`
  }

  return `${parts.minutes}m`
}

function formatDailyUsageWindow(subscription: UserSubscription): string {
  if (isOneTimeDailyQuota(subscription) && subscription.expires_at) {
    const parts = getRemainingDurationParts(subscription.expires_at)
    if (!parts) return t('userSubscriptions.windowNotActive')
    return t('userSubscriptions.quotaEndsIn', { time: formatDurationParts(parts) })
  }

  return t('userSubscriptions.resetIn', {
    time: formatResetTime(subscription.daily_window_start, 24)
  })
}

function formatResetTime(windowStart: string | null, windowHours: number): string {
  if (!windowStart) return t('userSubscriptions.windowNotActive')

  const start = new Date(windowStart)
  const end = new Date(start.getTime() + windowHours * 60 * 60 * 1000)
  const parts = getRemainingDurationParts(end)

  return parts ? formatDurationParts(parts) : t('userSubscriptions.windowNotActive')
}

onMounted(() => {
  loadSubscriptions()
})
</script>
