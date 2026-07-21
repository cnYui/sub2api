<template>
  <AppLayout>
    <div
      data-testid="payment-phase-track"
      :class="['relative mx-auto space-y-6', paymentPhase === 'select' && !currentPurchaseProduct ? 'max-w-7xl' : 'max-w-4xl']"
    >
      <div v-if="loading" class="flex items-center justify-center py-20">
        <div class="h-8 w-8 animate-spin rounded-full border-4 border-gray-900 border-t-transparent dark:border-gray-100 dark:border-t-transparent"></div>
      </div>
      <template v-else>
        <Transition name="payment-phase">
          <div :key="paymentPhase" data-testid="payment-phase-state" class="space-y-6">
            <!-- Payment in progress -->
            <template v-if="paymentPhase === 'paying'">
              <PaymentStatusPanel
                :order-id="paymentState.orderId"
                :qr-code="paymentState.qrCode"
                :qr-image-url="paymentState.qrImageUrl"
                :expires-at="paymentState.expiresAt"
                :payment-type="paymentState.paymentType"
                :pay-url="paymentState.payUrl"
                :order-type="paymentState.orderType"
                :currency="paymentState.currency || selectedCurrency"
                @done="onPaymentDone"
                @success="onPaymentSuccess"
                @settled="onPaymentSettled"
              />
            </template>
            <!-- Subscription and traffic pack purchases -->
            <template v-else>
              <div v-if="paymentPhase === 'recharge' || currentPurchaseProduct" class="mb-1">
                <button
                  type="button"
                  class="inline-flex items-center gap-2 text-sm font-medium text-gray-500 transition-colors hover:text-gray-900 dark:text-gray-400 dark:hover:text-white"
                  @click="backToSubscriptionList"
                >
                  <Icon name="arrowLeft" size="sm" />
                  {{ t('common.back') }}
                </button>
              </div>
              <!-- Traffic pack confirm (inline, reuses subscription payment flow) -->
              <template v-if="paymentPhase === 'recharge'">
            <div class="card p-5">
              <h3 class="text-lg font-bold text-gray-900 dark:text-white">{{ t('payment.recharge.title') }}</h3>
              <label class="mt-4 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('payment.recharge.amount') }}</label>
              <input
                v-model="rechargeAmount"
                data-testid="balance-recharge-amount"
                class="input mt-2"
                inputmode="numeric"
                autocomplete="off"
              />
              <p v-if="rechargeError" class="mt-2 text-sm text-red-600 dark:text-red-400">{{ rechargeError }}</p>
            </div>
            <div class="card p-6">
              <PaymentMethodSelector
                :methods="rechargeMethodOptions"
                selected="alipay"
                @select="selectedMethod = 'alipay'"
              />
            </div>
            <div class="card p-6">
              <div class="flex justify-between text-sm">
                <span class="text-gray-500 dark:text-gray-400">{{ t('payment.recharge.amount') }}</span>
                <span class="text-gray-900 dark:text-white">¥{{ validRechargeAmount.toFixed(2) }}</span>
              </div>
              <div class="mt-2 flex justify-between border-t border-gray-200 pt-2 text-sm dark:border-dark-600">
                <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.actualPay') }}</span>
                <span class="text-lg font-bold text-gray-900 dark:text-gray-100">¥{{ validRechargeAmount.toFixed(2) }}</span>
              </div>
            </div>
            <button data-testid="balance-recharge-submit" class="btn btn-alipay w-full py-3 text-base font-medium" :disabled="!!rechargeError || submitting" @click="confirmRecharge">
              <span v-if="submitting" class="flex items-center justify-center gap-2">
                <span class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
                {{ t('common.processing') }}
              </span>
              <span v-else>{{ t('payment.recharge.confirm', { amount: validRechargeAmount.toFixed(0) }) }}</span>
            </button>
            <button class="btn btn-secondary w-full" @click="backToSubscriptionList">{{ t('common.back') }}</button>
              </template>
              <template v-else-if="currentPurchaseProduct">
            <template v-if="selectedTrafficPack">
              <div class="card p-5">
                <div class="mb-3 flex flex-wrap items-center gap-2">
                  <span class="rounded-md border border-gray-200 bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200">GPT</span>
                  <h3 class="text-lg font-bold text-gray-900 dark:text-white">{{ selectedTrafficPack.name }}</h3>
                </div>
                <div class="flex items-baseline gap-2">
                  <span class="text-3xl font-bold text-gray-900 dark:text-white">{{ formatSelectedPaymentAmount(selectedTrafficPack.price) }}</span>
                  <span class="text-sm text-gray-500 dark:text-gray-400">/ {{ trafficPackCreditAmount(selectedTrafficPack.credit_usd) }}</span>
                </div>
                <p class="mt-2 text-sm leading-relaxed text-gray-500 dark:text-gray-400">
                  {{ selectedTrafficPack.description || trafficPackDefaultDescription(selectedTrafficPack.validity_days) }}
                </p>
                <div class="mt-3 grid grid-cols-2 gap-3">
                  <div>
                    <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('payment.planCard.validity') }}</span>
                    <div class="text-lg font-semibold text-gray-800 dark:text-gray-200">{{ selectedTrafficPack.validity_days }} {{ t('payment.days') }}</div>
                  </div>
                  <div>
                    <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('payment.trafficPack.availableQuota') }}</span>
                    <div class="text-lg font-semibold text-gray-800 dark:text-gray-200">{{ trafficPackUSDAmount(selectedTrafficPack.credit_usd) }}</div>
                  </div>
                </div>
              </div>
            </template>
            <template v-else-if="selectedPlan">
              <div class="card p-5">
                <div class="mb-3 flex flex-wrap items-center gap-2">
                  <span :class="['rounded-md border px-2 py-0.5 text-xs font-medium', planBadgeClass]">
                    {{ platformLabel(selectedPlan.group_platform || '') }}
                  </span>
                  <h3 class="text-lg font-bold text-gray-900 dark:text-white">{{ selectedPlan.name }}</h3>
                </div>
                <div class="flex items-baseline gap-2">
                  <span v-if="selectedPlan.original_price" class="text-sm text-gray-400 line-through dark:text-gray-500">
                    {{ formatSelectedPaymentAmount(selectedPlan.original_price) }}
                  </span>
                  <span :class="['text-3xl font-bold', planTextClass]">{{ formatSelectedPaymentAmount(selectedPlan.price) }}</span>
                  <span class="text-sm text-gray-500 dark:text-gray-400">/ {{ planValiditySuffix }}</span>
                </div>
                <p v-if="selectedPlanDescription" class="mt-2 text-sm leading-relaxed text-gray-500 dark:text-gray-400">
                  {{ selectedPlanDescription }}
                </p>
                <div class="mt-3 grid grid-cols-2 gap-3">
                  <div>
                    <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('payment.planCard.rate') }}</span>
                    <div class="flex items-baseline">
                      <span :class="['text-lg font-bold', planTextClass]">×{{ selectedPlan.rate_multiplier ?? 1 }}</span>
                    </div>
                  </div>
                  <div v-for="row in selectedPlanQuotaRows" :key="row.label">
                    <span class="text-xs text-gray-400 dark:text-gray-500">{{ row.label }}</span>
                    <div class="text-lg font-semibold text-gray-800 dark:text-gray-200">{{ row.value }}</div>
                  </div>
                </div>
              </div>
            </template>
            <div v-if="purchaseMethodOptions.length >= 1" class="card p-6">
              <PaymentMethodSelector
                :methods="purchaseMethodOptions"
                :selected="selectedMethod"
                @select="selectedMethod = $event"
              />
            </div>
            <div v-else class="card p-4 text-center">
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.notAvailable') }}</p>
            </div>
            <div v-if="feeRate > 0 && currentPurchaseProduct.price > 0" class="card p-6">
              <div class="space-y-2 text-sm">
                <div class="flex justify-between">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('payment.amountLabel') }}</span>
                  <span class="text-gray-900 dark:text-white">{{ formatSelectedPaymentAmount(currentPurchaseProduct.price) }}</span>
                </div>
                <div class="flex justify-between">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('payment.fee') }} ({{ feeRate }}%)</span>
                  <span class="text-gray-900 dark:text-white">{{ formatSelectedPaymentAmount(purchaseFeeAmount) }}</span>
                </div>
                <template v-if="purchaseHybridSummary">
                  <div class="flex justify-between border-t border-gray-200 pt-2 dark:border-dark-600">
                    <span class="text-gray-500 dark:text-gray-400">{{ t('payment.hybrid.balanceDeduction') }}</span>
                    <span class="font-medium text-emerald-600 dark:text-emerald-400">-{{ formatSelectedPaymentAmount(purchaseHybridSummary.balanceAmount) }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.hybrid.gatewayPay') }}</span>
                    <span class="text-lg font-bold text-gray-900 dark:text-gray-100">{{ formatSelectedPaymentAmount(purchaseHybridSummary.gatewayAmount) }}</span>
                  </div>
                </template>
                <div class="flex justify-between border-t border-gray-200 pt-2 dark:border-dark-600">
                  <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.actualPay') }}</span>
                  <span class="text-lg font-bold text-gray-900 dark:text-gray-100">{{ formatSelectedPaymentAmount(purchaseTotalAmount) }}</span>
                </div>
              </div>
            </div>
            <button :class="['btn w-full py-3 text-base font-medium', paymentButtonClass]" :disabled="!canSubmitPurchase || submitting" @click="confirmPurchase">
              <span v-if="submitting" class="flex items-center justify-center gap-2">
                <span class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
                {{ t('common.processing') }}
              </span>
              <span v-else>{{ t('payment.createOrder') }} {{ purchaseSubmitAmountText }}</span>
            </button>
            <button class="btn btn-secondary w-full" @click="backToSubscriptionList">{{ t('common.back') }}</button>
              </template>
              <template v-else>
            <div v-if="purchaseProducts.length === 0" class="card py-16 text-center">
              <Icon name="gift" size="xl" class="mx-auto mb-3 text-gray-300 dark:text-dark-600" />
              <p class="text-gray-500 dark:text-gray-400">{{ t('payment.noPlans') }}</p>
            </div>
            <div v-else :class="purchaseProductGridClass">
              <PurchaseProductCard
                v-for="item in purchaseProducts"
                :key="item.id"
                :product="item.product"
                @select="selectPurchaseProduct(item)"
              />
            </div>
              </template>
            </template>
            <div v-if="(checkout.help_text || checkout.help_image_url) && paymentPhase === 'select' && !currentPurchaseProduct" class="card p-4">
              <div class="flex flex-col items-center gap-3">
                <img v-if="checkout.help_image_url" :src="checkout.help_image_url" alt=""
                  class="h-40 max-w-full cursor-pointer rounded-lg object-contain transition-opacity hover:opacity-80"
                  @click="previewImage = checkout.help_image_url" />
                <p v-if="checkout.help_text" class="text-center text-sm text-gray-500 dark:text-gray-400">{{ checkout.help_text }}</p>
              </div>
            </div>
          </div>
        </Transition>
      </template>
    </div>
    <!-- Renewal Plan Selection Modal -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="showRenewalModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4" @click.self="closeRenewalModal">
          <div class="relative w-full max-w-lg rounded-2xl border border-gray-200 bg-white p-6 shadow-2xl dark:border-dark-700 dark:bg-dark-900">
            <!-- Close button -->
            <button class="absolute right-4 top-4 rounded-lg p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-200" @click="closeRenewalModal">
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
            <h3 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white">{{ t('payment.selectPlan') }}</h3>
            <div class="space-y-4">
              <PurchaseProductCard
                v-for="item in renewalProducts"
                :key="item.id"
                :product="item.product"
                @select="selectPlanFromModal(item.plan)"
              />
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
    <!-- Image Preview Overlay -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="previewImage" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 backdrop-blur-sm" @click="previewImage = ''">
          <img :src="previewImage" alt="" class="max-h-[85vh] max-w-[90vw] rounded-xl object-contain shadow-2xl" />
        </div>
      </Transition>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { usePaymentStore } from '@/stores/payment'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { useAppStore } from '@/stores'
import { paymentAPI } from '@/api/payment'
import { extractApiErrorMessage, extractI18nErrorMessage } from '@/utils/apiError'
import { isMobileDevice } from '@/utils/device'
import type { BalancePayOrderRequest, SubscriptionPlan, CheckoutInfoResponse, CreateOrderResult, OrderType, TrafficPack } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import PaymentMethodSelector from '@/components/payment/PaymentMethodSelector.vue'
import { METHOD_ORDER, getPaymentPopupFeatures } from '@/components/payment/providerConfig'
import {
  PAYMENT_RECOVERY_STORAGE_KEY,
  buildCreateOrderPayload,
  clearPaymentRecoverySnapshot,
  decidePaymentLaunch,
  getUserExternalPaymentMethods,
  getVisibleMethods,
  normalizeVisibleMethod,
  readPaymentRecoverySnapshot,
  type PaymentRecoverySnapshot,
  writePaymentRecoverySnapshot,
} from '@/components/payment/paymentFlow'
import { platformBadgeClass, platformTextClass, platformLabel } from '@/utils/platformColors'
import {
  PUBLIC_CODEX_SUBSCRIPTION_VALIDITY_DAYS,
  formatSubscriptionQuotaUSD,
  publicCodexSubscriptionWeeklyLimitUSD,
} from '@/utils/subscriptionQuota'
import PurchaseProductCard from '@/components/payment/PurchaseProductCard.vue'
import PaymentStatusPanel from '@/components/payment/PaymentStatusPanel.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatPaymentAmount, normalizePaymentCurrency } from '@/components/payment/currency'
import type { PaymentMethodOption } from '@/components/payment/PaymentMethodSelector.vue'
import { calculatePayableAmount } from '@/components/payment/payableAmount'
import type { PurchaseProductCardModel } from '@/components/payment/purchaseProductCard'
import { buildPaymentErrorToastMessage, describePaymentScenarioError } from './paymentUx'
import { hasWechatResumeQuery, parseWechatResumeRoute, stripWechatResumeQuery } from './paymentWechatResume'

const i18n = useI18n()
const { t } = i18n
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const paymentStore = usePaymentStore()
const subscriptionStore = useSubscriptionStore()
const appStore = useAppStore()

const activeSubscriptions = computed(() => subscriptionStore.activeSubscriptions)

const loading = ref(true)
const submitting = ref(false)
const errorMessage = ref('')
const errorHintMessage = ref('')
const amount = ref<number | null>(null)
const selectedMethod = ref('')
const previewImage = ref('')

const paymentPhase = ref<'select' | 'recharge' | 'paying'>('select')
const rechargeAmount = ref('1')
const rechargeError = computed(() => {
  if (!/^\d+$/.test(rechargeAmount.value.trim())) return t('payment.recharge.invalidAmount')
  const value = Number(rechargeAmount.value)
  if (value < 1 || value > 100) return t('payment.recharge.invalidAmount')
  return ''
})
const validRechargeAmount = computed(() => rechargeError.value ? 0 : Number(rechargeAmount.value))

type CurrentPurchaseProduct =
  | { orderType: 'subscription'; price: number; plan: SubscriptionPlan }
  | { orderType: 'traffic_pack'; price: number; trafficPack: TrafficPack }

const currentPurchaseProduct = ref<CurrentPurchaseProduct | null>(null)
const selectedPlan = computed(() =>
  currentPurchaseProduct.value?.orderType === 'subscription'
    ? currentPurchaseProduct.value.plan
    : null,
)
const selectedTrafficPack = computed(() =>
  currentPurchaseProduct.value?.orderType === 'traffic_pack'
    ? currentPurchaseProduct.value.trafficPack
    : null,
)

interface CreateOrderOptions {
  openid?: string
  wechatResumeToken?: string
  paymentType?: string
  trafficPackId?: number
  isResume?: boolean
  mobileQrFallbackAttempted?: boolean
}

interface WeixinJSBridgeLike {
  invoke(
    action: string,
    payload: Record<string, unknown>,
    callback: (result: Record<string, unknown>) => void,
  ): void
}

function emptyPaymentState(): PaymentRecoverySnapshot {
  return {
    orderId: 0,
    amount: 0,
    qrCode: '',
    qrImageUrl: '',
    expiresAt: '',
    paymentType: '',
    payUrl: '',
    outTradeNo: '',
    clientSecret: '',
    intentId: '',
    currency: '',
    countryCode: '',
    paymentEnv: '',
    payAmount: 0,
    orderType: '',
    paymentMode: '',
    resumeToken: '',
    fundingMode: '',
    balanceAmount: 0,
    gatewayAmount: 0,
    paymentResolutionStatus: '',
    paymentResolutionDeadline: '',
    compensationAmount: 0,
    compensatedAt: '',
    createdAt: 0,
  }
}

function getWeixinJSBridge(): WeixinJSBridgeLike | undefined {
  return (window as Window & { WeixinJSBridge?: WeixinJSBridgeLike }).WeixinJSBridge
}

function waitForWeixinJSBridge(timeoutMs = 4000): Promise<WeixinJSBridgeLike | null> {
  const existing = getWeixinJSBridge()
  if (existing) return Promise.resolve(existing)

  return new Promise((resolve) => {
    let settled = false
    const finish = (bridge: WeixinJSBridgeLike | null) => {
      if (settled) return
      settled = true
      document.removeEventListener('WeixinJSBridgeReady', handleReady)
      document.removeEventListener('onWeixinJSBridgeReady', handleReady)
      window.clearTimeout(timer)
      resolve(bridge)
    }
    const handleReady = () => finish(getWeixinJSBridge() ?? null)
    const timer = window.setTimeout(() => finish(getWeixinJSBridge() ?? null), timeoutMs)
    document.addEventListener('WeixinJSBridgeReady', handleReady, false)
    document.addEventListener('onWeixinJSBridgeReady', handleReady, false)
  })
}

async function invokeWechatJsapiPayment(payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  const bridge = await waitForWeixinJSBridge()
  if (!bridge) {
    throw new Error('WECHAT_JSAPI_UNAVAILABLE')
  }
  return new Promise((resolve) => {
    bridge.invoke('getBrandWCPayRequest', payload, (result) => resolve(result || {}))
  })
}

const paymentState = ref<PaymentRecoverySnapshot>(emptyPaymentState())

function persistRecoverySnapshot(snapshot: PaymentRecoverySnapshot) {
  if (typeof window === 'undefined' || !snapshot.orderId) return
  writePaymentRecoverySnapshot(window.localStorage, snapshot, PAYMENT_RECOVERY_STORAGE_KEY)
}

function removeRecoverySnapshot() {
  if (typeof window === 'undefined') return
  clearPaymentRecoverySnapshot(window.localStorage, PAYMENT_RECOVERY_STORAGE_KEY)
}

function resetPayment() {
  paymentPhase.value = 'select'
  paymentState.value = emptyPaymentState()
  removeRecoverySnapshot()
}

async function redirectToPaymentResult(state: PaymentRecoverySnapshot): Promise<void> {
  const query: Record<string, string | undefined> = {}
  if (state.orderId > 0) {
    query.order_id = String(state.orderId)
  }
  if (state.outTradeNo) {
    query.out_trade_no = state.outTradeNo
  }
  if (state.resumeToken) {
    query.resume_token = state.resumeToken
  }
  await router.push({
    path: '/payment/result',
    query,
  })
}

function buildWechatOAuthAuthorizeUrl(
  authorizeUrl: string,
  context: { paymentType: string; orderType: OrderType; planId?: number; trafficPackId?: number; orderAmount: number },
): string {
  const normalizedUrl = authorizeUrl.trim()
  if (!normalizedUrl || typeof window === 'undefined') {
    return normalizedUrl
  }

  try {
    const targetUrl = new URL(normalizedUrl, window.location.origin)
    const redirectPath = targetUrl.searchParams.get('redirect') || '/purchase'
    const redirectUrl = new URL(redirectPath, window.location.origin)
    const paymentType = normalizeVisibleMethod(context.paymentType) || context.paymentType.trim() || 'wxpay'

    redirectUrl.searchParams.set('payment_type', paymentType)
    redirectUrl.searchParams.set('order_type', context.orderType)

    if (context.planId) {
      redirectUrl.searchParams.set('plan_id', String(context.planId))
    } else {
      redirectUrl.searchParams.delete('plan_id')
    }
    if (context.trafficPackId) {
      redirectUrl.searchParams.set('traffic_pack_id', String(context.trafficPackId))
    } else {
      redirectUrl.searchParams.delete('traffic_pack_id')
    }

    if (context.orderAmount > 0) {
      redirectUrl.searchParams.set('amount', String(context.orderAmount))
    } else {
      redirectUrl.searchParams.delete('amount')
    }

    targetUrl.searchParams.set('redirect', `${redirectUrl.pathname}${redirectUrl.search}`)
    return targetUrl.toString()
  } catch {
    return normalizedUrl
  }
}

function onPaymentDone() {
  const wasSubscription = paymentState.value.orderType === 'subscription'
  const wasTrafficPack = paymentState.value.orderType === 'traffic_pack'
  resetPayment()
  currentPurchaseProduct.value = null
  if (wasSubscription) {
    subscriptionStore.fetchActiveSubscriptions(true).catch(() => {})
  }
  if (wasTrafficPack) {
    reloadCheckoutInfo().catch(() => {})
  }
}

function onPaymentSuccess() {
  removeRecoverySnapshot()
  authStore.refreshUser()
  if (paymentState.value.orderType === 'subscription') {
    subscriptionStore.fetchActiveSubscriptions(true).catch(() => {})
  }
  if (paymentState.value.orderType === 'traffic_pack') {
    reloadCheckoutInfo().catch(() => {})
  }
}

function onPaymentSettled(outcome?: string) {
  removeRecoverySnapshot()
  if (outcome === 'compensated') {
    authStore.refreshUser()
  }
}

// All checkout data from single API call
const checkout = ref<CheckoutInfoResponse>({
  methods: {}, global_min: 0, global_max: 0,
  plans: [], traffic_packs: [], traffic_credit_summary: null, traffic_credits: [], balance_disabled: false, recharge_fee_rate: 0, help_text: '', help_image_url: '', stripe_publishable_key: '',
})

const visibleMethods = computed(() => getVisibleMethods(checkout.value.methods))
const visibleExternalMethods = computed(() => getUserExternalPaymentMethods(checkout.value.methods))
const enabledMethods = computed(() => Object.keys(visibleMethods.value))
const validAmount = computed(() => amount.value ?? 0)
const purchaseProductGridClass = computed(() => {
  const n = purchaseProducts.value.length
  if (n <= 2) return 'grid auto-rows-[minmax(380px,auto)] grid-cols-1 gap-6 sm:grid-cols-2 sm:gap-8'
  if (n >= 4) return 'grid auto-rows-[minmax(380px,auto)] grid-cols-1 gap-6 sm:grid-cols-2 sm:gap-8 lg:grid-cols-4 lg:gap-12'
  return 'grid auto-rows-[minmax(380px,auto)] grid-cols-1 gap-6 sm:grid-cols-2 sm:gap-8 lg:grid-cols-3 lg:gap-12'
})

// Check if an amount fits a method's [min, max]. 0 = no limit.
function amountFitsMethod(amt: number, methodType: string): boolean {
  if (amt <= 0) return true
  const ml = visibleMethods.value[methodType]
  if (!ml) return false
  if (ml.single_min > 0 && amt < ml.single_min) return false
  if (ml.single_max > 0 && amt > ml.single_max) return false
  return true
}

// Selected method's limits (for validation and error messages)
const selectedLimit = computed(() => visibleMethods.value[selectedMethod.value])
const selectedCurrency = computed(() => normalizePaymentCurrency(selectedLimit.value?.currency))
const localeCode = computed(() => {
  const raw = i18n.locale as unknown
  if (typeof raw === 'string') return raw
  if (raw && typeof raw === 'object' && 'value' in raw) {
    return String((raw as { value?: string }).value || '')
  }
  return undefined
})

function formatSelectedPaymentAmount(value: number): string {
  return formatPaymentAmount(value, selectedCurrency.value, localeCode.value)
}

const feeRate = computed(() => checkout.value?.recharge_fee_rate ?? 0)

function roundPaymentCent(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.round((value + Number.EPSILON) * 100) / 100
}

function paymentAmountExpectation(value: number): string {
  return roundPaymentCent(value).toFixed(2)
}

type BalanceRechargePurchaseProduct = { id: 'balance-recharge'; type: 'balance_recharge'; product: PurchaseProductCardModel }
type SubscriptionPurchaseProduct = { id: string; type: 'subscription'; plan: SubscriptionPlan; product: PurchaseProductCardModel }
type TrafficPackPurchaseProduct = { id: string; type: 'traffic_pack'; pack: TrafficPack; product: PurchaseProductCardModel }
type PurchaseProduct = BalanceRechargePurchaseProduct | SubscriptionPurchaseProduct | TrafficPackPurchaseProduct

const formatCompactNumber = (value: number) => {
  if (!Number.isFinite(value)) return '0'
  return Number(value.toFixed(2)).toString()
}

const formatCardPrice = (value: number) => `¥${formatCompactNumber(value)}`
const formatCardBasePrice = (value: number) => `¥${formatCompactNumber(value)}元`
const cardFeeDetail = (price: number) =>
  feeRate.value > 0 ? `${formatCardBasePrice(price)} + ${formatCompactNumber(feeRate.value)}%` : formatCardBasePrice(price)
const trafficPackUSDAmount = (value: number) => t('payment.trafficPack.usdAmount', { amount: formatCompactNumber(value) })
const trafficPackCreditAmount = (value: number) => t('payment.trafficPack.creditAmount', { amount: formatCompactNumber(value) })
const trafficPackDefaultDescription = (days: number) => t('payment.trafficPack.defaultDescription', { days })

const planTitleSuffix = (index: number) => {
  if (index >= 0 && index < 26) return String.fromCharCode(65 + index)
  return String(index + 1)
}

const isPlanActive = (plan: SubscriptionPlan) =>
  activeSubscriptions.value.some(s => s.group_id === plan.group_id && s.status === 'active')

type PlanDetailRow = { label: string; value: string }

function planEffectiveValidityDays(plan: SubscriptionPlan): number {
  if (publicCodexSubscriptionWeeklyLimitUSD(plan.group_name) != null) {
    return PUBLIC_CODEX_SUBSCRIPTION_VALIDITY_DAYS
  }
  return plan.effective_validity_days ?? plan.validity_days
}

function planWeeklyLimitUSD(plan: SubscriptionPlan): number | null {
  return plan.weekly_limit_usd ?? publicCodexSubscriptionWeeklyLimitUSD(plan.group_name)
}

function planDisplayDescription(plan: SubscriptionPlan): string {
  if (publicCodexSubscriptionWeeklyLimitUSD(plan.group_name) != null) {
    return t('payment.planCard.weeklyDescription')
  }
  return plan.description || ''
}

function planPeriodTotalQuotaUSD(plan: SubscriptionPlan): number | null {
  if (plan.period_total_quota_usd != null) return plan.period_total_quota_usd
  const weeklyLimit = publicCodexSubscriptionWeeklyLimitUSD(plan.group_name)
  return weeklyLimit == null ? null : weeklyLimit * 4
}

function isWeeklyQuotaPlan(plan: SubscriptionPlan): boolean {
  return plan.quota_window_unit === 'week' || planWeeklyLimitUSD(plan) != null
}

function planQuotaRows(plan: SubscriptionPlan): PlanDetailRow[] {
  const validity = `${planEffectiveValidityDays(plan)} ${t('payment.days')}`

  if (isWeeklyQuotaPlan(plan)) {
    const weeklyLimit = planWeeklyLimitUSD(plan)
    const periodTotalQuota = planPeriodTotalQuotaUSD(plan)
    return [
      { label: t('payment.planCard.weeklyLimit'), value: formatSubscriptionQuotaUSD(weeklyLimit) },
      ...(periodTotalQuota == null ? [] : [
        { label: t('payment.planCard.periodTotalQuota'), value: formatSubscriptionQuotaUSD(periodTotalQuota) },
      ]),
      { label: t('payment.planCard.refreshTime'), value: t('payment.planCard.weeklyRefresh') },
      { label: t('payment.planCard.validity'), value: validity },
    ]
  }

  if (plan.daily_limit_usd != null) {
    return [
      { label: t('payment.planCard.dailyLimit'), value: formatSubscriptionQuotaUSD(plan.daily_limit_usd) },
      { label: t('payment.planCard.refreshTime'), value: t('payment.planCard.dailyRefresh') },
      { label: t('payment.planCard.validity'), value: validity },
    ]
  }

  if (plan.monthly_limit_usd != null) {
    return [
      { label: t('payment.planCard.monthlyLimit'), value: formatSubscriptionQuotaUSD(plan.monthly_limit_usd) },
      { label: t('payment.planCard.validity'), value: validity },
    ]
  }

  return [
    { label: t('payment.planCard.quota'), value: t('payment.planCard.unlimited') },
    { label: t('payment.planCard.validity'), value: validity },
  ]
}

async function refreshAndBlockDifferentActiveSubscription(plan: SubscriptionPlan): Promise<boolean> {
  try {
    await subscriptionStore.fetchActiveSubscriptions(true)
  } catch {
    // 刷新失败时保留当前缓存判断，最终仍由后端购买保护兜底。
  }
  const hasDifferentActiveSubscription = activeSubscriptions.value.some(
    subscription => subscription.status === 'active' && subscription.group_id !== plan.group_id,
  )
  if (!hasDifferentActiveSubscription) return false
  appStore.showError(t('payment.errors.ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND'))
  return true
}

function buildBalanceRechargeProduct(): BalanceRechargePurchaseProduct {
  return {
    id: 'balance-recharge',
    type: 'balance_recharge',
    product: {
      testId: 'purchase-product-card',
      eyebrowText: t('payment.productCard.balanceRecharge'),
      title: t('payment.recharge.title'),
      priceLabel: t('payment.productCard.price'),
      priceText: '¥1 起',
      buttonText: t('payment.recharge.button'),
      detailRows: [
        { label: t('payment.recharge.usage'), value: t('payment.recharge.usageValue') },
        { label: t('payment.recharge.arrival'), value: t('payment.recharge.arrivalValue') },
        { label: t('payment.recharge.fee'), value: t('payment.recharge.noFee') },
      ],
    },
  }
}

function buildSubscriptionProduct(plan: SubscriptionPlan, index: number): SubscriptionPurchaseProduct {
  const active = isPlanActive(plan)
  return {
    id: `subscription-${plan.id}`,
    type: 'subscription',
    plan,
    product: {
      testId: 'purchase-product-card',
      eyebrowText: t('payment.productCard.subscription'),
      title: plan.name || `订阅套餐${planTitleSuffix(index)}`,
      priceLabel: t('payment.productCard.price'),
      priceText: formatCardPrice(calculatePayableAmount(plan.price, feeRate.value)),
      buttonText: active ? t('payment.renewNow') : t('payment.subscribeNow'),
      active,
      detailRows: [
        ...planQuotaRows(plan),
        { label: t('payment.planCard.feeDetail'), value: cardFeeDetail(plan.price) },
      ],
    },
  }
}

function buildTrafficPackProduct(pack: TrafficPack): TrafficPackPurchaseProduct {
  return {
    id: `traffic-pack-${pack.id}`,
    type: 'traffic_pack',
    pack,
    product: {
      testId: 'purchase-product-card',
      eyebrowText: t('payment.trafficPack.eyebrow'),
      title: t('payment.trafficPack.title', { amount: formatCompactNumber(pack.credit_usd) }),
      priceLabel: t('payment.productCard.price'),
      priceText: formatCardPrice(calculatePayableAmount(pack.price, feeRate.value)),
      buttonText: t('payment.trafficPack.buyNow'),
      detailRows: [
        { label: t('payment.trafficPack.availableQuota'), value: trafficPackUSDAmount(pack.credit_usd) },
        { label: t('payment.planCard.validity'), value: `${pack.validity_days} ${t('payment.days')}` },
        { label: t('payment.planCard.feeDetail'), value: cardFeeDetail(pack.price) },
      ],
    },
  }
}

const purchaseProducts = computed<PurchaseProduct[]>(() => [
  buildBalanceRechargeProduct(),
  ...checkout.value.plans.map((plan, index) => buildSubscriptionProduct(plan, index)),
  ...checkout.value.traffic_packs.map(pack => buildTrafficPackProduct(pack)),
])

const rechargeMethodOptions = computed<PaymentMethodOption[]>(() => {
  const alipay = visibleExternalMethods.value.alipay
  return alipay ? [{ type: 'alipay', fee_rate: 0, available: alipay.available !== false }] : []
})

function productMethodOptionsFor(price: number): PaymentMethodOption[] {
  const methods: PaymentMethodOption[] = []
  const alipay = visibleExternalMethods.value.alipay
  if (alipay) {
    methods.push({
      type: 'alipay',
      fee_rate: alipay.fee_rate ?? 0,
      available: alipay.available !== false && amountFitsMethod(price, 'alipay'),
    })
  }
  methods.push({ type: 'balance', fee_rate: feeRate.value, available: true })
  return methods
}

const purchaseMethodOptions = computed<PaymentMethodOption[]>(() =>
  productMethodOptionsFor(currentPurchaseProduct.value?.price ?? 0),
)

const purchaseFeeAmount = computed(() => {
  const price = currentPurchaseProduct.value?.price ?? 0
  if (feeRate.value <= 0 || price <= 0) return 0
  return Math.ceil(((price * feeRate.value) / 100) * 100) / 100
})

const purchaseTotalAmount = computed(() => {
  const price = currentPurchaseProduct.value?.price ?? 0
  if (feeRate.value <= 0 || price <= 0) return price
  return Math.round((price + purchaseFeeAmount.value) * 100) / 100
})

const purchaseHybridSummary = computed(() => buildHybridPaymentSummary(purchaseTotalAmount.value))
const purchaseSubmitAmountText = computed(() => {
  const hybrid = purchaseHybridSummary.value
  if (hybrid) return formatSelectedPaymentAmount(hybrid.gatewayAmount)
  return formatSelectedPaymentAmount(feeRate.value > 0 ? purchaseTotalAmount.value : (currentPurchaseProduct.value?.price ?? 0))
})

const canSubmitPurchase = computed(() =>
  !!currentPurchaseProduct.value
  && purchaseMethodOptions.value.some(method => method.type === selectedMethod.value && method.available),
)

function buildHybridPaymentSummary(payAmount: number): { balanceAmount: number; gatewayAmount: number } | null {
  const visibleMethod = normalizeVisibleMethod(selectedMethod.value) || selectedMethod.value
  if (visibleMethod !== 'alipay') return null
  const balance = roundPaymentCent(userBalanceAmount())
  const total = roundPaymentCent(payAmount)
  if (balance <= 0 || total <= 0 || balance >= total) return null
  return {
    balanceAmount: balance,
    gatewayAmount: roundPaymentCent(total - balance),
  }
}

function hybridSummaryForOrderType(orderType: OrderType): { balanceAmount: number; payAmount: number } | null {
  if (currentPurchaseProduct.value?.orderType !== orderType || !purchaseHybridSummary.value) return null
  return {
    balanceAmount: purchaseHybridSummary.value.balanceAmount,
    payAmount: purchaseTotalAmount.value,
  }
}

function selectFirstAvailableProductMethod(methods: PaymentMethodOption[]) {
  const current = methods.find(method => method.type === selectedMethod.value && method.available)
  if (current) return
  selectedMethod.value = methods.find(method => method.available)?.type || ''
}

// Auto-switch to first available method when current selection can't handle the amount
watch(() => [validAmount.value, selectedMethod.value] as const, ([amt, method]) => {
  if (amt <= 0 || amountFitsMethod(amt, method)) return
  const available = enabledMethods.value.find((m) => amountFitsMethod(amt, m))
  if (available) selectedMethod.value = available
})

// Payment button class: follows selected payment method color
const paymentButtonClass = computed(() => {
  const m = selectedMethod.value
  if (!m) return 'btn-primary'
  if (m.includes('alipay')) return 'btn-alipay'
  if (m.includes('wxpay')) return 'btn-wxpay'
  if (m === 'stripe') return 'btn-stripe'
  if (m === 'airwallex') return 'btn-airwallex'
  return 'btn-primary'
})

// Subscription confirm: platform accent colors (clean card, no gradient)
const planBadgeClass = computed(() => platformBadgeClass(selectedPlan.value?.group_platform || ''))
const planTextClass = computed(() => platformTextClass(selectedPlan.value?.group_platform || ''))

// Renewal modal state
const showRenewalModal = ref(false)
const renewGroupId = ref<number | null>(null)
const renewalPlans = computed(() => {
  if (renewGroupId.value == null) return []
  return checkout.value.plans.filter(p => p.group_id === renewGroupId.value)
})
const renewalProducts = computed(() => renewalPlans.value.map((plan, index) => buildSubscriptionProduct(plan, index)))
const selectedPlanQuotaRows = computed(() => selectedPlan.value ? planQuotaRows(selectedPlan.value) : [])
const selectedPlanDescription = computed(() => selectedPlan.value ? planDisplayDescription(selectedPlan.value) : '')

const planValiditySuffix = computed(() => {
  if (!selectedPlan.value) return ''
  if (isWeeklyQuotaPlan(selectedPlan.value)) {
    return `${planEffectiveValidityDays(selectedPlan.value)}${t('payment.days')}`
  }
  const u = selectedPlan.value.validity_unit || 'day'
  if (u === 'month') return t('payment.perMonth')
  if (u === 'year') return t('payment.perYear')
  return `${planEffectiveValidityDays(selectedPlan.value)}${t('payment.days')}`
})

async function selectPlan(plan: SubscriptionPlan) {
  if (await refreshAndBlockDifferentActiveSubscription(plan)) return
  selectSubscriptionProduct(plan)
}

function selectSubscriptionProduct(plan: SubscriptionPlan) {
  currentPurchaseProduct.value = { orderType: 'subscription', price: plan.price, plan }
  errorMessage.value = ''
  selectFirstAvailableProductMethod(productMethodOptionsFor(plan.price))
}

function selectTrafficPack(pack: TrafficPack) {
  currentPurchaseProduct.value = { orderType: 'traffic_pack', price: pack.price, trafficPack: pack }
  errorMessage.value = ''
  selectFirstAvailableProductMethod(productMethodOptionsFor(pack.price))
}

async function selectPurchaseProduct(item: PurchaseProduct) {
  if (item.type === 'balance_recharge') {
    openRechargeConfirm(1)
    return
  }
  if (item.type === 'subscription') {
    await selectPlan(item.plan)
    return
  }
  selectTrafficPack(item.pack)
}

function backToSubscriptionList() {
  paymentPhase.value = 'select'
  currentPurchaseProduct.value = null
  errorMessage.value = ''
  errorHintMessage.value = ''
}

function openRechargeConfirm(defaultAmount = 1) {
  currentPurchaseProduct.value = null
  paymentPhase.value = 'recharge'
  selectedMethod.value = 'alipay'
  rechargeAmount.value = String(Math.min(100, Math.max(1, Math.ceil(defaultAmount))))
}

async function selectPlanFromModal(plan: SubscriptionPlan) {
  if (await refreshAndBlockDifferentActiveSubscription(plan)) return
  showRenewalModal.value = false
  renewGroupId.value = null
  selectSubscriptionProduct(plan)
}

function closeRenewalModal() {
  showRenewalModal.value = false
  renewGroupId.value = null
}

async function confirmPurchase() {
  const product = currentPurchaseProduct.value
  if (!product || submitting.value) return
  if (product.orderType === 'subscription' && await refreshAndBlockDifferentActiveSubscription(product.plan)) return
  if (!canSubmitPurchase.value) {
    appStore.showError(t('payment.notAvailable'))
    return
  }
  if (!ensureBalanceEnough(purchaseTotalAmount.value)) return
  if (selectedMethod.value === 'balance') {
    await balancePayProduct(
      product.orderType === 'subscription'
        ? { order_type: 'subscription', plan_id: product.plan.id }
        : { order_type: 'traffic_pack', traffic_pack_id: product.trafficPack.id },
    )
    return
  }
  await createOrder(product.price, product.orderType, product.orderType === 'subscription' ? product.plan.id : undefined, {
    trafficPackId: product.orderType === 'traffic_pack' ? product.trafficPack.id : undefined,
  })
}

async function confirmRecharge() {
  if (rechargeError.value || submitting.value) return
  await createOrder(validRechargeAmount.value, 'balance', undefined, { paymentType: 'alipay' })
}

function userBalanceAmount(): number {
  return Number(authStore.user?.balance || 0)
}

function ensureBalanceEnough(totalAmount: number): boolean {
  if (selectedMethod.value !== 'balance') return true
  const shortage = Math.max(0, totalAmount - userBalanceAmount())
  if (shortage <= 0) return true
  openRechargeConfirm(shortage)
  if (shortage > 100) {
    appStore.showWarning(t('payment.recharge.maxOnce'))
  }
  return false
}

async function balancePayProduct(payload: BalancePayOrderRequest) {
  submitting.value = true
  try {
    const result = await paymentAPI.balancePayOrder(payload)
    appStore.showSuccess(t('payment.balancePay.success'))
    await authStore.refreshUser()
    if (payload.order_type === 'subscription') {
      await subscriptionStore.fetchActiveSubscriptions(true)
    }
    if (payload.order_type === 'traffic_pack') {
      await reloadCheckoutInfo()
    }
    backToSubscriptionList()
    paymentState.value = {
      ...emptyPaymentState(),
      orderId: result.data.order_id,
      amount: result.data.amount,
      payAmount: result.data.pay_amount,
      paymentType: 'balance',
      orderType: payload.order_type,
      outTradeNo: result.data.out_trade_no || '',
      currency: 'CNY',
    }
  } catch (err: unknown) {
    const reason = typeof err === 'object' && err && 'reason' in err ? String(err.reason) : ''
    if (reason === 'BALANCE_INSUFFICIENT') {
      openRechargeConfirm(1)
      return
    }
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('payment.result.failed')))
  } finally {
    submitting.value = false
  }
}

