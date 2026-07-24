import { describe, expect, it } from 'vitest'

import { calculateFeeAmount, calculatePayableAmount } from '../payableAmount'

describe('payableAmount', () => {
  it('计算 249 元 1% 手续费时不被浮点误差抬高一分钱', () => {
    expect(calculateFeeAmount(49, 1)).toBe(0.49)
    expect(calculatePayableAmount(49, 1)).toBe(49.49)
    expect(calculateFeeAmount(249, 1)).toBe(2.49)
    expect(calculatePayableAmount(249, 1)).toBe(251.49)
  })

  it('保留真实超过分位的手续费向上取整', () => {
    expect(calculateFeeAmount(249.01, 1)).toBe(2.5)
    expect(calculatePayableAmount(249.01, 1)).toBe(251.51)
  })
})
