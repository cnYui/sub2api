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
            <div v-else :class="catalogGridClass">
              <PurchaseProductCard
                v-for="item in products"
                :key="item.id"
                :product="item.product"
                @select="selectCatalogProduct(item)"
              />
            </div>
            <section v-if="trafficProducts.length" class="space-y-4">
              <div :class="catalogGridClass">
                <PurchaseProductCard
                  v-for="item in trafficProducts"
                  :key="item.id"
                  :product="item.product"
                  @select="selectCatalogProduct(item)"
                />
              </div>
            </section>
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

            <template v-if="selectedTrafficPack">
              <div class="card p-5">
                <p class="text-xs font-medium uppercase tracking-wider text-gray-400">GPT 流量卡</p>
                <h3 class="mt-2 text-lg font-bold text-gray-900 dark:text-white">{{ selectedTrafficPack.name }}</h3>
                <div class="mt-3 flex items-baseline gap-2">
                  <span class="text-3xl font-bold text-gray-900 dark:text-white">{{ formatCNY(selectedTrafficPack.price) }}</span>
                  <span class="text-sm text-gray-500 dark:text-gray-400">/ ${{ formatUSDValue(selectedTrafficPack.credit_usd) }} 额度</span>
                </div>
                <div class="mt-4 grid grid-cols-2 gap-3">
                  <div><span class="text-xs text-gray-400">可用平台</span><div class="text-lg font-semibold text-gray-800 dark:text-gray-200">OpenAI</div></div>
                  <div><span class="text-xs text-gray-400">有效期</span><div class="text-lg font-semibold text-gray-800 dark:text-gray-200">{{ selectedTrafficPack.validity_days }} 天</div></div>
                </div>
              </div>
            </template>
            <template v-else-if="selectedBalancePackage">
              <div class="card p-5">
                <h3 class="text-lg font-bold text-gray-900 dark:text-white">{{ selectedBalancePackage.name }}</h3>
                <div class="flex items-baseline gap-2">
                  <span class="text-3xl font-bold text-gray-900 dark:text-white">{{ formatCNY(selectedBalancePackage.price_cny) }}</span>
                  <span class="text-sm text-gray-500 dark:text-gray-400">/ {{ selectedBalancePackage.validity_days }} 天</span>
                </div>
                <div class="mt-4 grid grid-cols-2 gap-3">
                  <div v-for="item in balancePackageDetails(selectedBalancePackage)" :key="item.label">
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
                  <span class="text-gray-900 dark:text-white">{{ formatCNY(currentAmount) }}</span>
                </div>
                <div v-if="feeAmount > 0" class="flex justify-between">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('payment.fee') }} ({{ feeRate }}%)</span>
                  <span class="text-gray-900 dark:text-white">{{ formatCNY(feeAmount) }}</span>
                </div>
                <div class="flex justify-between border-t border-gray-200 pt-2 dark:border-dark-600">
                  <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.actualPay') }}</span>
                  <span class="text-lg font-bold text-gray-900 dark:text-gray-100">{{ formatCNY(totalAmount) }}</span>
                </div>
              </div>
            </div>

            <button :class="['btn w-full py-3 text-base font-medium', paymentButtonClass]" :disabled="!canSubmit || submitting" @click="submitOrder">
              <span v-if="submitting" class="flex items-center justify-center gap-2">
                <span class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                {{ t('common.processing') }}
              </span>
              <span v-else>{{ t('payment.createOrder') }} {{ formatCNY(totalAmount) }}</span>
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
import { paymentAPI } from '@/api/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PaymentMethodSelector, { type PaymentMethodOption } from '@/components/payment/PaymentMethodSelector.vue'
import PaymentStatusPanel from '@/components/payment/PaymentStatusPanel.vue'
import PurchaseProductCard from '@/components/payment/PurchaseProductCard.vue'
import { type PurchaseProductCardModel } from '@/components/payment/purchaseProductCard'
import { METHOD_ORDER, getPaymentPopupFeatures, isBuiltInAlipayMethod, isBuiltInWxpayMethod } from '@/components/payment/providerConfig'
import { currencySymbol, formatPaymentAmount } from '@/components/payment/currency'
import { buildCreateOrderPayload, clearPaymentRecoverySnapshot, decidePaymentLaunch, getVisibleMethods, PAYMENT_RECOVERY_STORAGE_KEY, type PaymentRecoverySnapshot, writePaymentRecoverySnapshot } from '@/components/payment/paymentFlow'
import type { BalancePackagePlan, CheckoutInfoResponse, CreateOrderResult, OrderType, TrafficPack } from '@/types/payment'