async function createOrder(orderAmount: number, orderType: OrderType, planId?: number, options: CreateOrderOptions = {}) {
  submitting.value = true
  errorMessage.value = ''
  errorHintMessage.value = ''
  const requestType = normalizeVisibleMethod(options.paymentType || selectedMethod.value) || options.paymentType || selectedMethod.value
  const hybridSummary = hybridSummaryForOrderType(orderType)
  try {
    const payload = buildCreateOrderPayload({
      amount: orderAmount,
      paymentType: requestType,
      orderType,
      planId,
      trafficPackId: options.trafficPackId,
      origin: typeof window !== 'undefined' ? window.location.origin : '',
      isMobile: isMobileDevice(),
      isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
      forceQRCode: !!(checkout.value.alipay_force_qrcode && normalizeVisibleMethod(requestType) === 'alipay'),
      useBalance: !options.isResume && !!hybridSummary && normalizeVisibleMethod(requestType) === 'alipay',
      expectedPayAmount: hybridSummary ? paymentAmountExpectation(hybridSummary.payAmount) : undefined,
      expectedBalanceAmount: hybridSummary ? paymentAmountExpectation(hybridSummary.balanceAmount) : undefined,
    })
    if (options.openid) {
      payload.openid = options.openid
    }
    if (options.wechatResumeToken) {
      payload.wechat_resume_token = options.wechatResumeToken
    }

    const result = await paymentStore.createOrder(payload) as CreateOrderResult & { resume_token?: string }
    const openWindow = (url: string) => {
      const win = window.open(url, 'paymentPopup', getPaymentPopupFeatures())
      if (!win || win.closed) {
        window.location.href = url
      }
    }
    const visibleMethod = normalizeVisibleMethod(requestType) || requestType
    // When user clicks the dedicated Stripe button, leave method blank so the
    // landing page renders Stripe's full Payment Element (card/link/alipay/wxpay).
    const stripeMethod = visibleMethod === 'stripe'
      ? ''
      : visibleMethod === 'wxpay' ? 'wechat_pay' : 'alipay'
    const stripeRouteUrl = result.client_secret && visibleMethod !== 'airwallex'
      ? router.resolve({
        path: '/payment/stripe',
        query: {
          order_id: String(result.order_id),
          client_secret: result.client_secret,
          method: stripeMethod || undefined,
          resume_token: result.resume_token || undefined,
        },
      }).href
      : ''
    const airwallexRouteUrl = result.client_secret && result.intent_id
      ? router.resolve({
        path: '/payment/airwallex',
        query: {
          order_id: String(result.order_id),
          out_trade_no: result.out_trade_no || undefined,
          resume_token: result.resume_token || undefined,
        },
      }).href
      : ''
    const decision = decidePaymentLaunch(result, {
      visibleMethod,
      orderType,
      isMobile: isMobileDevice(),
      isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
      forceQRCode: !!(checkout.value.alipay_force_qrcode && visibleMethod === 'alipay'),
      stripePopupUrl: stripeRouteUrl,
      stripeRouteUrl,
      airwallexRouteUrl,
    })

    if (decision.kind === 'wechat_oauth' && decision.oauth?.authorize_url) {
      window.location.href = buildWechatOAuthAuthorizeUrl(decision.oauth.authorize_url, {
        paymentType: visibleMethod,
        orderType,
        planId,
        trafficPackId: options.trafficPackId,
        orderAmount,
      })
      return
    }

    if (decision.kind === 'unhandled') {
      applyScenarioError({ reason: 'UNHANDLED_PAYMENT_SCENARIO' }, visibleMethod)
      return
    }

    paymentState.value = decision.paymentState
    paymentPhase.value = 'paying'
    persistRecoverySnapshot(decision.recovery)

    if (decision.kind === 'stripe_popup') {
      openWindow(decision.paymentState.payUrl)
      return
    }
    if (decision.kind === 'stripe_route') {
      window.location.href = decision.paymentState.payUrl
      return
    }
    if (decision.kind === 'airwallex_route') {
      window.location.href = decision.paymentState.payUrl
      return
    }
    if (decision.kind === 'wechat_jsapi' && decision.jsapi) {
      try {
        const jsapiResult = await invokeWechatJsapiPayment(decision.jsapi as Record<string, unknown>)
        const errMsg = String(jsapiResult.err_msg || '').toLowerCase()
        if (errMsg.includes('cancel')) {
          appStore.showInfo(t('payment.qr.cancelled'))
          resetPayment()
        } else if (errMsg && !errMsg.includes('ok')) {
          resetPayment()
          const fallbackApplied = await attemptMobileQrFallback(
            { reason: 'WECHAT_JSAPI_FAILED', message: errMsg },
            {
              orderAmount,
              orderType,
              planId,
              trafficPackId: options.trafficPackId,
              paymentType: visibleMethod,
              hybrid: hybridSummary,
              attempted: options.mobileQrFallbackAttempted === true,
            },
          )
          if (!fallbackApplied) {
            applyScenarioError({ reason: 'WECHAT_JSAPI_FAILED', message: errMsg }, visibleMethod)
          }
        } else {
          const resultState = { ...decision.paymentState }
          resetPayment()
          await redirectToPaymentResult(resultState)
        }
      } catch (err: unknown) {
        resetPayment()
        const fallbackApplied = await attemptMobileQrFallback(err, {
          orderAmount,
          orderType,
          planId,
          trafficPackId: options.trafficPackId,
          paymentType: visibleMethod,
          hybrid: hybridSummary,
          attempted: options.mobileQrFallbackAttempted === true,
        })
        if (!fallbackApplied) {
          throw err
        }
      }
      return
    }
    if (decision.kind === 'redirect_waiting' && decision.paymentState.payUrl) {
      if (isMobileDevice()) {
        window.location.href = decision.paymentState.payUrl
        return
      }
      openWindow(decision.paymentState.payUrl)
    }
  } catch (err: unknown) {
    const apiErr = err as Record<string, unknown>
    if (apiErr.reason === 'TOO_MANY_PENDING') {
      const metadata = apiErr.metadata as Record<string, unknown> | undefined
      errorMessage.value = t('payment.errors.tooManyPending', { max: metadata?.max || '' })
      errorHintMessage.value = ''
    } else if (apiErr.reason === 'CANCEL_RATE_LIMITED') {
      errorMessage.value = t('payment.errors.cancelRateLimited')
      errorHintMessage.value = ''
    } else if (await attemptMobileQrFallback(err, {
      orderAmount,
      orderType,
      planId,
      trafficPackId: options.trafficPackId,
      paymentType: requestType,
      hybrid: hybridSummary,
      attempted: options.mobileQrFallbackAttempted === true,
    })) {
      return
    } else {
      const handled = applyScenarioError(
        err,
        normalizeVisibleMethod(options.paymentType || selectedMethod.value) || selectedMethod.value,
      )
      if (!handled) {
        errorMessage.value = extractI18nErrorMessage(err, t, 'payment.errors', extractApiErrorMessage(err, t('payment.result.failed')))
        errorHintMessage.value = ''
      }
      if (handled) {
        return
      }
    }
    appStore.showError(buildPaymentErrorToastMessage(errorMessage.value, errorHintMessage.value))
  } finally {
    submitting.value = false
  }
}

