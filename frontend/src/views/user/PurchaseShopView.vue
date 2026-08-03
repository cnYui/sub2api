<template>
  <AppLayout>
    <div :class="['relative mx-auto space-y-6', viewMode === 'catalog' ? 'max-w-7xl' : 'max-w-4xl']">
      <div v-if="loading" class="flex items-center justify-center py-20">
        <div class="h-8 w-8 animate-spin rounded-full border-4 border-gray-900 border-t-transparent dark:border-gray-100 dark:border-t-transparent" />
      </div>
      <Transition v-else name="payment-phase" mode="out-in">
        <section :key="viewMode" class="space-y-6">
          <PaymentStatusPanel
            v-if="viewMode === 'paying'"
            :order-id="paymentState.orderId"
            :amount="paymentState.amount"
            :pay-amount="paymentState.payAmount"
            :qr-code="paymentState.qrCode"
            :qr-image-url="paymentState.qrImageUrl"
            :expires-at="paymentState.expiresAt"
            :payment-type="paymentState.paymentType"
            :pay-url="paymentState.payUrl"
            :order-type="paymentState.orderType"
            :currency="paymentState.currency"
            :out-trade-no="paymentState.outTradeNo"
            :mobile-alipay-deep-link="paymentState.alipayMobilePrecreateDeepLink"
            @done="backToCatalog"
            @success="onPaymentSuccess"
            @settled="removeRecoverySnapshot"
          />

          <template v-else-if="viewMode === 'catalog'">
            <div v-if="products.length === 0" class="card py-16 text-center">
              <p class="text-gray-500 dark:text-gray-400">{{ t('payment.noPlans') }}</p>
            </div>
            <div v-else :class="productGridClass">
              <PurchaseProductCard
                v-for="item in products"
                :key="item.id"
                :product="item.product"
                @select="selectProduct(item)"
              />
            </div>
          </template>

          <template v-else>
            <button
              type="button"
              class="inline-flex items-center gap-2 text-sm font-medium text-gray-500 transition-colors hover:text-gray-900 dark:text-gray-400 dark:hover:text-white"
              @click="backToCatalog"
            >
              <Icon name="arrowLeft" size="sm" />
              {{ t('common.back') }}
            </button>

            <template v-if="viewMode === 'recharge'">
              <div class="card p-5">
                <h3 class="text-lg font-bold text-gray-900 dark:text-white">余额充值</h3>
                <label class="mt-4 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('payment.amountLabel') }}</label>
                <input v-model="rechargeAmountText" class="input mt-2" inputmode="decimal" autocomplete="off" @input="syncRechargeAmount" />
                <p v-if="rechargeError" class="mt-2 text-sm text-red-600 dark:text-red-400">{{ rechargeError }}</p>
              </div>
            </template>

            <template v-else-if="selectedPlan">
              <div class="card p-5">
                <div class="mb-3 flex flex-wrap items-center gap-2">
                  <span :class="['rounded-md border px-2 py-0.5 text-xs font-medium', planBadgeClass]">{{ platformLabel(selectedPlan.group_platform || '') }}</span>
                  <h3 class="text-lg font-bold text-gray-900 dark:text-white">{{ selectedPlan.name }}</h3>
                </div>
                <div class="flex items-baseline gap-2">
                  <span class="text-3xl font-bold text-gray-900 dark:text-white">{{ formatAmount(selectedPlan.price) }}</span>
                  <span class="text-sm text-gray-500 dark:text-gray-400">/ {{ selectedPlan.validity_days }} {{ t('payment.days') }}</span>
                </div>
                <p v-if="selectedPlan.description" class="mt-2 text-sm leading-relaxed text-gray-500 dark:text-gray-400">{{ selectedPlan.description }}</p>
                <div class="mt-4 grid grid-cols-2 gap-3">
                  <div v-for="item in subscriptionDetails(selectedPlan)" :key="item.label">
                    <span class="text-xs text-gray-400 dark:text-gray-500">{{ item.label }}</span>
                    <div class="text-lg font-semibold text-gray-800 dark:text-gray-200">{{ item.value }}</div>
                  </div>
                </div>
              </div>
            </template>

            <div v-if="methodOptions.length" class="card p-6">
              <PaymentMethodSelector :methods="methodOptions" :selected="selectedMethod" @select="selectedMethod = $event" />
            </div>
            <div v-else class="card p-4 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('payment.notAvailable') }}</div>

            <div class="card p-6">
              <div class="space-y-2 text-sm">
                <div class="flex justify-between">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('payment.paymentAmount') }}</span>
                  <span class="text-gray-900 dark:text-white">{{ formatAmount(currentAmount) }}</span>
                </div>
                <div v-if="feeAmount > 0" class="flex justify-between">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('payment.fee') }} ({{ feeRate }}%)</span>
                  <span class="text-gray-900 dark:text-white">{{ formatAmount(feeAmount) }}</span>
                </div>
                <div class="flex justify-between border-t border-gray-200 pt-2 dark:border-dark-600">
                  <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.actualPay') }}</span>
                  <span class="text-lg font-bold text-gray-900 dark:text-gray-100">{{ formatAmount(totalAmount) }}</span>
                </div>
              </div>
            </div>

            <button :class="['btn w-full py-3 text-base font-medium', paymentButtonClass]" :disabled="!canSubmit || submitting" @click="submitOrder">
              <span v-if="submitting" class="flex items-center justify-center gap-2">
                <span class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                {{ t('common.processing') }}
              </span>
              <span v-else>{{ t('payment.createOrder') }} {{ formatAmount(totalAmount) }}</span>
            </button>
          </template>

          <div v-if="errorMessage" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300">
            {{ errorMessage }}
          </div>
        </section>
      </Transition>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores'
