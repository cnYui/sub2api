<template>
  <div>
    <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
      {{ t('payment.paymentMethod') }}
    </label>
    <div
      data-testid="payment-method-grid"
      class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4"
    >
      <button
        v-for="method in sortedMethods"
        :key="method.type"
        type="button"
        :title="methodLabel(method)"
        :disabled="!method.available"
        :class="[
          'relative flex h-[60px] min-w-0 flex-col items-center justify-center rounded-xl border px-3 backdrop-blur-sm transition-all',
          !method.available
            ? 'cursor-not-allowed border-gray-200/80 bg-gray-50/70 opacity-50 dark:border-dark-700/70 dark:bg-dark-900/40'
            : selected === method.type
              ? methodSelectedClass(method.type)
              : 'border-gray-200/80 bg-white/60 text-gray-700 hover:border-gray-300 hover:bg-white/80 dark:border-dark-700/70 dark:bg-dark-900/30 dark:text-gray-200 dark:hover:border-dark-600 dark:hover:bg-dark-800/60',
        ]"
        @click="method.available && emit('select', method.type)"
      >
        <span class="flex w-full min-w-0 items-center justify-center gap-2">
          <img :src="methodIcon(method.type)" :alt="methodLabel(method)" class="h-7 w-7 shrink-0 object-contain" />
          <span class="flex min-w-0 flex-col items-start leading-none">
            <span data-testid="payment-method-label" class="block w-full truncate text-base font-semibold">
              {{ methodLabel(method) }}
            </span>
            <span
              v-if="method.fee_rate > 0"
              class="text-[10px] tracking-wide text-gray-500 dark:text-dark-400"
            >
              {{ t('payment.fee') }} {{ method.fee_rate }}%
            </span>
          </span>
        </span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { METHOD_ORDER, isBuiltInAlipayMethod, isBuiltInWxpayMethod } from './providerConfig'
import alipayIcon from '@/assets/icons/alipay.svg'
import wxpayIcon from '@/assets/icons/wxpay.svg'
import stripeIcon from '@/assets/icons/stripe.svg'
import airwallexIcon from '@/assets/icons/airwallex.svg'
import paymentIcon from '@/assets/icons/payment.svg'

export interface PaymentMethodOption {
  type: string
  display_name?: string
  fee_rate: number
  available: boolean
}

const props = defineProps<{
  methods: PaymentMethodOption[]
  selected: string
}>()

const emit = defineEmits<{
  select: [type: string]
}>()

const { t } = useI18n()

const METHOD_ICONS: Record<string, string> = {
  alipay: alipayIcon,
  wxpay: wxpayIcon,
  stripe: stripeIcon,
  airwallex: airwallexIcon,
  credit_card: paymentIcon,
}

const sortedMethods = computed(() => {
  const order: readonly string[] = METHOD_ORDER
  return [...props.methods].sort((a, b) => {
    const ai = order.indexOf(a.type)
    const bi = order.indexOf(b.type)
    return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
  })
})

function methodIcon(type: string): string {
  if (isBuiltInAlipayMethod(type)) return METHOD_ICONS.alipay
  if (isBuiltInWxpayMethod(type)) return METHOD_ICONS.wxpay
  if (type === 'airwallex') return METHOD_ICONS.airwallex
  return METHOD_ICONS[type] || paymentIcon
}

function methodLabel(method: PaymentMethodOption): string {
  return method.display_name || t(`payment.methods.${method.type}`, method.type)
}

function methodSelectedClass(type: string): string {
  if (isBuiltInAlipayMethod(type)) return 'border-[#02A9F1]/70 bg-blue-50/70 text-gray-900 shadow-card dark:bg-blue-950/40 dark:text-gray-100'
  if (isBuiltInWxpayMethod(type)) return 'border-[#09BB07]/70 bg-green-50/70 text-gray-900 shadow-card dark:bg-green-950/40 dark:text-gray-100'
  if (type === 'stripe') return 'border-[#676BE5]/70 bg-indigo-50/70 text-gray-900 shadow-card dark:bg-indigo-950/40 dark:text-gray-100'
  if (type === 'airwallex') return 'border-[#FF6B3D]/70 bg-orange-50/70 text-gray-900 shadow-card dark:border-[#FF8E3C]/70 dark:bg-orange-950/40 dark:text-gray-100'
  return 'border-primary-500/70 bg-primary-50/70 text-gray-900 shadow-card dark:bg-primary-950/40 dark:text-gray-100'
}
</script>