interface MobileQrFallbackContext {
  orderAmount: number
  orderType: OrderType
  planId?: number
  trafficPackId?: number
  paymentType: string
  hybrid?: { balanceAmount: number; payAmount: number } | null
  attempted: boolean
}

function shouldFallbackToDesktopQr(err: unknown, paymentMethod: string, attempted: boolean): boolean {
  if (attempted || !isMobileDevice()) {
    return false
  }

  const normalizedMethod = normalizeVisibleMethod(paymentMethod) || paymentMethod
  const reason = typeof err === 'object' && err && 'reason' in err && typeof err.reason === 'string'
    ? err.reason
    : ''
  const message = err instanceof Error
    ? err.message
    : (typeof err === 'object' && err && 'message' in err && typeof err.message === 'string'
      ? err.message
      : '')
  const normalizedMessage = message.toLowerCase()

  if (normalizedMethod === 'wxpay') {
    return reason === 'WECHAT_H5_NOT_AUTHORIZED'
      || reason === 'WECHAT_PAYMENT_MP_NOT_CONFIGURED'
      || reason === 'WECHAT_JSAPI_FAILED'
      || reason === 'PAYMENT_GATEWAY_ERROR'
      || reason === 'UNHANDLED_PAYMENT_SCENARIO'
      || normalizedMessage.includes('weixinjsbridge is unavailable')
      || normalizedMessage.includes('wechat_jsapi_unavailable')
  }

  if (normalizedMethod === 'alipay') {
    return reason === 'PAYMENT_GATEWAY_ERROR' || reason === 'UNHANDLED_PAYMENT_SCENARIO'
  }

  return false
}

