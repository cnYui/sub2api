<template>
  <div
    :data-testid="product.testId || 'purchase-product-card'"
    :class="[
      'group relative flex min-h-[380px] flex-col overflow-hidden rounded-2xl border border-white/15 border-t-white/40 bg-black shadow-[0_20px_40px_rgba(0,0,0,0.35)] transition-[transform,box-shadow,border-color] duration-500 ease-out hover:-translate-y-2 hover:border-white/30 hover:border-t-white/70 hover:shadow-[0_24px_48px_rgba(0,0,0,0.7)]',
      product.active ? 'border-white/25' : '',
    ]"
  >
    <div class="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(255,255,255,0.035)_0%,rgba(255,255,255,0)_100%)] opacity-80 transition-opacity duration-500 group-hover:opacity-100" />

    <div class="relative z-10 flex h-full flex-1 flex-col justify-between p-6 sm:p-8 xl:p-10">
      <div class="w-full">
        <span class="mb-4 block border-b border-white/10 pb-2 text-[12px] font-medium uppercase leading-4 tracking-normal text-[#999999]">PLAN</span>
        <h3 class="mb-2 text-[32px] font-normal leading-[40px] tracking-normal text-white">{{ product.title }}</h3>

        <div class="mt-8">
          <div class="mb-6 flex items-end justify-between gap-4">
            <span class="text-base font-normal leading-6 text-[#999999]">Price</span>
            <span class="text-[40px] font-semibold leading-none tracking-normal text-white">{{ product.priceText }}</span>
          </div>
          <ul class="space-y-3 border-t border-white/10 pt-4 text-sm leading-6 text-[#999999]">
            <li v-for="item in product.detailRows" :key="item.label" class="flex justify-between gap-4">
              <span>{{ item.label }}</span>
              <span class="text-right font-medium text-white">{{ item.value }}</span>
            </li>
          </ul>
        </div>
      </div>

      <button
        type="button"
        class="mt-8 w-full rounded-full border border-white bg-white px-6 py-4 text-[12px] font-bold leading-4 tracking-normal text-black transition-all duration-300 hover:scale-[0.98] hover:bg-transparent hover:text-white active:scale-[0.96]"
        @click="emit('select', product)"
      >
        {{ product.buttonText }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PurchaseProductCardModel } from './purchaseProductCard'

defineProps<{ product: PurchaseProductCardModel }>()
const emit = defineEmits<{ select: [product: PurchaseProductCardModel] }>()
</script>