interface ProductDetail { label: string; value: string }
interface CatalogProduct {
  id: string
  product: PurchaseProductCardModel
  balancePackage?: BalancePackagePlan
  trafficPack?: TrafficPack
}

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()
const paymentStore = usePaymentStore()

const loading = ref(true)
const submitting = ref(false)
const errorMessage = ref('')
const viewMode = ref<'catalog' | 'balance_package' | 'traffic_pack' | 'paying'>('catalog')
const selectedBalancePackage = ref<BalancePackagePlan | null>(null)
const selectedTrafficPack = ref<TrafficPack | null>(null)
const selectedMethod = ref('')
const paymentState = ref<PaymentRecoverySnapshot>(emptyPaymentState())
const checkout = ref<CheckoutInfoResponse>({
  methods: {}, global_min: 0, global_max: 0, plans: [], balance_packages: [], traffic_packs: [], traffic_credit_summary: null, balance_disabled: false,
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
const feeRate = computed(() => checkout.value.recharge_fee_rate || 0)
const trafficPacks = computed(() => checkout.value.traffic_packs ?? [])
const trafficProducts = computed<CatalogProduct[]>(() => trafficPacks.value.map((trafficPack) => ({
  id: `traffic-pack-${trafficPack.id}`,
  trafficPack,
  product: {
    testId: `traffic-pack-card-${trafficPack.id}`,
    eyebrowText: 'GPT 流量卡',
    title: trafficPack.name,
    priceLabel: '价格',
    priceText: formatCNY(trafficPack.price),
    buttonText: '立即购买',
    detailRows: [
      { label: '可用额度', value: `$${formatUSDValue(trafficPack.credit_usd)}` },
      { label: '有效期', value: `${trafficPack.validity_days} 天` },
      { label: '扣费方式', value: '余额不足时使用' },
    ],
  },
})))
const currentAmount = computed(() => selectedTrafficPack.value?.price ?? selectedBalancePackage.value?.price_cny ?? 0)
const feeAmount = computed(() => currentAmount.value > 0 && feeRate.value > 0 ? Math.ceil(currentAmount.value * feeRate.value) / 100 : 0)
const totalAmount = computed(() => Math.round((currentAmount.value + feeAmount.value) * 100) / 100)
const methodOptions = computed<PaymentMethodOption[]>(() => enabledMethods.value.map((type) => {
  const method = visibleMethods.value[type]
  return { type, display_name: method?.display_name, fee_rate: method?.fee_rate || 0, available: method?.available !== false }
}))
const canSubmit = computed(() => currentAmount.value > 0 && methodOptions.value.some(method => method.type === selectedMethod.value && method.available))
const paymentButtonClass = computed(() => {
  if (isBuiltInAlipayMethod(selectedMethod.value)) return 'btn-alipay'
  if (isBuiltInWxpayMethod(selectedMethod.value)) return 'btn-wxpay'
  if (selectedMethod.value === 'stripe') return 'btn-stripe'
  if (selectedMethod.value === 'airwallex') return 'btn-airwallex'
  return 'btn-primary'
})
const products = computed<CatalogProduct[]>(() => (checkout.value.balance_packages ?? []).map((balancePackage) => ({
  id: `balance-package-${balancePackage.id}`,
  balancePackage,
  product: {
    eyebrowText: 'API 余额套餐',
    title: `${formatPlainCNY(balancePackage.price_cny)} 元余额套餐`,
    priceLabel: '价格',
    priceText: formatCNY(balancePackage.price_cny),
    buttonText: '立即购买',
    detailRows: balancePackageDetails(balancePackage),
  },
})))
const catalogGridClass = computed(() => {
  const count = Math.max(products.value.length, trafficProducts.value.length)
  if (count <= 2) return 'grid auto-rows-[minmax(380px,auto)] grid-cols-1 gap-6 sm:grid-cols-2 sm:gap-8'
  if (count >= 4) return 'grid auto-rows-[minmax(380px,auto)] grid-cols-1 gap-6 sm:grid-cols-2 sm:gap-8 lg:grid-cols-4 lg:gap-12'
  return 'grid auto-rows-[minmax(380px,auto)] grid-cols-1 gap-6 sm:grid-cols-2 sm:gap-8 lg:grid-cols-3 lg:gap-12'
})

function formatCNY(value: number): string {
  return formatPaymentAmount(value, 'CNY', 'zh-CN')
}
function formatPlainCNY(value: number): string { return Number.isInteger(value) ? String(value) : value.toFixed(2) }
function formatUSD(value: number): string { return `${currencySymbol('USD')}${Number.isInteger(value) ? value : value.toFixed(2)}` }
function formatUSDValue(value: number): string { return Number.isInteger(value) ? String(value) : value.toFixed(2) }
function balancePackageDetails(balancePackage: BalancePackagePlan): ProductDetail[] {
  return [
    { label: '每周到账', value: formatUSD(balancePackage.weekly_credit_usd) },
    { label: '有效期', value: `${balancePackage.validity_days} 天` },
    { label: '到账次数', value: `${balancePackage.refresh_count} 次` },
    { label: '总到账', value: formatUSD(balancePackage.weekly_credit_usd * balancePackage.refresh_count) },
  ]
}
function selectCatalogProduct(item: CatalogProduct): void {
  errorMessage.value = ''
  if (item.trafficPack) {
    selectedTrafficPack.value = item.trafficPack
    selectedBalancePackage.value = null
    viewMode.value = 'traffic_pack'
    return
  }
  if (item.balancePackage) {
    selectedBalancePackage.value = item.balancePackage
    selectedTrafficPack.value = null
    viewMode.value = 'balance_package'
  }
}
function removeRecoverySnapshot(): void {
  if (typeof window !== 'undefined') clearPaymentRecoverySnapshot(window.localStorage, PAYMENT_RECOVERY_STORAGE_KEY)
}
function backToCatalog(): void {
  viewMode.value = 'catalog'
  selectedBalancePackage.value = null
  selectedTrafficPack.value = null
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
  const orderType: OrderType = selectedTrafficPack.value ? 'traffic_pack' : 'balance_subscription'
  submitting.value = true
  errorMessage.value = ''
  try {
    const result = await paymentStore.createOrder(buildCreateOrderPayload({
      amount: currentAmount.value, paymentType: selectedMethod.value, orderType, balancePackagePlanId: selectedBalancePackage.value?.id, trafficPackId: selectedTrafficPack.value?.id,
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
  window.dispatchEvent(new Event('traffic-credit-updated'))
  paymentAPI.getCheckoutInfo().then((response) => { checkout.value = response.data }).catch(() => {})
}
onMounted(async () => {
  try {
    checkout.value = (await paymentAPI.getCheckoutInfo()).data
    selectedMethod.value = [...enabledMethods.value].sort((left, right) => {
      const leftIndex = METHOD_ORDER.indexOf(left as typeof METHOD_ORDER[number])
      const rightIndex = METHOD_ORDER.indexOf(right as typeof METHOD_ORDER[number])
      return (leftIndex < 0 ? 999 : leftIndex) - (rightIndex < 0 ? 999 : rightIndex)
    })[0] || ''
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