import { usePaymentStore } from '@/stores/payment'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { paymentAPI } from '@/api/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PaymentMethodSelector, { type PaymentMethodOption } from '@/components/payment/PaymentMethodSelector.vue'
import PaymentStatusPanel from '@/components/payment/PaymentStatusPanel.vue'
import PurchaseProductCard from '@/components/payment/PurchaseProductCard.vue'
import { type PurchaseProductCardModel } from '@/components/payment/purchaseProductCard'
import { METHOD_ORDER, getPaymentPopupFeatures, isBuiltInAlipayMethod, isBuiltInWxpayMethod } from '@/components/payment/providerConfig'
import { formatPaymentAmount, normalizePaymentCurrency } from '@/components/payment/currency'
import { buildCreateOrderPayload, clearPaymentRecoverySnapshot, decidePaymentLaunch, getVisibleMethods, PAYMENT_RECOVERY_STORAGE_KEY, type PaymentRecoverySnapshot, writePaymentRecoverySnapshot } from '@/components/payment/paymentFlow'
import { platformBadgeClass, platformLabel } from '@/utils/platformColors'
import type { CheckoutInfoResponse, CreateOrderResult, OrderType, SubscriptionPlan } from '@/types/payment'

interface ProductDetail { label: string; value: string }
interface CatalogProduct { id: string; kind: 'balance' | 'subscription'; product: PurchaseProductCardModel; plan?: SubscriptionPlan }

const { t, locale } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()
const paymentStore = usePaymentStore()
const subscriptionStore = useSubscriptionStore()

const loading = ref(true)
const submitting = ref(false)
const errorMessage = ref('')
const viewMode = ref<'catalog' | 'recharge' | 'subscription' | 'paying'>('catalog')
const selectedPlan = ref<SubscriptionPlan | null>(null)
const selectedMethod = ref('')
const rechargeAmountText = ref('10')
const rechargeAmount = ref(10)
const paymentState = ref<PaymentRecoverySnapshot>(emptyPaymentState())
const checkout = ref<CheckoutInfoResponse>({
  methods: {}, global_min: 0, global_max: 0, plans: [], balance_disabled: false,
  balance_recharge_multiplier: 1, subscription_usd_to_cny_rate: 0, recharge_fee_rate: 0,
  help_text: '', help_image_url: '', stripe_publishable_key: '',
})

function emptyPaymentState(): PaymentRecoverySnapshot {
  return {
    orderId: 0, amount: 0, qrCode: '', qrImageUrl: '', expiresAt: '', paymentType: '', payUrl: '',
    outTradeNo: '', clientSecret: '', intentId: '', currency: '', countryCode: '', paymentEnv: '',
    payAmount: 0, orderType: '', paymentMode: '', resumeToken: '', alipayMobilePrecreateDeepLink: false,
    createdAt: 0,
  }
}

