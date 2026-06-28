<template>
  <div
    class="overflow-hidden rounded-2xl border bg-white dark:bg-dark-800"
    :class="platformBorderClass(platform)"
  >
    <div class="flex items-center justify-between border-b border-gray-100 p-4 dark:border-dark-700">
      <div class="flex items-center gap-3">
        <div :class="['h-1.5 w-1.5 shrink-0 rounded-full', platformAccentBarClass(platform)]" />
        <div>
          <div class="flex items-center gap-2">
            <h3 class="font-semibold text-gray-900 dark:text-white">
              {{ title }}
            </h3>
            <span :class="['rounded-md border px-2 py-0.5 text-[11px] font-medium', platformBadgeClass(platform)]">
              {{ platformLabel(platform) }}
            </span>
          </div>
          <p v-if="description" class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
            {{ description }}
          </p>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <span :class="statusClass">
          {{ statusLabel }}
        </span>
      </div>
    </div>

    <div class="space-y-4 p-4">
      <div class="flex items-center justify-between text-sm">
        <span class="text-gray-500 dark:text-dark-400">{{ expirationLabel }}</span>
        <span :class="expirationValueClass">{{ expirationValue }}</span>
      </div>

      <div v-for="row in usageRows" :key="row.label" class="space-y-2">
        <div class="flex items-center justify-between">
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ row.label }}
          </span>
          <span class="text-sm text-gray-500 dark:text-dark-400">
            {{ row.value }}
          </span>
        </div>
        <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
          <div
            class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
            :class="row.progressClass"
            :data-testid="row.testId"
            :style="{ width: row.progressWidth }"
          ></div>
        </div>
        <p v-if="row.footer" class="text-xs text-gray-500 dark:text-dark-400">
          {{ row.footer }}
        </p>
      </div>

      <div
        v-if="usageRows.length === 0"
        class="flex items-center justify-center rounded-lg border border-gray-200 bg-gray-50 py-6 dark:border-dark-700 dark:bg-dark-900"
      >
        <div class="flex items-center gap-3">
          <span class="text-4xl text-gray-700 dark:text-gray-200">∞</span>
          <div>
            <p class="text-sm font-medium text-gray-800 dark:text-gray-100">
              {{ unlimitedTitle }}
            </p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ unlimitedDescription }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  platformAccentBarClass,
  platformBadgeClass,
  platformBorderClass,
  platformLabel,
} from '@/utils/platformColors'

export interface SubscriptionUsageRow {
  label: string
  value: string
  progressWidth: string
  progressClass: string
  footer?: string
  testId?: string
}

const props = defineProps<{
  title: string
  platform: string
  description?: string
  status: string
  statusLabel: string
  expirationLabel: string
  expirationValue: string
  expirationValueClass?: string
  usageRows: SubscriptionUsageRow[]
  unlimitedTitle: string
  unlimitedDescription: string
}>()

const statusClass = computed(() => {
  if (props.status === 'active') {
    return 'rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-200'
  }
  if (props.status === 'expired') {
    return 'rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-400'
  }
  return 'rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700 dark:bg-red-900/40 dark:text-red-300'
})
</script>
