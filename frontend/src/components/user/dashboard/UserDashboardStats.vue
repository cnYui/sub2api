<template>
  <!-- Row 1: Core Stats -->
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
    <!-- Balance -->
    <div v-if="!isSimple" class="card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-md border border-gray-200 bg-gray-100 p-2 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200">
          <svg class="h-5 w-5 text-gray-900 dark:text-gray-100" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" />
          </svg>
        </div>
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('dashboard.balance') }}</p>
          <p class="text-xl font-bold text-gray-900 dark:text-gray-100">${{ formatBalance(balance) }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.available') }}</p>
        </div>
      </div>
    </div>

    <!-- API Keys -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-md border border-gray-200 bg-gray-100 p-2 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200">
          <Icon name="key" size="md" class="text-gray-900 dark:text-gray-100" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('dashboard.apiKeys') }}</p>
          <p class="text-xl font-bold text-gray-900 dark:text-white">{{ stats?.total_api_keys || 0 }}</p>
          <p class="text-xs text-gray-900 dark:text-gray-100">{{ stats?.active_api_keys || 0 }} {{ t('common.active') }}</p>
        </div>
      </div>
    </div>

    <!-- Today Requests -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-md border border-gray-200 bg-gray-100 p-2 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200">
          <Icon name="chart" size="md" class="text-gray-900 dark:text-gray-100" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('dashboard.todayRequests') }}</p>
          <p class="text-xl font-bold text-gray-900 dark:text-white">{{ stats?.today_requests || 0 }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.total') }}: {{ formatNumber(stats?.total_requests || 0) }}</p>
        </div>
      </div>
    </div>

    <!-- Subscription Quota -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-md border border-gray-200 bg-gray-100 p-2 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200">
          <Icon name="dollar" size="md" class="text-gray-900 dark:text-gray-100" :stroke-width="2" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('dashboard.subscriptionQuota') }}</p>
          <p class="truncate text-base font-bold text-gray-900 dark:text-white">
            <span class="mr-1 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('dashboard.todayQuota') }}</span>
            <span>${{ formatCost(quotaTodayUsed) }} / ${{ formatCost(quotaTodayLimit) }}</span>
          </p>
          <p class="truncate text-xs text-gray-500 dark:text-gray-400">
            <span>{{ periodQuotaLabel }}: </span>
            <span class="text-gray-900 dark:text-gray-100">${{ formatCost(quotaPeriodUsed) }} / ${{ formatCost(quotaPeriodLimit) }}</span>
          </p>
        </div>
      </div>
    </div>
  </div>

  <!-- Row 2: Token Stats -->
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
    <!-- Today Tokens -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-md border border-gray-200 bg-gray-100 p-2 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200">
          <Icon name="cube" size="md" class="text-gray-900 dark:text-gray-100" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('dashboard.todayTokens') }}</p>
          <p class="text-xl font-bold text-gray-900 dark:text-white">{{ formatTokens(stats?.today_tokens || 0) }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('dashboard.input') }}: {{ formatTokens(stats?.today_input_tokens || 0) }} / {{ t('dashboard.output') }}: {{ formatTokens(stats?.today_output_tokens || 0) }}</p>
        </div>
      </div>
    </div>

    <!-- Total Tokens -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-md border border-gray-200 bg-gray-100 p-2 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200">
          <Icon name="database" size="md" class="text-gray-900 dark:text-gray-100" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('dashboard.totalTokens') }}</p>
          <p class="text-xl font-bold text-gray-900 dark:text-white">{{ formatTokens(stats?.total_tokens || 0) }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('dashboard.input') }}: {{ formatTokens(stats?.total_input_tokens || 0) }} / {{ t('dashboard.output') }}: {{ formatTokens(stats?.total_output_tokens || 0) }}</p>
        </div>
      </div>
    </div>

    <!-- Performance (RPM/TPM) -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-md border border-gray-200 bg-gray-100 p-2 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200">
          <Icon name="bolt" size="md" class="text-gray-900 dark:text-gray-100" :stroke-width="2" />
        </div>
        <div class="flex-1">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('dashboard.performance') }}</p>
          <div class="flex items-baseline gap-2">
            <p class="text-xl font-bold text-gray-900 dark:text-white">{{ formatTokens(stats?.rpm || 0) }}</p>
            <span class="text-xs text-gray-500 dark:text-gray-400">RPM</span>
          </div>
          <div class="flex items-baseline gap-2">
            <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatTokens(stats?.tpm || 0) }}</p>
            <span class="text-xs text-gray-500 dark:text-gray-400">TPM</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Avg Response Time -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-md border border-gray-200 bg-gray-100 p-2 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200">
          <Icon name="clock" size="md" class="text-gray-900 dark:text-gray-100" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('dashboard.avgResponse') }}</p>
          <p class="text-xl font-bold text-gray-900 dark:text-white">{{ formatDuration(stats?.average_duration_ms || 0) }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('dashboard.averageTime') }}</p>
        </div>
      </div>
    </div>
  </div>

  <!-- Row 3: Available models -->
  <div class="card p-4">
    <div class="mb-3 flex items-center justify-between">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('dashboard.availableModels') }}</h3>
      <span class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('dashboard.availableModelsCount', { count: AVAILABLE_MODELS.length }) }}
      </span>
    </div>
    <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5">
      <span
        v-for="model in AVAILABLE_MODELS"
        :key="model"
        class="truncate rounded-md border border-gray-200 bg-gray-50 px-3 py-2 font-mono text-xs text-gray-800 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100"
        :title="model"
      >
        {{ model }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { UserDashboardStats as UserStatsType } from '@/api/usage'

const props = defineProps<{
  stats: UserStatsType
  balance: number
  isSimple: boolean
}>()
const { t } = useI18n()

const AVAILABLE_MODELS = [
  'gpt-5.6-sol',
  'gpt-5.6-terra',
  'gpt-5.6-luna',
  'gpt-5.5',
  'gpt-5.4',
  'gpt-image-2',
]

const quota = computed(() => props.stats?.quota)
const quotaTodayUsed = computed(() => quota.value?.today_usage_usd ?? 0)
const quotaTodayLimit = computed(() => quota.value?.today_limit_usd ?? 0)
const quotaPeriodUsed = computed(() => quota.value?.period_usage_usd ?? 0)
const quotaPeriodLimit = computed(() => quota.value?.period_limit_usd ?? 0)
const periodQuotaLabel = computed(() => {
  if (quota.value?.period_mode === 'rolling_30d_legacy' || quota.value?.period_mode === 'none') {
    return t('dashboard.last30DaysQuota')
  }
  return t('dashboard.periodQuota')
})

const formatBalance = (b: number) =>
  new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(b)

const formatNumber = (n: number) => n.toLocaleString()
const formatCost = (c: number) => c.toFixed(4)
const formatTokens = (t: number) => {
  if (t >= 1_000_000) return `${(t / 1_000_000).toFixed(1)}M`
  if (t >= 1000) return `${(t / 1000).toFixed(1)}K`
  return t.toString()
}
const formatDuration = (ms: number) => ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${ms.toFixed(0)}ms`
</script>