async function attemptMobileQrFallback(err: unknown, context: MobileQrFallbackContext): Promise<boolean> {
  if (!shouldFallbackToDesktopQr(err, context.paymentType, context.attempted)) {
    return false
  }

  try {
    const visibleMethod = normalizeVisibleMethod(context.paymentType) || context.paymentType
    const payload = buildCreateOrderPayload({
      amount: context.orderAmount,
      paymentType: visibleMethod,
      orderType: context.orderType,
      planId: context.planId,
      trafficPackId: context.trafficPackId,
      origin: typeof window !== 'undefined' ? window.location.origin : '',
      isMobile: false,
      isWechatBrowser: false,
      useBalance: !!context.hybrid,
      expectedPayAmount: context.hybrid ? paymentAmountExpectation(context.hybrid.payAmount) : undefined,
      expectedBalanceAmount: context.hybrid ? paymentAmountExpectation(context.hybrid.balanceAmount) : undefined,
    })
    const result = await paymentStore.createOrder(payload) as CreateOrderResult & { resume_token?: string }
    const stripeMethod = visibleMethod === 'wxpay' ? 'wechat_pay' : 'alipay'
    const stripeRouteUrl = result.client_secret
      ? router.resolve({
        path: '/payment/stripe',
        query: {
          order_id: String(result.order_id),
          client_secret: result.client_secret,
          method: stripeMethod,
          resume_token: result.resume_token || undefined,
        },
      }).href
      : ''
    const decision = decidePaymentLaunch(result, {
      visibleMethod,
      orderType: context.orderType,
      isMobile: false,
      isWechatBrowser: false,
      stripePopupUrl: stripeRouteUrl,
      stripeRouteUrl,
    })

    if (decision.kind !== 'qr_waiting' || (!decision.paymentState.qrCode && !decision.paymentState.qrImageUrl)) {
      return false
    }

    errorMessage.value = ''
    errorHintMessage.value = ''
    paymentState.value = decision.paymentState
    paymentPhase.value = 'paying'
    persistRecoverySnapshot(decision.recovery)
    appStore.showWarning(t('payment.errors.mobilePaymentFallbackToQr'))
    return true
  } catch {
    return false
  }
}