const visibleMethods = computed(() => getVisibleMethods(checkout.value.methods))
const enabledMethods = computed(() => Object.keys(visibleMethods.value))
const selectedLimit = computed(() => visibleMethods.value[selectedMethod.value])
const selectedCurrency = computed(() => normalizePaymentCurrency(selectedLimit.value?.currency))
const feeRate = computed(() => checkout.value.recharge_fee_rate || 0)
const currentAmount = computed(() => viewMode.value === 'subscription' ? (selectedPlan.value?.price || 0) : rechargeAmount.value)
const feeAmount = computed(() => currentAmount.value > 0 && feeRate.value > 0 ? Math.ceil(currentAmount.value * feeRate.value) / 100 : 0)
const totalAmount = computed(() => Math.round((currentAmount.value + feeAmount.value) * 100) / 100)
const rechargeError = computed(() => {
  if (!Number.isFinite(rechargeAmount.value) || rechargeAmount.value <= 0) return t('payment.enterAmount')
  if (checkout.value.global_min > 0 && rechargeAmount.value < checkout.value.global_min) return t('payment.amountTooLow', { min: formatAmount(checkout.value.global_min) })
  if (checkout.value.global_max > 0 && rechargeAmount.value > checkout.value.global_max) return t('payment.amountTooHigh', { max: formatAmount(checkout.value.global_max) })
  return ''
})
const methodOptions = computed<PaymentMethodOption[]>(() => enabledMethods.value.map((type) => {
  const method = visibleMethods.value[type]
  return { type, display_name: method?.display_name, fee_rate: method?.fee_rate || 0, available: method?.available !== false }
}))
const canSubmit = computed(() => !(viewMode.value === 'recharge' && rechargeError.value) && currentAmount.value > 0 && methodOptions.value.some(method => method.type === selectedMethod.value && method.available))
const paymentButtonClass = computed(() => {
  if (isBuiltInAlipayMethod(selectedMethod.value)) return 'btn-alipay'
  if (isBuiltInWxpayMethod(selectedMethod.value)) return 'btn-wxpay'
  if (selectedMethod.value === 'stripe') return 'btn-stripe'
  if (selectedMethod.value === 'airwallex') return 'btn-airwallex'
  return 'btn-primary'
})
const planBadgeClass = computed(() => platformBadgeClass(selectedPlan.value?.group_platform || ''))

const products = computed<CatalogProduct[]>(() => [
  ...(checkout.value.balance_disabled ? [] : [{
    id: 'balance', kind: 'balance' as const,
    product: {
      eyebrowText: '余额充值', title: 'API 余额', priceLabel: '价格', priceText: `${formatAmount(Math.max(checkout.value.global_min || 1, 1))} 起`, buttonText: '立即充值',
      detailRows: [{ label: '用途', value: 'API 调用余额' }, { label: '到账', value: '实时到账' }, { label: '手续费', value: feeRate.value > 0 ? `${feeRate.value}%` : '无' }],
    },
  }]),
  ...checkout.value.plans.map((plan, index) => ({
    id: `subscription-${plan.id}`, kind: 'subscription' as const, plan,
    product: { eyebrowText: '订阅', title: plan.name || `订阅套餐 ${index + 1}`, priceLabel: '价格', priceText: formatAmount(plan.price), buttonText: '立即开通', detailRows: subscriptionDetails(plan) },
  })),
])
const productGridClass = computed(() => {
  const count = products.value.length
  if (count <= 2) return 'grid auto-rows-[minmax(380px,auto)] grid-cols-1 gap-6 sm:grid-cols-2 sm:gap-8'
  if (count >= 4) return 'grid auto-rows-[minmax(380px,auto)] grid-cols-1 gap-6 sm:grid-cols-2 sm:gap-8 lg:grid-cols-4 lg:gap-12'
  return 'grid auto-rows-[minmax(380px,auto)] grid-cols-1 gap-6 sm:grid-cols-2 sm:gap-8 lg:grid-cols-3 lg:gap-12'
})

