import type { PaymentOrder } from '@/types/payment'
import { isAutomaticRefundAllowed } from '@/components/payment/orderUtils'

export function canRequestOrderRefund(order: PaymentOrder, refundEligibleProviders: Set<string>): boolean {
  if (order.order_type !== 'subscription') return false
  if (!isAutomaticRefundAllowed(order.payment_type)) return false
  if (order.status === 'REFUND_FAILED') return order.refund_retryable === true
  if (order.status !== 'COMPLETED') return false
  if (order.payment_type === 'balance') return true
  if (order.payment_type !== 'alipay') return false
  if (!order.provider_instance_id) return false
  return refundEligibleProviders.has(order.provider_instance_id)
}
