export function calculateFeeAmount(amount: number, feeRate: number): number {
  if (!Number.isFinite(amount) || !Number.isFinite(feeRate) || amount <= 0 || feeRate <= 0) return 0
  return Math.ceil(((amount * feeRate) / 100) * 100) / 100
}

export function calculatePayableAmount(amount: number, feeRate: number): number {
  if (!Number.isFinite(amount) || amount <= 0) return 0
  if (!Number.isFinite(feeRate) || feeRate <= 0) return amount
  return Math.round((amount + calculateFeeAmount(amount, feeRate)) * 100) / 100
}
