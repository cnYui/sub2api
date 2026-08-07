import { describe, expect, it } from 'vitest'
import { calculatePaymentFee, calculatePaymentTotal } from '../paymentAmount'

describe('paymentAmount', () => {
  it('按 1% 向上取整到分并计算余额套餐实付金额', () => {
    expect(calculatePaymentFee(29, 1)).toBe(0.29)
    expect(calculatePaymentTotal(29, 1)).toBe(29.29)
  })

  it('流量卡同样按标价收取 1% 手续费', () => {
    expect(calculatePaymentFee(2, 1)).toBe(0.02)
    expect(calculatePaymentTotal(2, 1)).toBe(2.02)
  })

  it('手续费按分向上取整', () => {
    expect(calculatePaymentFee(29.99, 1)).toBe(0.3)
    expect(calculatePaymentTotal(29.99, 1)).toBe(30.29)
  })

  it('手续费关闭时实付金额保持标价', () => {
    expect(calculatePaymentFee(49.99, 0)).toBe(0)
    expect(calculatePaymentTotal(49.99, 0)).toBe(49.99)
  })
})
