const PAYMENT_AMOUNT_EPSILON = 1e-9

export function calculatePaymentFee(amount: number, feeRate: number): number {
  if (!Number.isFinite(amount) || !Number.isFinite(feeRate) || amount <= 0 || feeRate <= 0) return 0
  return Math.ceil((amount * feeRate) - PAYMENT_AMOUNT_EPSILON) / 100
}

export function calculatePaymentTotal(amount: number, feeRate: number): number {
  const fee = calculatePaymentFee(amount, feeRate)
  return Math.round((amount + fee) * 100) / 100
}
