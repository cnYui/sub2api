<template>
  <div
    data-testid="traffic-pack-card"
    :class="[
      'group relative flex flex-col overflow-hidden rounded-lg border transition-colors',
      borderClass,
      'bg-white hover:border-gray-400 dark:bg-dark-800 dark:hover:border-dark-500',
    ]"
  >
    <div data-testid="traffic-pack-accent" :class="['h-1.5', accentClass]" />

    <div class="flex flex-1 flex-col p-4">
      <div class="mb-3 flex items-start justify-between gap-2">
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <h3 class="truncate text-base font-bold text-gray-900 dark:text-white">{{ pack.name }}</h3>
            <span :class="['shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium', badgeLightClass]">
              {{ platformName }}
            </span>
          </div>
          <p class="mt-0.5 text-xs leading-relaxed text-gray-500 dark:text-dark-400 line-clamp-2">
            {{ summaryText }}
          </p>
        </div>
        <div class="shrink-0 text-right">
          <span :class="['text-2xl font-extrabold tracking-tight', textClass]">{{ priceText }}</span>
        </div>
      </div>

      <div class="flex-1" />

      <button
        type="button"
        :class="['w-full rounded-xl py-2.5 text-sm font-semibold transition-all active:scale-[0.98]', btnClass]"
        @click="emit('select', pack)"
      >
        购买
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { TrafficPack } from '@/types/payment'
import {
  platformAccentBarClass,
  platformBadgeLightClass,
  platformBorderClass,
  platformButtonClass,
  platformLabel,
  platformTextClass,
} from '@/utils/platformColors'

const props = defineProps<{ pack: TrafficPack }>()
const emit = defineEmits<{ select: [pack: TrafficPack] }>()

const platform = computed(() => props.pack.platform || 'openai')
const accentClass = computed(() => platformAccentBarClass(platform.value))
const borderClass = computed(() => platformBorderClass(platform.value))
const badgeLightClass = computed(() => platformBadgeLightClass(platform.value))
const textClass = computed(() => platformTextClass(platform.value))
const btnClass = computed(() => platformButtonClass(platform.value))
const platformName = computed(() => platformLabel(platform.value))

const formatCompactNumber = (value: number) => {
  if (!Number.isFinite(value)) return '0'
  return Number(value.toFixed(2)).toString()
}

const priceText = computed(() => `¥${formatCompactNumber(props.pack.price)}元`)
const summaryText = computed(() =>
  `一次性流量包-有效期 ${props.pack.validity_days}天，额度 ${formatCompactNumber(props.pack.credit_usd)}刀，用于写代码和生图`
)
</script>
