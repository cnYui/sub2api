<template>
  <div
    :data-testid="product.testId || 'purchase-product-card'"
    :class="[
      'group relative flex min-h-[380px] flex-col overflow-hidden rounded-2xl border border-gray-200/80 bg-white/70 text-gray-900 shadow-card backdrop-blur-xl transition-all duration-300 ease-out hover:-translate-y-1 hover:border-gray-300 hover:shadow-card-hover dark:border-dark-700/70 dark:bg-dark-800/60 dark:text-white',
      product.active ? 'border-gray-300 dark:border-white/25' : '',
    ]"
  >
    <div class="relative z-10 flex h-full flex-1 flex-col justify-between p-5 sm:p-6 lg:p-7">
      <div class="w-full">
        <span class="mb-4 block border-b border-gray-200/80 pb-2 text-[11px] font-semibold uppercase leading-4 tracking-wider text-gray-400 dark:border-dark-700/60 dark:text-dark-400">{{ product.eyebrowText }}</span>
        <h3 class="mb-2 break-words text-2xl font-semibold leading-8 tracking-tight text-gray-950 dark:text-white sm:text-[28px] sm:leading-9">{{ product.title }}</h3>
        <div class="mt-6 rounded-xl border border-gray-100/80 bg-gray-50/80 p-4 backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-900/40">
          <div class="flex items-end justify-between gap-4">
            <span class="text-sm font-medium leading-6 text-gray-500 dark:text-dark-400">{{ product.priceLabel }}</span>
            <span class="whitespace-nowrap text-3xl font-bold leading-none tracking-tight text-gray-950 dark:text-white sm:text-4xl">{{ product.priceText }}</span>
          </div>
          <ul class="mt-4 space-y-2 border-t border-gray-200/80 pt-3 text-sm leading-6 text-gray-500 dark:border-dark-700/60 dark:text-dark-400">
            <li v-for="item in product.detailRows" :key="item.label" class="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-3">
              <span class="min-w-0 break-words">{{ item.label }}</span>
              <span class="min-w-0 max-w-[9rem] break-words text-right font-semibold text-gray-950 dark:text-white">{{ item.value }}</span>
            </li>
          </ul>
        </div>
      </div>
      <button
        type="button"
        class="btn btn-primary mt-6 w-full whitespace-nowrap py-3 text-sm font-semibold"
        @click="emit('select', product)"
      >
        <Icon name="arrowRight" size="sm" />
        {{ product.buttonText }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PurchaseProductCardModel } from './purchaseProductCard'
import Icon from '@/components/icons/Icon.vue'

defineProps<{ product: PurchaseProductCardModel }>()
const emit = defineEmits<{ select: [product: PurchaseProductCardModel] }>()
</script>
