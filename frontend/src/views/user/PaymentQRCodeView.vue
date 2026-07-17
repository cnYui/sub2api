<template>
  <AppLayout>
    <div class="mx-auto max-w-md py-8">
      <PaymentStatusPanel
        :order-id="orderId"
        :qr-code="qrCode"
        :qr-image-url="qrImageUrl"
        :expires-at="expiresAt"
        :payment-type="paymentType"
        :pay-url="payUrl"
        :order-type="orderType"
        :currency="currency"
        @done="router.push('/purchase')"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PaymentStatusPanel from '@/components/payment/PaymentStatusPanel.vue'

const route = useRoute()
const router = useRouter()

function queryValue(name: string): string {
  const value = route.query[name]
  if (Array.isArray(value)) return value[0] || ''
  return value ? String(value) : ''
}

const orderId = Number(queryValue('order_id')) || 0
const qrCode = queryValue('qr')
const qrImageUrl = queryValue('qr_image_url')
const payUrl = queryValue('pay_url')
const expiresAt = queryValue('expires_at')
const paymentType = queryValue('payment_type')
const orderType = queryValue('order_type')
const currency = queryValue('currency')
</script>