function formatAmount(value: number): string {
  return formatPaymentAmount(value, selectedCurrency.value, typeof locale.value === 'string' ? locale.value : undefined)
}
function subscriptionDetails(plan: SubscriptionPlan): ProductDetail[] {
  const rows: ProductDetail[] = [{ label: '倍率', value: `x${plan.rate_multiplier ?? 1}` }, { label: '有效期', value: `${plan.validity_days} ${t('payment.days')}` }]
  if (plan.daily_limit_usd != null) rows.unshift({ label: t('payment.planCard.dailyLimit'), value: `$${plan.daily_limit_usd}` })
  if (plan.weekly_limit_usd != null) rows.unshift({ label: t('payment.planCard.weeklyLimit'), value: `$${plan.weekly_limit_usd}` })
  if (plan.monthly_limit_usd != null) rows.unshift({ label: t('payment.planCard.monthlyLimit'), value: `$${plan.monthly_limit_usd}` })
  rows.push({ label: '手续费', value: feeRate.value > 0 ? `${feeRate.value}%` : '无' })
  return rows
}
function syncRechargeAmount(): void { rechargeAmount.value = Number(rechargeAmountText.value) || 0 }
function selectProduct(item: CatalogProduct): void {
  errorMessage.value = ''
  if (item.kind === 'balance') {
    const minimum = checkout.value.global_min > 0 ? checkout.value.global_min : 10
    rechargeAmountText.value = String(minimum)
    rechargeAmount.value = minimum
    viewMode.value = 'recharge'
    return
  }
  selectedPlan.value = item.plan || null
  viewMode.value = 'subscription'
}
function removeRecoverySnapshot(): void {
  if (typeof window !== 'undefined') clearPaymentRecoverySnapshot(window.localStorage, PAYMENT_RECOVERY_STORAGE_KEY)
}
function backToCatalog(): void {
  viewMode.value = 'catalog'
  selectedPlan.value = null
  errorMessage.value = ''
  paymentState.value = emptyPaymentState()
  removeRecoverySnapshot()
}
function openWindow(url: string): void {
  const popup = window.open(url, 'paymentPopup', getPaymentPopupFeatures())
  if (!popup || popup.closed) window.location.href = url
}
async function submitOrder(): Promise<void> {
  if (!canSubmit.value || submitting.value) return
  const orderType: OrderType = viewMode.value === 'subscription' ? 'subscription' : 'balance'
  submitting.value = true
  errorMessage.value = ''
  try {
    const result = await paymentStore.createOrder(buildCreateOrderPayload({
      amount: currentAmount.value, paymentType: selectedMethod.value, orderType, planId: selectedPlan.value?.id,
      origin: window.location.origin, isMobile: /Mobi|Android/i.test(window.navigator.userAgent), isWechatBrowser: /MicroMessenger/i.test(window.navigator.userAgent),
      forceQRCode: checkout.value.alipay_force_qrcode === true, mobilePrecreateDeepLink: checkout.value.alipay_mobile_precreate_deep_link === true,
    })) as CreateOrderResult
    const stripeUrl = router.resolve({ path: '/payment/stripe', query: { order_id: String(result.order_id), client_secret: result.client_secret || '' } }).href
    const airwallexUrl = router.resolve({ path: '/payment/airwallex', query: { order_id: String(result.order_id) } }).href
    const decision = decidePaymentLaunch(result, {
      visibleMethod: selectedMethod.value, orderType, isMobile: /Mobi|Android/i.test(window.navigator.userAgent), isWechatBrowser: /MicroMessenger/i.test(window.navigator.userAgent),
      forceQRCode: checkout.value.alipay_force_qrcode === true, mobilePrecreateDeepLink: checkout.value.alipay_mobile_precreate_deep_link === true,
      stripePopupUrl: stripeUrl, stripeRouteUrl: stripeUrl, airwallexRouteUrl: airwallexUrl,
    })
    if (decision.kind === 'wechat_oauth' && decision.oauth?.authorize_url) {
      window.location.href = decision.oauth.authorize_url
      return
    }
    if (decision.kind === 'unhandled') {
      errorMessage.value = t('common.error')
      return
    }
    paymentState.value = decision.paymentState
    writePaymentRecoverySnapshot(window.localStorage, decision.recovery, PAYMENT_RECOVERY_STORAGE_KEY)
    viewMode.value = 'paying'
    if (decision.kind === 'stripe_route' || decision.kind === 'airwallex_route') window.location.href = decision.paymentState.payUrl
    if ((decision.kind === 'stripe_popup' || decision.kind === 'redirect_waiting') && decision.paymentState.payUrl) openWindow(decision.paymentState.payUrl)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('common.error')
  } finally {
    submitting.value = false
  }
}
function onPaymentSuccess(): void {
  authStore.refreshUser()
  subscriptionStore.fetchActiveSubscriptions(true).catch(() => {})
}
onMounted(async () => {
  try {
    checkout.value = (await paymentAPI.getCheckoutInfo()).data
    selectedMethod.value = [...enabledMethods.value].sort((left, right) => {
      const leftIndex = METHOD_ORDER.indexOf(left as typeof METHOD_ORDER[number])
      const rightIndex = METHOD_ORDER.indexOf(right as typeof METHOD_ORDER[number])
      return (leftIndex < 0 ? 999 : leftIndex) - (rightIndex < 0 ? 999 : rightIndex)
    })[0] || ''
    subscriptionStore.fetchActiveSubscriptions().catch(() => {})
  } catch {
    errorMessage.value = t('common.error')
    appStore.showError(errorMessage.value)
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.payment-phase-enter-active,
.payment-phase-leave-active { transition: transform 180ms var(--ease-out), opacity 180ms var(--ease-out); }
.payment-phase-enter-from,
.payment-phase-leave-to { opacity: 0; transform: translate3d(0, 4px, 0); }
@media (prefers-reduced-motion: reduce) {
  .payment-phase-enter-active,
  .payment-phase-leave-active { transition-property: opacity; }
  .payment-phase-enter-from,
  .payment-phase-leave-to { transform: none; }
}
</style>
