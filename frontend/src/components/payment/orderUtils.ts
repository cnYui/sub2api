/** 支付订单展示和退款入口共用判断。 */

import type { PaymentOrder } from '@/types/payment'

const STATUS_BADGE_MAP: Record<string, string> = {
  PENDING: 'badge-warning',
  PAID: 'badge-info',
  RECHARGING: 'badge-info',
  COMPLETED: 'badge-success',
  EXPIRED: 'badge-secondary',
  CANCELLED: 'badge-secondary',
  FAILED: 'badge-danger',
  REFUND_REQUESTED: 'badge-warning',
  REFUNDING: 'badge-warning',
  REFUND_PENDING: 'badge-warning',
  PARTIALLY_REFUNDED: 'badge-warning',
  REFUNDED: 'badge-info',
  REFUND_FAILED: 'badge-danger',
}

const REFUNDABLE_STATUSES = ['REFUND_REQUESTED', 'REFUND_FAILED']
const REAL_PAYMENT_TYPES = new Set(['alipay', 'alipay_direct', 'wxpay', 'wxpay_direct', 'stripe', 'card', 'link', 'easypay', 'airwallex'])

export function statusBadgeClass(status: string): string {
  return STATUS_BADGE_MAP[status] || 'badge-secondary'
}

export function canRefund(status: string): boolean {
  return REFUNDABLE_STATUSES.includes(status)
}

/** 只有真实支付产生的余额套餐才允许进入退款流程。 */
export function isRealPaidBalancePackageOrder(order: PaymentOrder | null | undefined): boolean {
  if (!order) return false
  return order.order_type === 'balance_subscription'
    && REAL_PAYMENT_TYPES.has(String(order.payment_type).trim().toLowerCase())
    && Number(order.pay_amount) > 0
    && Boolean(order.paid_at)
}

export function formatOrderDateTime(dateStr: string): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString()
}