function applyScenarioError(err: unknown, paymentMethod: string): boolean {
  const descriptor = describePaymentScenarioError(err, {
    paymentMethod,
    isMobile: isMobileDevice(),
    isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
  })
  if (!descriptor) {
    errorMessage.value = ''
    errorHintMessage.value = ''
    return false
  }
  errorMessage.value = t(descriptor.messageKey)
  errorHintMessage.value = descriptor.hintKey ? t(descriptor.hintKey) : ''
  appStore.showError(buildPaymentErrorToastMessage(errorMessage.value, errorHintMessage.value))
  return true
}

async function resumeWechatPaymentFromQuery() {
  const resume = parseWechatResumeRoute(route.query, checkout.value.plans, validAmount.value)
  if (!resume) {
    return
  }

  selectedMethod.value = resume.paymentType
  if (resume.orderType === 'balance' && resume.orderAmount > 0) {
    amount.value = resume.orderAmount
  }
  if (resume.orderType === 'subscription' && resume.planId) {
    const plan = checkout.value.plans.find(item => item.id === resume.planId)
    if (plan) selectSubscriptionProduct(plan)
  }
  if (resume.orderType === 'traffic_pack' && resume.trafficPackId) {
    const pack = checkout.value.traffic_packs.find(item => item.id === resume.trafficPackId)
    if (pack) selectTrafficPack(pack)
  }

  await router.replace({ path: route.path, query: stripWechatResumeQuery(route.query) })

  if (resume.wechatResumeToken) {
    await createOrder(0, resume.orderType, resume.planId, {
      wechatResumeToken: resume.wechatResumeToken,
      paymentType: resume.paymentType,
      isResume: true,
    })
    return
  }

  if (resume.orderAmount > 0 && resume.openid) {
    await createOrder(resume.orderAmount, resume.orderType, resume.planId, {
      openid: resume.openid,
      paymentType: resume.paymentType,
      isResume: true,
    })
  }
}

