<template>
  <div class="card">
    <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('dashboard.quickActions') }}</h2>
    </div>
    <div class="space-y-2 p-3">
      <button @click="router.push('/keys')" class="group flex w-full items-center gap-3 rounded-md border border-gray-200 bg-white p-3 text-left transition-colors hover:border-gray-300 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:hover:border-dark-600 dark:hover:bg-dark-800">
        <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-md bg-gray-100 dark:bg-dark-800">
          <Icon name="key" size="md" class="text-gray-700 dark:text-gray-200" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('dashboard.createApiKey') }}</p>
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('dashboard.generateNewKey') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-gray-400 transition-colors group-hover:text-gray-700 dark:text-dark-500 dark:group-hover:text-gray-200"
        />
      </button>

      <button @click="router.push('/usage')" class="group flex w-full items-center gap-3 rounded-md border border-gray-200 bg-white p-3 text-left transition-colors hover:border-gray-300 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:hover:border-dark-600 dark:hover:bg-dark-800">
        <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-md bg-gray-100 dark:bg-dark-800">
          <Icon name="chart" size="md" class="text-gray-700 dark:text-gray-200" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('dashboard.viewUsage') }}</p>
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('dashboard.checkDetailedLogs') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-gray-400 transition-colors group-hover:text-gray-700 dark:text-dark-500 dark:group-hover:text-gray-200"
        />
      </button>

      <button @click="router.push('/redeem')" class="group flex w-full items-center gap-3 rounded-md border border-gray-200 bg-white p-3 text-left transition-colors hover:border-gray-300 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:hover:border-dark-600 dark:hover:bg-dark-800">
        <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-md bg-gray-100 dark:bg-dark-800">
          <Icon name="gift" size="md" class="text-gray-700 dark:text-gray-200" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('dashboard.redeemCode') }}</p>
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('dashboard.addBalanceWithCode') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-gray-400 transition-colors group-hover:text-gray-700 dark:text-dark-500 dark:group-hover:text-gray-200"
        />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
const router = useRouter()
const { t } = useI18n()
</script>
