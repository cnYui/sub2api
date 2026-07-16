/**
 * Shared utility functions for payment order display.
 * Used by AdminOrderDetail, AdminOrderTable, AdminRefundDialog, AdminOrdersView, etc.
 */

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
  PARTIALLY_REFUNDED: 'badge-warning',
  REFUNDED: 'badge-info',
  REFUND_FAILED: 'badge-danger',
}

const REFUNDABLE_STATUSES = ['COMPLETED', 'PARTIALLY_REFUNDED', 'REFUND_REQUESTED']

export function statusBadgeClass(status: string): string {
  return STATUS_BADGE_MAP[status] || 'badge-secondary'
}

export function canRefund(status: string, refundRetryable = false): boolean {
  if (status === 'REFUND_FAILED') return refundRetryable
  return REFUNDABLE_STATUSES.includes(status)
}

export function isAutomaticRefundAllowed(paymentType: string): boolean {
  return paymentType !== 'offline'
}

export function formatOrderDateTime(dateStr: string): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString()
}