async function reloadCheckoutInfo() {
  const res = await paymentAPI.getCheckoutInfo()
  checkout.value = {
    ...res.data,
    traffic_packs: res.data.traffic_packs ?? [],
    traffic_credit_summary: res.data.traffic_credit_summary ?? null,
    traffic_credits: res.data.traffic_credits ?? [],
  }
}

onMounted(async () => {
  try {
    await reloadCheckoutInfo()
    if (enabledMethods.value.length) {
      const order: readonly string[] = METHOD_ORDER
      const sorted = [...enabledMethods.value].sort((a, b) => {
        const ai = order.indexOf(a)
        const bi = order.indexOf(b)
        return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
      })
      selectedMethod.value = sorted[0]
    }
    if (typeof window !== 'undefined') {
      if (hasWechatResumeQuery(route.query)) {
        removeRecoverySnapshot()
      }
      const routeResumeToken = typeof route.query.resume_token === 'string'
        ? route.query.resume_token
        : typeof route.query.wechat_resume_token === 'string'
          ? route.query.wechat_resume_token
          : undefined
      const restored = readPaymentRecoverySnapshot(
        window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY),
        { resumeToken: routeResumeToken },
      )
      if (restored) {
        paymentState.value = restored
        paymentPhase.value = 'paying'
        const restoredMethod = normalizeVisibleMethod(restored.paymentType)
        if (restoredMethod) {
          selectedMethod.value = restoredMethod
        }
      } else {
        removeRecoverySnapshot()
      }
    }
    await resumeWechatPaymentFromQuery()
    // Handle renewal navigation: ?tab=subscription&group=123
    if (route.query.group) {
      const groupId = Number(route.query.group)
      const groupPlans = checkout.value.plans.filter(p => p.group_id === groupId)
      if (groupPlans.length > 0 && !(await refreshAndBlockDifferentActiveSubscription(groupPlans[0]))) {
        if (groupPlans.length === 1) {
          selectSubscriptionProduct(groupPlans[0])
        } else {
          renewGroupId.value = groupId
          showRenewalModal.value = true
        }
      }
    }
  } catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
  finally { loading.value = false }
  // Fetch active subscriptions (uses cache, non-blocking)
  subscriptionStore.fetchActiveSubscriptions().catch(() => {})
})
</script>

<style scoped>
.payment-phase-enter-active,
.payment-phase-leave-active {
  transition: transform 180ms var(--ease-out), opacity 180ms var(--ease-out);
}

.payment-phase-leave-active {
  position: absolute;
  left: 0;
  right: 0;
  pointer-events: none;
}

.payment-phase-enter-from,
.payment-phase-leave-to {
  opacity: 0;
  transform: translate3d(0, 4px, 0);
}

@media (prefers-reduced-motion: reduce) {
  .payment-phase-enter-active,
  .payment-phase-leave-active {
    transition-property: opacity;
  }

  .payment-phase-enter-from,
  .payment-phase-leave-to {
    transform: none;
  }
}
</style>
